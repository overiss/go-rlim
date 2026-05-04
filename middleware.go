package rlim

import (
	"net/http"
)

// Middleware returns an http.Handler that runs AllowHTTPRequest before next.
// On error it responds with 500; on deny with 429, Retry-After (seconds) when known,
// and X-RateLimit-Remaining / X-RateLimit-Reset headers when allowed or denied.
// It is equivalent to the custom-middleware pattern using WriteLimiterError,
// WriteTooManyRequests, WriteRateLimitHeaders, and next.ServeHTTP.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decision, err := l.AllowHTTPRequest(r.Context(), r)
		if err != nil {
			WriteLimiterError(w, err)
			return
		}

		if !decision.Allowed {
			WriteTooManyRequests(w, decision)
			return
		}

		WriteRateLimitHeaders(w, decision)
		next.ServeHTTP(w, r)
	})
}
