package iprate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestLimiter(t *testing.T) {
	limiter := NewLimiter(rate.Limit(2), 1)

	t.Run("rate limiting works", func(t *testing.T) {
		ip := "192.168.1.1"

		assert.True(t, limiter.GetLimiter(ip).Allow())
		assert.False(t, limiter.GetLimiter(ip).Allow())
		time.Sleep(501 * time.Millisecond)
		assert.True(t, limiter.GetLimiter(ip).Allow())
	})

	t.Run("different IPs get different limits", func(t *testing.T) {
		ip1 := "10.0.0.1"
		ip2 := "10.0.0.2"

		assert.True(t, limiter.GetLimiter(ip1).Allow())
		assert.True(t, limiter.GetLimiter(ip2).Allow())
	})

	t.Run("cleanup removes old entries", func(t *testing.T) {
		testLimiter := &Limiter{
			ips:             make(map[string]*rateLimiterEntry),
			r:               rate.Limit(1),
			b:               1,
			cleanupInterval: time.Millisecond,
		}

		testLimiter.mu.Lock()
		testLimiter.ips["old-ip"] = &rateLimiterEntry{
			limiter:  rate.NewLimiter(rate.Limit(1), 1),
			lastSeen: time.Now().Add(-2 * time.Hour),
		}
		testLimiter.mu.Unlock()

		testLimiter.GetLimiter("new-ip")

		testLimiter.cleanup()

		testLimiter.mu.Lock()
		_, oldExists := testLimiter.ips["old-ip"]
		_, newExists := testLimiter.ips["new-ip"]
		testLimiter.mu.Unlock()

		assert.False(t, oldExists)
		assert.True(t, newExists)
	})
}
