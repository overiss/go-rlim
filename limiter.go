package rlim

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Limiter coordinates per-bucket limits, client identity, and a Store. It is safe
// for concurrent use if the underlying Store is safe for concurrent use.
type Limiter struct {
	store        Store
	defaultLimit Limit
	limitsByKey  map[string]Limit
	opts         options
}

// New constructs a Limiter. store must be non-nil. defaultLimit applies to any
// bucket not present in limitsByKey; each entry in limitsByKey is validated.
// Options tune cookie name, bucket resolution, and client key resolution.
func New(store Store, defaultLimit Limit, limitsByKey map[string]Limit, opts ...Option) (*Limiter, error) {
	if store == nil {
		return nil, ErrNoStore
	}
	if err := defaultLimit.validate(); err != nil {
		return nil, err
	}

	compiled := options{
		cookieName:        "rlim_id",
		bucketResolver:    func(r *http.Request) string { return r.URL.Path },
		clientKeyResolver: defaultClientKeyResolver,
		nowFn:             time.Now,
	}
	for _, opt := range opts {
		opt(&compiled)
	}

	validated := make(map[string]Limit, len(limitsByKey))
	for k, v := range limitsByKey {
		if err := v.validate(); err != nil {
			return nil, err
		}
		validated[k] = v
	}

	return &Limiter{
		store:        store,
		defaultLimit: defaultLimit,
		limitsByKey:  validated,
		opts:         compiled,
	}, nil
}

// AllowClient evaluates the limit for bucket using client-derived identity. bucket
// must be non-empty; identity comes from opts.clientKeyResolver (see WithClientKeyResolver).
func (l *Limiter) AllowClient(ctx context.Context, bucket string, client Client) (Decision, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return Decision{}, ErrNoBucket
	}
	clientID := strings.TrimSpace(l.opts.clientKeyResolver(client, l.opts.cookieName))
	if clientID == "" {
		return Decision{}, ErrNoClientID
	}

	return l.allow(ctx, bucket, clientID)
}

// AllowHTTPRequest is the primary API for plain *http.Request: it resolves the bucket
// via opts.bucketResolver, builds a Client from r (see ClientFromRequest), then runs
// the same logic as AllowClient. For handlers or custom net/http middleware, combine
// with WriteRateLimitHeaders, WriteTooManyRequests, and WriteLimiterError instead of
// using Limiter.Middleware.
func (l *Limiter) AllowHTTPRequest(ctx context.Context, r *http.Request) (Decision, error) {
	bucket := strings.TrimSpace(l.opts.bucketResolver(r))
	return l.AllowClient(ctx, bucket, ClientFromRequest(r))
}

// allow loads the Limit for bucket, increments the store key bucket:clientID, and
// builds Allowed / Remaining / RetryAfter from the resulting count.
func (l *Limiter) allow(ctx context.Context, bucket, clientID string) (Decision, error) {
	limit, ok := l.limitsByKey[bucket]
	if !ok {
		limit = l.defaultLimit
	}

	now := l.opts.nowFn()
	key := bucket + ":" + clientID
	count, resetAt, err := l.store.Increment(ctx, key, limit.Window, now)
	if err != nil {
		return Decision{}, err
	}

	remaining := limit.Requests - count
	if remaining < 0 {
		remaining = 0
	}
	allowed := count <= limit.Requests
	retryAfter := time.Duration(0)
	if !allowed {
		retryAfter = time.Until(resetAt)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return Decision{
		Allowed:    allowed,
		Count:      count,
		Remaining:  remaining,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}, nil
}
