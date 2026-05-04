package rlim

import (
	"net"
	"net/http"
	"strings"
)

// Client carries identity fields for service-layer rate limiting. Handlers can
// build it explicitly; ClientFromRequest fills IP and Cookies from an *http.Request.
type Client struct {
	// ID is an optional application-level identifier (e.g. user id, API key id).
	ID string
	// IP is the client IP string used when cookie and ID are absent or empty.
	IP string
	// Cookies maps cookie name to value; used when the configured cookie name is set.
	Cookies map[string]string
}

// ClientKeyResolver derives a stable string key from Client for counter namespacing.
// cookieName is the limiter's configured cookie name (from WithCookieName); return
// empty string if no identity can be resolved (AllowClient will return ErrNoClientID).
type ClientKeyResolver func(client Client, cookieName string) string

// defaultClientKeyResolver prefers the named cookie, then Client.ID, then Client.IP,
// prefixing values to reduce accidental collisions between namespaces.
func defaultClientKeyResolver(client Client, cookieName string) string {
	if cookieName != "" {
		if v := strings.TrimSpace(client.Cookies[cookieName]); v != "" {
			return "cookie:" + v
		}
	}
	if v := strings.TrimSpace(client.ID); v != "" {
		return "id:" + v
	}
	if v := strings.TrimSpace(client.IP); v != "" {
		return "ip:" + v
	}
	return ""
}

// ClientFromRequest builds a Client using RemoteAddr-derived IP and all cookies
// present on the request. It does not read the request body.
func ClientFromRequest(r *http.Request) Client {
	client := Client{
		IP:      requestIP(r),
		Cookies: map[string]string{},
	}

	for _, c := range r.Cookies() {
		client.Cookies[c.Name] = c.Value
	}

	return client
}

// requestIP returns a best-effort client IP: X-Forwarded-For (first hop),
// then X-Real-IP, then host part of RemoteAddr, else raw RemoteAddr.
func requestIP(r *http.Request) string {
	ff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if ff != "" {
		first := strings.TrimSpace(strings.Split(ff, ",")[0])
		if first != "" {
			return first
		}
	}
	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
