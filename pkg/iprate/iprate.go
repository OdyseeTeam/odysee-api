package iprate

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	ips             map[string]*rateLimiterEntry
	mu              sync.Mutex
	r               rate.Limit
	b               int
	cleanupInterval time.Duration
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type Option func(*Limiter)

func WithCleanupInterval(d time.Duration) Option {
	return func(l *Limiter) {
		l.cleanupInterval = d
	}
}

// NewLimiter creates a new rate limit holder replenishing tokens at rate r and allowing bursts of b.
// Example:
//
// limiter := NewLimiter(rate.Limit(0.05), 3) // 3 tokens per minute with bursts of 3
// clientIP := r.RemoteAddr
// limiter := limiter.GetLimiter(clientIP)
//
//	if !limiter.Allow() {
//		w.Header().Set("Retry-After", "60")
//		http.Error(w, "Too many failed authentication attempts", http.StatusTooManyRequests)
//		return
//	}
func NewLimiter(r rate.Limit, b int, opts ...Option) *Limiter {
	limiter := &Limiter{
		ips:             make(map[string]*rateLimiterEntry),
		r:               r,
		b:               b,
		cleanupInterval: 5 * time.Minute,
	}

	for _, opt := range opts {
		opt(limiter)
	}

	go limiter.cleanupLoop()

	return limiter
}

func (i *Limiter) cleanupLoop() {
	ticker := time.NewTicker(i.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		i.cleanup()
	}
}

func (i *Limiter) cleanup() {
	i.mu.Lock()
	defer i.mu.Unlock()

	expirationTime := time.Now().Add(-1 * time.Hour)
	for ip, entry := range i.ips {
		if entry.lastSeen.Before(expirationTime) {
			delete(i.ips, ip)
		}
	}
}

func (i *Limiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, exists := i.ips[ip]
	if !exists {
		limiter := rate.NewLimiter(i.r, i.b)
		entry = &rateLimiterEntry{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		i.ips[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}

	return entry.limiter
}
