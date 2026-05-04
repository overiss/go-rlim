// Package fiberadapter integrates rlim with github.com/gofiber/fiber/v2 by synthesizing
// an *http.Request for AllowHTTPRequest (path, method, headers, client IP).
package fiberadapter

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/rykovm/go-rlim"
)

// Middleware returns a fiber.Handler that applies l before c.Next().
func Middleware(l *rlim.Limiter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		req, err := http.NewRequestWithContext(c.UserContext(), c.Method(), "http://fiber.local"+c.OriginalURL(), nil)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		req.RemoteAddr = c.IP()
		req.URL.Path = c.Path()

		c.Request().Header.VisitAll(func(key, val []byte) {
			req.Header.Add(string(key), string(val))
		})

		decision, err := l.AllowHTTPRequest(c.UserContext(), req)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		c.Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
		if !decision.Allowed {
			if decision.RetryAfter > 0 {
				c.Set("Retry-After", strconv.FormatInt(int64(decision.RetryAfter.Seconds()), 10))
			}
			return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
		}

		return c.Next()
	}
}
