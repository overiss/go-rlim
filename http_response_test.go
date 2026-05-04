package rlim_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/overiss/go-rlim"
	"github.com/overiss/go-rlim/stores"
)

// TestCustomHTTPMiddlewarePattern verifies AllowHTTPRequest plus public write helpers
// match the built-in Middleware behavior for an allowed request.
func TestCustomHTTPMiddlewarePattern(t *testing.T) {
	store := stores.NewMemoryStore(time.Second, time.Second)
	t.Cleanup(func() { _ = store.Close() })

	limiter, err := rlim.New(store, rlim.Limit{Requests: 5, Window: time.Minute}, nil)
	if err != nil {
		t.Fatal(err)
	}

	custom := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d, err := limiter.AllowHTTPRequest(r.Context(), r)
			if err != nil {
				rlim.WriteLimiterError(w, err)
				return
			}
			if !d.Allowed {
				rlim.WriteTooManyRequests(w, d)
				return
			}
			rlim.WriteRateLimitHeaders(w, d)
			next.ServeHTTP(w, r)
		})
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	custom(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Fatal("expected X-RateLimit-Remaining header")
	}
}

// TestAllowHTTPRequestInsideHandler exercises direct *http.Request use without wrapping middleware.
func TestAllowHTTPRequestInsideHandler(t *testing.T) {
	store := stores.NewMemoryStore(time.Second, time.Second)
	t.Cleanup(func() { _ = store.Close() })

	limiter, err := rlim.New(store, rlim.Limit{Requests: 1, Window: time.Minute}, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/p", nil)
	req.RemoteAddr = "192.168.1.1:1"

	d1, err := limiter.AllowHTTPRequest(context.Background(), req)
	if err != nil || !d1.Allowed {
		t.Fatalf("first: err=%v d=%+v", err, d1)
	}
	d2, err := limiter.AllowHTTPRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if d2.Allowed {
		t.Fatal("second request should be denied")
	}
}
