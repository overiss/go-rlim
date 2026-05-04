package stores

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdStore implements rlim.Store using etcd compare-and-swap; suitable when Redis
// is unavailable but you still need consistent cross-node counters.
type EtcdStore struct {
	client     *clientv3.Client
	prefix     string
	maxRetries int
}

// NewEtcdStore creates a store with optional key prefix (default "rlim/") and a
// bounded retry count for transaction conflicts.
func NewEtcdStore(client *clientv3.Client, prefix string) *EtcdStore {
	if prefix == "" {
		prefix = "rlim/"
	}
	return &EtcdStore{client: client, prefix: prefix, maxRetries: 8}
}

// Increment reads the current value (if any), applies fixed-window logic in memory,
// then commits with a txn whose compare clause matches the last seen revision.
func (s *EtcdStore) Increment(ctx context.Context, key string, window time.Duration, now time.Time) (int64, time.Time, error) {
	fullKey := s.prefix + key

	for i := 0; i < s.maxRetries; i++ {
		getResp, err := s.client.Get(ctx, fullKey)
		if err != nil {
			return 0, time.Time{}, err
		}

		var (
			currentCount int64
			currentExp   time.Time
			modRev       int64
			exists       bool
		)

		if len(getResp.Kvs) > 0 {
			exists = true
			kv := getResp.Kvs[0]
			modRev = kv.ModRevision
			currentCount, currentExp, err = parseEtcdValue(string(kv.Value))
			if err != nil {
				return 0, time.Time{}, err
			}
			if now.After(currentExp) {
				exists = false
			}
		}

		nextCount := int64(1)
		nextExp := now.Add(window)
		if exists {
			nextCount = currentCount + 1
			nextExp = currentExp
		}

		val := formatEtcdValue(nextCount, nextExp)
		var txnResp *clientv3.TxnResponse
		if exists {
			txnResp, err = s.client.Txn(ctx).
				If(clientv3.Compare(clientv3.ModRevision(fullKey), "=", modRev)).
				Then(clientv3.OpPut(fullKey, val)).
				Commit()
		} else {
			txnResp, err = s.client.Txn(ctx).
				If(clientv3.Compare(clientv3.Version(fullKey), "=", 0)).
				Then(clientv3.OpPut(fullKey, val)).
				Commit()
		}
		if err != nil {
			return 0, time.Time{}, err
		}
		if txnResp.Succeeded {
			return nextCount, nextExp, nil
		}
	}

	return 0, time.Time{}, fmt.Errorf("etcd increment retries exceeded for key %q", key)
}

// parseEtcdValue decodes "count|expiryNanos" written by formatEtcdValue.
func parseEtcdValue(raw string) (int64, time.Time, error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 2 {
		return 0, time.Time{}, fmt.Errorf("invalid etcd value format: %q", raw)
	}
	count, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, time.Time{}, err
	}
	expNanos, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, time.Time{}, err
	}
	return count, time.Unix(0, expNanos), nil
}

// formatEtcdValue encodes count and window end time for atomic replacement in etcd.
func formatEtcdValue(count int64, exp time.Time) string {
	return strconv.FormatInt(count, 10) + "|" + strconv.FormatInt(exp.UnixNano(), 10)
}
