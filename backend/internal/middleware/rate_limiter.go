package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter stores rate limiters per IP address
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewIPRateLimiter creates a new IP-based rate limiter
// r: requests per second (e.g., rate.Limit(100) = 100 req/sec)
// b: burst size (max requests that can be made in a short burst)
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
}

// GetLimiter returns the rate limiter for the given IP address
// Creates a new limiter if one doesn't exist for this IP
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rate, i.burst)
		i.limiters[ip] = limiter
	}

	return limiter
}

// CleanupOldLimiters removes limiters that haven't been used recently
// Should be called periodically to prevent memory leaks
func (i *IPRateLimiter) CleanupOldLimiters(maxAge time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Note: golang.org/x/time/rate.Limiter doesn't track last used time
	// For production, consider using a more sophisticated cache with TTL
	// or limit the map size and use LRU eviction

	// For now, we simply clear all limiters if map grows too large
	if len(i.limiters) > 10000 {
		i.limiters = make(map[string]*rate.Limiter)
	}
}

// RateLimitMiddleware creates a Gin middleware for rate limiting
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		ip := c.ClientIP()

		// Get or create rate limiter for this IP
		rateLimiter := limiter.GetLimiter(ip)

		// Check if request is allowed
		if !rateLimiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// RateLimitWithConfig creates a configurable rate limit middleware
type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
	Enabled           bool
}

// ManagedRateLimiter wraps IPRateLimiter with a stoppable cleanup goroutine
// CRITICAL: The Stop() method must be called during graceful shutdown to prevent goroutine leaks
type ManagedRateLimiter struct {
	*IPRateLimiter
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// Stop stops the cleanup goroutine
// CRITICAL: Must be called during graceful shutdown to prevent goroutine leaks
func (m *ManagedRateLimiter) Stop() {
	close(m.stopChan)
	m.wg.Wait()
}

// RateLimitMiddlewareWithConfig creates rate limit middleware with config
// CRITICAL: Returns a ManagedRateLimiter that must be stopped during graceful shutdown
// Example:
//   managedLimiter := RateLimitMiddlewareWithConfig(cfg)
//   defer managedLimiter.Stop() // Stop cleanup goroutine when done
//   router.Use(managedLimiter.Middleware())
func RateLimitMiddlewareWithConfig(cfg RateLimitConfig) *ManagedRateLimiter {
	if !cfg.Enabled {
		// Return a managed limiter with no-op middleware and no cleanup goroutine
		return &ManagedRateLimiter{
			IPRateLimiter: NewIPRateLimiter(0, 0),
			stopChan:      make(chan struct{}),
		}
	}

	limiter := NewIPRateLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst)
	stopChan := make(chan struct{})

	// Start cleanup goroutine to prevent memory leaks
	// CRITICAL: This goroutine can be stopped via Stop()
	managed := &ManagedRateLimiter{
		IPRateLimiter: limiter,
		stopChan:      stopChan,
	}

	managed.wg.Add(1)
	go func() {
		defer managed.wg.Done()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				limiter.CleanupOldLimiters(30 * time.Minute)
			case <-stopChan:
				// Stop goroutine gracefully
				return
			}
		}
	}()

	return managed
}

// Middleware returns the Gin middleware function from a ManagedRateLimiter
func (m *ManagedRateLimiter) Middleware() gin.HandlerFunc {
	if m.IPRateLimiter == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return RateLimitMiddleware(m.IPRateLimiter)
}


