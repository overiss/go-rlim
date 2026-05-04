// Package rlim provides HTTP-oriented fixed-window rate limiting with pluggable
// storage (in-memory, Redis, etcd), optional per-bucket limits, and identity
// resolution from cookies, explicit client IDs, or client IP. Use Limiter as
// net/http middleware, or call AllowHTTPRequest / AllowClient and optionally
// WriteRateLimitHeaders / WriteTooManyRequests / WriteLimiterError for custom handlers
// and middleware.
package rlim
