package rlim

import "errors"

// Sentinel errors returned by Limiter and configuration helpers.
var (
	// ErrInvalidLimit means Limit.Requests was not greater than zero.
	ErrInvalidLimit = errors.New("rlim: limit requests must be > 0")
	// ErrInvalidWindow means Limit.Window was not greater than zero.
	ErrInvalidWindow = errors.New("rlim: limit window must be > 0")
	// ErrNoClientID means the configured ClientKeyResolver produced an empty key.
	ErrNoClientID = errors.New("rlim: unable to resolve client id")
	// ErrNoBucket means AllowClient was called with an empty bucket string.
	ErrNoBucket = errors.New("rlim: bucket is required")
	// ErrNoDefaultLimit is defined for API symmetry; New validates limits via Limit.validate instead.
	ErrNoDefaultLimit = errors.New("rlim: default limit is required")
	// ErrNoStore means New was called with a nil Store.
	ErrNoStore = errors.New("rlim: store is required")
)
