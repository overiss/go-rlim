// Package ginadapter integrates rlim with github.com/gin-gonic/gin via middleware.
package ginadapter

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rykovm/go-rlim"
)

// Middleware returns a gin.HandlerFunc that enforces limits using l before c.Next().
// On failure it aborts with JSON; on rate limit with 429 and rate-limit headers.
func Middleware(l *rlim.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		decision, err := l.AllowHTTPRequest(c.Request.Context(), c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Header("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))

		if !decision.Allowed {
			if decision.RetryAfter > 0 {
				c.Header("Retry-After", strconv.FormatInt(int64(decision.RetryAfter.Seconds()), 10))
			}
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
