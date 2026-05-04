// Package echoadapter integrates rlim with github.com/labstack/echo/v4 via middleware.
package echoadapter

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/rykovm/go-rlim"
)

// Middleware returns echo.MiddlewareFunc that runs the limiter before the next handler.
func Middleware(l *rlim.Limiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			decision, err := l.AllowHTTPRequest(c.Request().Context(), c.Request())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			c.Response().Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))

			if !decision.Allowed {
				if decision.RetryAfter > 0 {
					c.Response().Header().Set("Retry-After", strconv.FormatInt(int64(decision.RetryAfter.Seconds()), 10))
				}
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			}

			return next(c)
		}
	}
}
