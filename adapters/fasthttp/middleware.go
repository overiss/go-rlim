// Package fasthttpadapter integrates rlim with github.com/valyala/fasthttp server handlers.
package fasthttpadapter

import (
	"context"
	"net/http"
	"strconv"

	"github.com/rykovm/go-rlim"
	"github.com/valyala/fasthttp"
)

// Middleware wraps a fasthttp.RequestHandler with rate limiting.
// It resolves limits through rlim.Limiter and, when blocked, writes 429 with
// standard rate-limit headers and "Too Many Requests" response body.
func Middleware(l *rlim.Limiter, next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		req, err := toHTTPRequest(ctx)
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.SetBodyString(err.Error())
			return
		}

		decision, err := l.AllowHTTPRequest(context.Background(), req)
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.SetBodyString(err.Error())
			return
		}

		ctx.Response.Header.Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
		ctx.Response.Header.Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))

		if !decision.Allowed {
			if decision.RetryAfter > 0 {
				ctx.Response.Header.Set("Retry-After", strconv.FormatInt(int64(decision.RetryAfter.Seconds()), 10))
			}
			ctx.SetStatusCode(http.StatusTooManyRequests)
			ctx.SetBodyString(http.StatusText(http.StatusTooManyRequests))
			return
		}

		next(ctx)
	}
}

// toHTTPRequest maps fasthttp request data into a standard *http.Request used by limiter.
func toHTTPRequest(ctx *fasthttp.RequestCtx) (*http.Request, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		string(ctx.Method()),
		"http://fasthttp.local"+string(ctx.RequestURI()),
		nil,
	)
	if err != nil {
		return nil, err
	}

	if addr := ctx.RemoteAddr(); addr != nil {
		req.RemoteAddr = addr.String()
	}
	req.URL.Path = string(ctx.Path())

	ctx.Request.Header.VisitAll(func(k, v []byte) {
		req.Header.Add(string(k), string(v))
	})

	return req, nil
}
