// Package clientutil provides HTTP client middleware for common functionality
// like rate limiting, logging, and user agent management.
package clientutil

import (
	"log/slog"
	"net/http"
	"time"
)

type Middleware func(http.RoundTripper) http.RoundTripper

func Chain(middlewares ...Middleware) Middleware {
	if len(middlewares) == 1 {
		return middlewares[0]
	}
	return func(final http.RoundTripper) http.RoundTripper {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

func WithLogging(logger *slog.Logger) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripFunc(func(r *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.RoundTrip(r)
			if err != nil {
				return nil, err
			}
			logger.DebugContext(r.Context(), "response", "status", resp.StatusCode, "url", r.URL, "took", time.Since(start))
			return resp, nil
		})
	}
}

func WithUserAgent(userAgent string) Middleware {
	if userAgent == "" {
		return Passthrough
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripFunc(func(r *http.Request) (*http.Response, error) {
			r.Header.Add("User-Agent", userAgent)
			return next.RoundTrip(r)
		})
	}
}

func Passthrough(next http.RoundTripper) http.RoundTripper {
	return next
}

type RoundTripFunc func(*http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func Wrap(c *http.Client, mw Middleware) *http.Client {
	if c == nil {
		c = &http.Client{}
	}
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	c.Transport = mw(c.Transport)
	return c
}

