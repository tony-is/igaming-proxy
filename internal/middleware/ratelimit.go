package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter holds per-IP rate limiters.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	rps      float64
	burst    int
}

// NewRateLimiter creates a RateLimiter and starts a background cleanup goroutine.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 100
	}
	if burst <= 0 {
		burst = 200
	}
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		rps:      rps,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	entry, ok := rl.limiters[ip]
	if !ok {
		entry = &ipLimiter{
			limiter: rate.NewLimiter(rate.Limit(rl.rps), rl.burst),
		}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, entry := range rl.limiters {
			if time.Since(entry.lastSeen) > 3*time.Minute {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// NewRateLimit returns a middleware that rate limits per client IP.
func NewRateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			limiter := rl.getLimiter(ip)
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
