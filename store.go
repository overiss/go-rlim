package rlim

import (
	"context"
	"time"
)

// Store persists fixed-window counters. Implementations must make Increment
// atomic (or equivalent) for a given key so concurrent requests see consistent counts.
type Store interface {
	// Increment adds one to the counter for key in the current window; if the window
	// for key has expired relative to now, a new window starts. It returns the new
	// total count and the instant when the current window ends (resetAt).
	Increment(ctx context.Context, key string, window time.Duration, now time.Time) (count int64, resetAt time.Time, err error)
}

// CloseableStore may be implemented by stores that own background goroutines or
// connections; callers can type-assert and Close on shutdown.
type CloseableStore interface {
	// Close releases resources (e.g. stops cleaner goroutines); safe to call once.
	Close() error
}
