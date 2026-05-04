# go-rlim

HTTP-oriented rate limiting for Go services: one core limiter, pluggable storage, optional framework middleware, and a first-class **service-layer** API so you can enforce limits from handlers, RPC layers, or jobs—not only from HTTP middleware.

## Why this library exists

Many teams need rate limits that are:

- **Consistent across replicas** (shared state, not per-process only).
- **Keyed by something stable** (session cookie, user id, or client IP) without duplicating identity logic in every handler.
- **Configurable per route or feature** (“login” stricter than “read profile”) without scattering magic strings.
- **Callable outside middleware** (gRPC gateways, WebSockets, background throttles, tests).

`go-rlim` separates **who** is limited (`Client` / cookie / IP), **what** bucket applies (`BucketResolver` + per-bucket `Limit` map), and **where** counts live (`Store`: memory, Redis, etcd). That keeps HTTP adapters thin and the limiter reusable.

## Capabilities and features

| Area | What you get |
|------|----------------|
| **Algorithm** | Fixed-window counters per `(bucket, clientKey)`; predictable reset at window boundary. |
| **Identity** | Default: named cookie → optional explicit `Client.ID` → IP (with `X-Forwarded-For` / `X-Real-IP` / `RemoteAddr` parsing). Fully replaceable resolver. |
| **HTTP integration** | Plain `*http.Request` via `AllowHTTPRequest` + response helpers; built-in `net/http` middleware; optional **Gin**, **Echo**, **Fiber**, **gorilla/mux**. |
| **Service layer** | `Limiter.AllowClient` / `AllowHTTPRequest` for non-middleware use; inject the same `*Limiter` everywhere. |
| **Per-bucket limits** | `defaultLimit` + `map[string]Limit` for route names, paths, or arbitrary keys. |
| **In-memory** | `MemoryStore` with a **background cleaner** to drop expired or idle keys and bound memory. |
| **Distributed** | **Redis** (Lua `INCR` + `PEXPIRE` for atomic window start); **etcd** (compare-and-swap transactions for correctness under concurrency). |
| **Observability** | `Decision` exposes count, remaining, reset time, suggested `RetryAfter`; middleware sets `X-RateLimit-*` and `Retry-After` where applicable. |

## Architecture (mental model)

1. **Bucket** — logical group for a limit (e.g. URL path, route name, `"api:write"`). Resolved from `*http.Request` or passed explicitly in `AllowClient`.
2. **Client key** — stable string derived from `Client` (cookie value, id, or IP). Colliding keys share one counter.
3. **Store** — atomically increments a counter for the composite key and returns `(count, resetAt)`.
4. **Limiter** — picks `Limit` for the bucket, calls `Store.Increment`, returns `Decision`.

## Installation

```bash
go get github.com/rykovm/go-rlim
```

Subpackages (optional adapters / stores):

```bash
go get github.com/rykovm/go-rlim/stores
go get github.com/rykovm/go-rlim/adapters/gin
# … echo, fiber, mux as needed
```

## Quick start (service layer)

Use this when you already have a user/session context and want to call the limiter directly (no middleware on that code path).

```go
mem := stores.NewMemoryStore(time.Minute, 5*time.Minute)
defer mem.Close()

limiter, err := rlim.New(
    mem,
    rlim.Limit{Requests: 100, Window: time.Minute},
    map[string]rlim.Limit{
        "/api/login": {Requests: 10, Window: time.Minute},
    },
    rlim.WithCookieName("session_id"),
)
if err != nil {
    log.Fatal(err)
}

decision, err := limiter.AllowClient(ctx, "/api/login", rlim.Client{
    IP: "10.0.0.1",
    Cookies: map[string]string{"session_id": "abc123"},
})
if err != nil {
    // store / validation error
}
if !decision.Allowed {
    // 429-style handling, logging, metrics
}
```

## Quick start (`net/http` middleware)

```go
mux := http.NewServeMux()
handler := limiter.Middleware(mux)
http.ListenAndServe(":8080", handler)
```

## Plain `*http.Request` (no middleware, or your own)

