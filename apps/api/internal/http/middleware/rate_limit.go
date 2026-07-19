package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

// RateLimit is a lightweight in-memory token-bucket limiter keyed by client IP.
// It protects unauthenticated, internet-facing endpoints (payment/shipping
// webhooks) from being hammered — either by a misbehaving upstream retrying in a
// tight loop or by an attacker probing the callback endpoints.
//
// It is deliberately simple (single process, no Redis) because webhook volume is
// low and the goal is a safety valve, not precise global rate control. For
// multi-instance precise limiting, back this with Redis.
type RateLimit struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens added per second
	capacity float64 // max burst
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimit builds a limiter that allows `burst` requests instantly and then
// refills at `perSecond` tokens/sec. A background sweeper evicts idle buckets so
// memory doesn't grow unbounded.
func NewRateLimit(perSecond float64, burst int) *RateLimit {
	if perSecond <= 0 {
		perSecond = 5
	}
	if burst <= 0 {
		burst = 10
	}
	rl := &RateLimit{
		buckets:  make(map[string]*bucket),
		rate:     perSecond,
		capacity: float64(burst),
	}
	go rl.sweep()
	return rl
}

// Handler returns the gin middleware. Requests over the limit get 429.
func (rl *RateLimit) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			response.Fail(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		c.Next()
	}
}

// allow reports whether a request from key may proceed, consuming one token.
func (rl *RateLimit) allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &bucket{tokens: rl.capacity - 1, lastSeen: now}
		return true
	}
	// Refill based on elapsed time, capped at capacity.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens = minFloat(rl.capacity, b.tokens+elapsed*rl.rate)
	b.lastSeen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep periodically drops buckets that have been idle so the map doesn't leak.
func (rl *RateLimit) sweep() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		rl.mu.Lock()
		for key, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
