package rate

import (
	"sync"
	"time"
)

// cleanupThreshold is the duration of inactivity before a bucket is deleted.
const cleanupThreshold = 10 * time.Minute

// RateLimiter manages multiple token buckets keyed by string.
//
// KEY FORMAT CONVENTIONS:
// - chat:msg:<userID>      - Message sending rate limit per user
// - chat:room:<userID>     - Room creation rate limit per user
// - ws:sub:<connectionID>  - WebSocket subscription rate limit per connection
//
// DESIGN PRINCIPLES:
// - In-memory only (no DB storage)
// - Automatic cleanup of unused buckets (lazy cleanup on access)
// - Thread-safe with RWMutex
type RateLimiter struct {
	buckets map[string]*bucketEntry
	mu      sync.RWMutex
}

// bucketEntry wraps a Bucket with last access time for cleanup.
type bucketEntry struct {
	bucket *Bucket
	lastAccessed time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucketEntry),
	}
}

// Allow checks if a request is permitted under the rate limit for the given key.
//
// If the bucket doesn't exist, it's created with the specified parameters.
// If the bucket exists and hasn't been used recently, it may be re-created.
//
// key: unique identifier for the rate limit bucket
// capacity: maximum number of tokens the bucket can hold
// refill: time duration between token refills (1 token added per refill)
//
// Returns true if the request is allowed, false if rate limited.
func (rl *RateLimiter) Allow(key string, capacity int, refill time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create bucket entry
	entry, exists := rl.buckets[key]
	if !exists {
		// Create new bucket
		entry = &bucketEntry{
			bucket: NewBucket(capacity, refill),
			lastAccessed: now,
		}
		rl.buckets[key] = entry
		return entry.bucket.Allow()
	}

	// Check if bucket should be cleaned up (inactive > cleanupThreshold)
	if now.Sub(entry.lastAccessed) > cleanupThreshold {
		// Replace with fresh bucket
		entry.bucket = NewBucket(capacity, refill)
		entry.lastAccessed = now
		return entry.bucket.Allow()
	}

	// Update last access time
	entry.lastAccessed = now

	// Check rate limit
	return entry.bucket.Allow()
}

// Cleanup removes all buckets that haven't been accessed recently.
// This is a manual cleanup method - automatic cleanup happens on access.
//
// For V1, lazy cleanup on access is sufficient.
// This method exists for manual cleanup if needed (e.g., monitoring hooks).
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.buckets {
		if now.Sub(entry.lastAccessed) > cleanupThreshold {
			delete(rl.buckets, key)
		}
	}
}

// BucketCount returns the current number of buckets (for monitoring).
func (rl *RateLimiter) BucketCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.buckets)
}
