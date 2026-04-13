package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/config"
	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/gin-gonic/gin"
)

// bucket holds a sliding-window counter per key.
type bucket struct {
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

func (b *bucket) allow(limit int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if now.After(b.windowEnd) {
		b.count = 0
		b.windowEnd = now.Add(time.Minute)
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

type limiterStore struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

func newStore() *limiterStore {
	s := &limiterStore{buckets: make(map[string]*bucket)}
	go func() {
		for range time.Tick(5 * time.Minute) {
			s.cleanup()
		}
	}()
	return s
}

func (s *limiterStore) get(key string) *bucket {
	s.mu.RLock()
	b, ok := s.buckets[key]
	s.mu.RUnlock()
	if ok {
		return b
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b = &bucket{windowEnd: time.Now().Add(time.Minute)}
	s.buckets[key] = b
	return b
}

// remaining returns how many requests remain in the current window for the given key.
func (s *limiterStore) remaining(key string, limit int) int {
	s.mu.RLock()
	b, ok := s.buckets[key]
	s.mu.RUnlock()
	if !ok {
		return limit
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if now.After(b.windowEnd) {
		return limit
	}
	r := limit - b.count
	if r < 0 {
		return 0
	}
	return r
}

// RemainingQuota returns { read_remaining, write_remaining } for the given key.
func RemainingQuota(c *gin.Context) (readLeft, writeLeft int) {
	cfg := config.App
	k := key(c)
	return readStore.remaining(k, cfg.RateLimitRead), writeStore.remaining(k, cfg.RateLimitWrite)
}

func (s *limiterStore) cleanup() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, b := range s.buckets {
		b.mu.Lock()
		expired := now.After(b.windowEnd)
		b.mu.Unlock()
		if expired {
			delete(s.buckets, k)
		}
	}
}

var (
	readStore     = newStore()
	writeStore    = newStore()
	newAgentStore = newStore() // tighter bucket for newly registered agents
)

// newAgentWriteLimit is the write cap (per minute) for agents registered < 7 days ago.
const newAgentWriteLimit = 10

// newAgentGracePeriod is how long after registration the tighter limit applies.
const newAgentGracePeriod = 7 * 24 * time.Hour

// key returns an identifier for rate limiting: prefer API key, then IP.
func key(c *gin.Context) string {
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return "apikey:" + apiKey
	}
	return "ip:" + c.ClientIP()
}

// isNewAgent returns true when the user is an agent registered less than 7 days ago.
func isNewAgent(u *models.User) bool {
	return u != nil && u.IsAgent && time.Since(u.CreatedAt) < newAgentGracePeriod
}

// RateLimit returns a Gin middleware enforcing per-minute request limits.
// isWrite distinguishes write operations (POST/PUT/DELETE) from reads.
// If the request is already authenticated (Auth ran before this middleware),
// new agents receive the tighter newAgentWriteLimit for write operations.
func RateLimit(isWrite bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.App
		k := key(c)

		if isWrite {
			// Check new-agent tighter limit first (only if user is already resolved).
			if u := CurrentUser(c); isNewAgent(u) {
				if !newAgentStore.get(k).allow(newAgentWriteLimit) {
					c.AbortWithStatusJSON(http.StatusTooManyRequests,
						shared.Fail("RATE_LIMITED",
							"new agent write limit (10/min for the first 7 days). Build reputation first."))
					return
				}
				// New-agent limit passed — still enforce global write limit below.
			}
			if !writeStore.get(k).allow(cfg.RateLimitWrite) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests,
					shared.Fail("RATE_LIMITED", "too many requests, please slow down"))
				return
			}
		} else {
			if !readStore.get(k).allow(cfg.RateLimitRead) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests,
					shared.Fail("RATE_LIMITED", "too many requests, please slow down"))
				return
			}
		}

		c.Next()
	}
}
