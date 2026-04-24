package middleware

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit returns a middleware that allows at most r requests per second
// with a burst of b from each client IP, returning 429 when exceeded.
// The provided context controls the lifetime of the background pruning goroutine.
func RateLimit(ctx context.Context, r rate.Limit, b int) func(http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*ipLimiter)
	)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				for ip, l := range limiters {
					if time.Since(l.lastSeen) > 5*time.Minute {
						delete(limiters, ip)
					}
				}
				mu.Unlock()
			}
		}
	}()

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := limiters[ip]
		if !ok {
			l = &ipLimiter{limiter: rate.NewLimiter(r, b)}
			limiters[ip] = l
		}
		l.lastSeen = time.Now()
		return l.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ip, _, err := net.SplitHostPort(req.RemoteAddr)
			if err != nil {
				ip = req.RemoteAddr
			}
			if !getLimiter(ip).Allow() {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
