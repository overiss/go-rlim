package rlim

import (
	"net/http"
	"strconv"
)

// WriteRateLimitHeaders sets X-RateLimit-Remaining and X-RateLimit-Reset from d.
// When d.Allowed is false and d.RetryAfter is positive, it also sets Retry-After
// (whole seconds). Use this from handlers or custom middleware after AllowHTTPRequest.
func WriteRateLimitHeaders(w http.ResponseWriter, d Decision) {
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(d.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(d.ResetAt.Unix(), 10))
	if !d.Allowed && d.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(int64(d.RetryAfter.Seconds()), 10))
	}
}

// WriteTooManyRequests writes rate-limit headers (via WriteRateLimitHeaders) and
// responds with HTTP 429 and a plain-text body. Call when AllowHTTPRequest returned
// d with d.Allowed == false.
func WriteTooManyRequests(w http.ResponseWriter, d Decision) {
	WriteRateLimitHeaders(w, d)
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}

// WriteLimiterError responds with HTTP 500 and the error message as body. Use when
// AllowHTTPRequest or AllowClient returns a non-nil error (e.g. store failure).
func WriteLimiterError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
