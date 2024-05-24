package clientutil

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gregjones/httpcache"
	"golang.org/x/time/rate"
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

func WithCache() Middleware {
	cache := NewMemoryCache()
	return func(next http.RoundTripper) http.RoundTripper {
		transport := httpcache.NewTransport(cache)
		transport.Transport = next
		return transport
	}
}

func WithRateLimit(interval time.Duration) Middleware {
	if interval == 0 {
		return Passthrough
	}
	return func(next http.RoundTripper) http.RoundTripper {
		limiter := rate.NewLimiter(rate.Every(interval), 1)
		return RoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := limiter.Wait(r.Context()); err != nil {
				return nil, err
			}
			return next.RoundTrip(r)
		})
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
			logger.Debug("response", "status", resp.StatusCode, "url", r.URL, "took", time.Since(start))
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

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string][]byte
}

func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{items: map[string][]byte{}}
	go func() {
		t := time.NewTicker(45 * time.Second)
		defer t.Stop()
		for range t.C {
			cache.mu.Lock()
			clear(cache.items)
			cache.mu.Unlock()
		}
	}()
	return cache
}

func (c *MemoryCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	resp, ok := c.items[key]
	return resp, ok
}

func (c *MemoryCache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = data
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
