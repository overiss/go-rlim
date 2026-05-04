package rlim

import "time"

// Limit defines a fixed-window quota: at most Requests events per Window duration
// for a given (bucket, client) counter. When the window expires, the counter resets.
type Limit struct {
	// Requests is the maximum number of allowed increments per window (must be > 0).
	Requests int64
	// Window is the duration of one fixed window (must be > 0).
	Window time.Duration
}

// validate checks that Requests and Window are positive; used by New and when
// registering per-bucket limits.
func (l Limit) validate() error {
	if l.Requests <= 0 {
		return ErrInvalidLimit
	}
	if l.Window <= 0 {
		return ErrInvalidWindow
	}
	return nil
}

// Decision is the outcome of a single AllowClient or AllowHTTPRequest check after
// the backing store has incremented the counter for the current window.
type Decision struct {
	// Allowed is true if the current count is still within the limit for this window.
	Allowed bool
	// Count is the total increments recorded in the current window (including this one).
	Count int64
	// Remaining is how many requests are left before the limit is hit (never negative).
	Remaining int64
	// ResetAt is when the current fixed window ends and the counter will roll over.
	ResetAt time.Time
	// RetryAfter is a suggested wait duration before retrying when Allowed is false.
	RetryAfter time.Duration
}
