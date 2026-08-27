package rate

import (
	"sync"
	"time"
)

// Bucket implements a token bucket rate limiter.
//
// DESIGN PRINCIPLES:
// - Deterministic refill logic (no goroutines)
// - Thread-safe with mutex
// - Auto-refills on each Allow() call
//
// REFILL STRATEGY:
// - Tokens refill at a constant rate
// - Each refill adds 1 token per refill duration
// - Tokens never exceed capacity
type Bucket struct {
	capacity int         // Maximum number of tokens
	tokens   int         // Current token count
	refill   time.Duration // Time between token additions
	last     time.Time   // Last time tokens were refilled
	mu       sync.Mutex  // Protects all fields
}

// NewBucket creates a new token bucket.
//
// capacity: maximum number of tokens the bucket can hold
// refill: time duration between token refills (1 token added per refill)
func NewBucket(capacity int, refill time.Duration) *Bucket {
	return &Bucket{
		capacity: capacity,
		tokens:   capacity, // Start with full bucket
		refill:   refill,
		last:     time.Now(),
	}
}

// Allow checks if a request is permitted under the rate limit.
//
// Returns true if the request is allowed (consumes 1 token).
// Returns false if rate limit exceeded (no tokens consumed).
//
// This method is thread-safe and can be called concurrently.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	// Calculate how many tokens to add based on elapsed time
	// This is deterministic: each refill duration adds 1 token
	elapsed := now.Sub(b.last)
	if elapsed >= b.refill {
		// Calculate number of tokens to add (integer division)
		tokensToAdd := int(elapsed / b.refill)

		// Add tokens, but don't exceed capacity
		b.tokens += tokensToAdd
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}

		// Update last refill time
		// Move forward by complete refill cycles only
		b.last = b.last.Add(time.Duration(tokensToAdd) * b.refill)
	}

	// Check if we have tokens available
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// Tokens returns the current token count (for testing/monitoring).
func (b *Bucket) Tokens() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens
}
