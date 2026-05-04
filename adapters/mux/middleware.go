// Package muxadapter integrates rlim with github.com/gorilla/mux router middleware.
package muxadapter

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/overiss/go-rlim"
)

// Middleware returns mux.MiddlewareFunc compatible with router.Use.
func Middleware(l *rlim.Limiter) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			decision, err := l.AllowHTTPRequest(r.Context(), r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))

			if !decision.Allowed {
				if decision.RetryAfter > 0 {
					w.Header().Set("Retry-After", strconv.FormatInt(int64(decision.RetryAfter.Seconds()), 10))
				}
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
