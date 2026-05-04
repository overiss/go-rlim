package rlim_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rykovm/go-rlim"
	"github.com/rykovm/go-rlim/stores"
)

// TestAllowClient verifies fixed-window behavior when keyed by IP only (no cookie).
func TestAllowClient(t *testing.T) {
	store := stores.NewMemoryStore(time.Second, time.Second)
	t.Cleanup(func() { _ = store.Close() })

	limiter, err := rlim.New(store, rlim.Limit{Requests: 2, Window: time.Minute}, nil)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	client := rlim.Client{IP: "127.0.0.1"}

	d1, err := limiter.AllowClient(context.Background(), "/test", client)
	if err != nil || !d1.Allowed {
		t.Fatalf("first request should be allowed, err=%v decision=%+v", err, d1)
	}

	d2, err := limiter.AllowClient(context.Background(), "/test", client)
	if err != nil || !d2.Allowed {
		t.Fatalf("second request should be allowed, err=%v decision=%+v", err, d2)
	}

	d3, err := limiter.AllowClient(context.Background(), "/test", client)
	if err != nil {
		t.Fatalf("third request unexpected error: %v", err)
	}
	if d3.Allowed {
		t.Fatalf("third request must be blocked")
	}
}

// TestCookiePreferredOverIP ensures two different RemoteAddr values share one counter
// when the same session cookie is present.
func TestCookiePreferredOverIP(t *testing.T) {
	store := stores.NewMemoryStore(time.Second, time.Second)
	t.Cleanup(func() { _ = store.Close() })

	limiter, err := rlim.New(store, rlim.Limit{Requests: 1, Window: time.Minute}, nil, rlim.WithCookieName("session"))
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	req1 := httptest.NewRequest("GET", "http://localhost/a", nil)
	req1.RemoteAddr = "10.0.0.1:1111"
	req1.AddCookie(&http.Cookie{Name: "session", Value: "abc"})

	req2 := httptest.NewRequest("GET", "http://localhost/a", nil)
	req2.RemoteAddr = "10.0.0.1:2222"
	req2.AddCookie(&http.Cookie{Name: "session", Value: "abc"})

	d1, err := limiter.AllowHTTPRequest(context.Background(), req1)
	if err != nil || !d1.Allowed {
		t.Fatalf("first request should be allowed, err=%v decision=%+v", err, d1)
	}

	d2, err := limiter.AllowHTTPRequest(context.Background(), req2)
	if err != nil {
		t.Fatalf("second request unexpected error: %v", err)
	}
	if d2.Allowed {
		t.Fatalf("second request must be blocked for same cookie identity")
	}
}
