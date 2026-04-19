package http

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// IPRateLimiter is an in-memory per-IP rate limiter (sliding window by request count).
// Safe for concurrent use.
type IPRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
	limit   int
	window  time.Duration
}

type ipEntry struct {
	count   int
	resetAt time.Time
}

// NewIPRateLimiter creates a rate limiter that allows up to limit requests per IP
// within the given window duration.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		entries: make(map[string]*ipEntry),
		limit:   limit,
		window:  window,
	}
}

// Allow reports whether the given IP is within the rate limit.
// It increments the counter and resets expired windows.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[ip]
	if !ok || now.After(e.resetAt) {
		l.entries[ip] = &ipEntry{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	e.count++
	return e.count <= l.limit
}

// Middleware returns an http.Handler middleware that returns 429 when the rate
// limit for the requesting IP is exceeded. It adds a Retry-After header.
func (l *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !l.Allow(ip) {
				retryAfter := int(l.window.Seconds())
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realIP extracts the client IP from RemoteAddr (chi's RealIP middleware should
// have already replaced it with X-Real-IP / X-Forwarded-For when applicable).
func realIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
