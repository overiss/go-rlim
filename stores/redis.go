package stores

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// incrScript atomically INCRs the key, sets PEXPIRE on first increment in a window,
// and returns the new count plus remaining TTL in milliseconds.
var incrScript = redis.NewScript(`
local value = redis.call('INCR', KEYS[1])
if value == 1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {value, ttl}
`)

// RedisStore implements rlim.Store using Redis; safe across replicas sharing one Redis.
type RedisStore struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisStore wraps a Redis client with an optional key prefix (default "rlim:").
func NewRedisStore(client redis.UniversalClient, prefix string) *RedisStore {
	if prefix == "" {
		prefix = "rlim:"
	}
	return &RedisStore{client: client, prefix: prefix}
}

// Increment runs a Lua script so INCR and window TTL are atomic per key.
func (s *RedisStore) Increment(ctx context.Context, key string, window time.Duration, now time.Time) (int64, time.Time, error) {
	res, err := incrScript.Run(ctx, s.client, []string{s.prefix + key}, window.Milliseconds()).Result()
	if err != nil {
		return 0, time.Time{}, err
	}
	parts := res.([]interface{})
	count := parts[0].(int64)
	ttlMillis := parts[1].(int64)
	if ttlMillis < 0 {
		ttlMillis = window.Milliseconds()
	}
	return count, now.Add(time.Duration(ttlMillis) * time.Millisecond), nil
}
