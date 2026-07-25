package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/api/respond"
)

type visitor struct {
	count     int
	resetTime time.Time
}

// RateLimiter tracks per-IP request frequency using a time window bucket.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

// NewRateLimiter constructs a new IP-based rate limiter and starts a background cleanup goroutine
// to evict expired IP tracking entries and prevent memory leaks.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}

	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.window)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.After(v.resetTime) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// GetClientIP extracts the real client IP address, checking X-Forwarded-For and X-Real-IP headers
// before falling back to RemoteAddr.
func GetClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0])
		}
	}
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Middleware returns the HTTP middleware handler enforcing rate limits.
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)

			rl.mu.Lock()
			v, exists := rl.visitors[ip]
			now := time.Now()

			if !exists || now.After(v.resetTime) {
				rl.visitors[ip] = &visitor{
					count:     1,
					resetTime: now.Add(rl.window),
				}
				rl.mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			v.count++
			if v.count > rl.limit {
				retryAfter := int(time.Until(v.resetTime).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				rl.mu.Unlock()

				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				slog.Warn("auth rate limit exceeded", "ip", ip, "path", r.URL.Path, "limit", rl.limit)
				respond.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded: too many requests, please try again later")
				return
			}
			rl.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit returns an IP-based rate limiting middleware with the specified limit and time window.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(limit, window)
	return limiter.Middleware()
}
