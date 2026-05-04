package rlim

import (
	"net/http"
	"time"
)

// BucketResolver maps an HTTP request to a logical bucket name (e.g. path, route id).
// The bucket selects which entry from limitsByKey applies, or falls back to defaultLimit.
type BucketResolver func(*http.Request) string

// options holds resolved Limiter configuration after applying Option functions.
type options struct {
	cookieName        string
	bucketResolver    BucketResolver
	clientKeyResolver ClientKeyResolver
	nowFn             func() time.Time
}

// Option is a functional option applied in order by New to customize the limiter.
type Option func(*options)

// WithCookieName sets the cookie name consulted first by the default ClientKeyResolver.
// The cookie should be set HttpOnly by your application if you rely on it for browser clients.
func WithCookieName(cookieName string) Option {
	return func(o *options) {
		o.cookieName = cookieName
	}
}

// WithBucketResolver replaces the default bucket function (URL path) with a custom mapping.
func WithBucketResolver(resolver BucketResolver) Option {
	return func(o *options) {
		if resolver != nil {
			o.bucketResolver = resolver
		}
	}
}

// WithClientKeyResolver replaces the default cookie → ID → IP identity extraction.
func WithClientKeyResolver(resolver ClientKeyResolver) Option {
	return func(o *options) {
		if resolver != nil {
			o.clientKeyResolver = resolver
		}
	}
}
