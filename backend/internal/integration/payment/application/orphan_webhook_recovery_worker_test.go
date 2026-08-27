package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOrphanWebhookRecoveryConfig(t *testing.T) {
	cfg := DefaultOrphanWebhookRecoveryConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 5*time.Second, cfg.RetryInterval)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 30*time.Second, cfg.ScanInterval)
	assert.Equal(t, 2*time.Second, cfg.MinRetryDelay)
}

func TestLoadOrphanWebhookRecoveryConfigFromEnv_DefaultsToOff(t *testing.T) {
	t.Setenv("DISABLE_ORPHAN_WEBHOOK_RECOVERY_WORKER", "")
	cfg := LoadOrphanWebhookRecoveryConfigFromEnv()

	assert.False(t, cfg.Enabled)
}

func TestLoadOrphanWebhookRecoveryConfigFromEnv_EnablesWhenDisableFalse(t *testing.T) {
	t.Setenv("DISABLE_ORPHAN_WEBHOOK_RECOVERY_WORKER", "false")
	t.Setenv("ORPHAN_WEBHOOK_RECOVERY_SCAN_INTERVAL", "45s")
	t.Setenv("ORPHAN_WEBHOOK_RECOVERY_RETRY_INTERVAL", "10s")
	t.Setenv("ORPHAN_WEBHOOK_RECOVERY_MIN_RETRY_DELAY", "3s")
	t.Setenv("ORPHAN_WEBHOOK_RECOVERY_BATCH_SIZE", "42")
	t.Setenv("ORPHAN_WEBHOOK_RECOVERY_MAX_RETRIES", "7")

	cfg := LoadOrphanWebhookRecoveryConfigFromEnv()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 45*time.Second, cfg.ScanInterval)
	assert.Equal(t, 10*time.Second, cfg.RetryInterval)
	assert.Equal(t, 3*time.Second, cfg.MinRetryDelay)
	assert.Equal(t, 42, cfg.BatchSize)
	assert.Equal(t, 7, cfg.MaxRetries)
}

func TestNormalizeOrphanWebhookRecoveryConfigFillsDefaults(t *testing.T) {
	cfg := normalizeOrphanWebhookRecoveryConfig(OrphanWebhookRecoveryConfig{Enabled: true})

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 5*time.Second, cfg.RetryInterval)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 30*time.Second, cfg.ScanInterval)
	assert.Equal(t, 2*time.Second, cfg.MinRetryDelay)
}