Use **`Limiter.AllowHTTPRequest(ctx, r)`** anywhere you have an `*http.Request`: inside a handler, before proxying, or inside **custom** `net/http` middleware. The built-in `Limiter.Middleware` is optional; it is implemented with the same public helpers you can call yourself:

- **`rlim.WriteLimiterError(w, err)`** — store or validation failure → **500**
- **`rlim.WriteTooManyRequests(w, decision)`** — limit exceeded → **429** + `Retry-After` when known
- **`rlim.WriteRateLimitHeaders(w, decision)`** — `X-RateLimit-*` (and `Retry-After` when denied)

**Inside a handler** (full control over status and JSON):

```go
func handle(w http.ResponseWriter, r *http.Request) {
    d, err := limiter.AllowHTTPRequest(r.Context(), r)
    if err != nil {
        rlim.WriteLimiterError(w, err)
        return
    }
    if !d.Allowed {
        rlim.WriteRateLimitHeaders(w, d)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        _, _ = w.Write([]byte(`{"error":"rate limited"}`))
        return
    }
    rlim.WriteRateLimitHeaders(w, d)
    // ... business logic
}
```

**Custom middleware** (same behavior as `limiter.Middleware`):

```go
func withRateLimit(l *rlim.Limiter, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        d, err := l.AllowHTTPRequest(r.Context(), r)
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
```

## Framework adapters

Each adapter wraps `Limiter.AllowHTTPRequest` and sets rate-limit headers; on deny it returns **429** (and JSON where the framework typically uses JSON).

- **Gin**: `github.com/rykovm/go-rlim/adapters/gin` → `ginadapter.Middleware(limiter)`
- **Echo**: `.../adapters/echo` → `echoadapter.Middleware(limiter)`
- **Fiber**: `.../adapters/fiber` → `fiberadapter.Middleware(limiter)` (builds an `*http.Request` for the limiter)
- **gorilla/mux**: `.../adapters/mux` → `muxadapter.Middleware(limiter)`

## Stores

### In-memory (`stores.MemoryStore`)

Good for single-instance dev or small deployments. The **cleaner** runs on an interval and removes entries past the rate window **or** idle longer than `maxIdle` to cap memory.

```go
store := stores.NewMemoryStore(
    1*time.Minute,  // how often to run cleanup
    5*time.Minute,  // drop keys with no touches longer than this
)
defer store.Close()
```

### Redis (`stores.RedisStore`)

Shared counter across all app replicas; window length is enforced via key TTL in milliseconds.

```go
rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store := stores.NewRedisStore(rdb, "rlim:")
```

### etcd (`stores.EtcdStore`)

Transactional increments with revision checks so concurrent writers on different nodes do not corrupt counts.

```go
cli, err := clientv3.New(clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}})
if err != nil {
    log.Fatal(err)
}
defer cli.Close()
store := stores.NewEtcdStore(cli, "rlim/")
```

## Configuration options (`rlim.Option`)

- **`WithCookieName(name)`** — cookie used first for identity (e.g. session id). Your app should set **HttpOnly** cookies if you rely on them for abuse resistance in browsers.
- **`WithBucketResolver(fn)`** — map `*http.Request` to a bucket string (default: `r.URL.Path`).
- **`WithClientKeyResolver(fn)`** — replace default cookie → id → IP logic (e.g. JWT subject, API key header).

## API documentation

Full **Go doc** comments are on every exported symbol and on internal helpers in the repository. From the module root:

```bash
go doc -all github.com/rykovm/go-rlim
go doc -all github.com/rykovm/go-rlim/stores
```

## Limitations (by design)

- **Fixed window** — simpler than sliding window; bursts can stack at window edges. Extend with a different `Store` / algorithm if you need token bucket or sliding window later.
- **Trust boundaries** — IP and `X-Forwarded-For` are only as trustworthy as your edge proxy configuration.
- **Cookie vs IP** — cookies identify browsers more stably than NAT IPs; choose `WithCookieName` and HttpOnly session cookies when appropriate.

## License

Add your license file as needed for your organization.
