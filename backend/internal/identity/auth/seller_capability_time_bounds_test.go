// Tests for PASS_18U: seller subscription authority must be time-aware, not
// status-only. The runtime gate is active-only and must deny capability once
// now reaches expires_at, regardless of any stale persisted status.
package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHasTimeBoundedSellerCapability_ActiveBeforeExpiry_Grants(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-1 * time.Hour)
	expiresAt := now.Add(24 * time.Hour)

	assert.True(t, hasTimeBoundedSellerCapability("active", startedAt, expiresAt, now))
}

func TestHasTimeBoundedSellerCapability_ActiveAfterExpiry_Denies(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-48 * time.Hour)
	expiresAt := now.Add(-1 * time.Minute)

	assert.False(t, hasTimeBoundedSellerCapability("active", startedAt, expiresAt, now))
}

func TestHasTimeBoundedSellerCapability_NonActiveStatuses_AlwaysDenied(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-24 * time.Hour)
	expiresAt := now.Add(24 * time.Hour)

	for _, status := range []string{"inactive", "expired", "unknown", ""} {
		assert.False(t, hasTimeBoundedSellerCapability(status, startedAt, expiresAt, now))
	}
}

func TestHasTimeBoundedSellerCapability_ZeroTimestamp_FailsSafe(t *testing.T) {
	now := time.Now()
	var zero time.Time

	assert.False(t, hasTimeBoundedSellerCapability("active", zero, zero, now))
}

func TestHasTimeBoundedSellerCapability_BoundaryIsExclusive(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-24 * time.Hour)

	assert.False(t, hasTimeBoundedSellerCapability("active", startedAt, now, now))
	assert.True(t, hasTimeBoundedSellerCapability("active", startedAt, now.Add(time.Nanosecond), now))
}

func TestHasTimeBoundedSellerCapability_WorkerDisabledScenario(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-60 * 24 * time.Hour)
	expiresAt := now.Add(-30 * 24 * time.Hour)

	assert.False(t, hasTimeBoundedSellerCapability("active", startedAt, expiresAt, now))
}
