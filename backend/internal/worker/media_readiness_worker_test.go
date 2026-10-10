package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hishumi/backend/internal/pkg/mediaref"
	"go.uber.org/zap/zaptest"
)

// The sweep SQL itself (content_media / comment_media / products.media_urls)
// is proven by the content/commerce repository suites; these tests pin the
// worker's contract: the pure transition rule, canonical defaults, and
// construction/lifecycle invariants.

func TestDecideMediaTarget(t *testing.T) {
	const timeout = 30 * time.Minute
	cases := []struct {
		name     string
		posterOK bool
		age      time.Duration
		want     string
		wantFlip bool
	}{
		{"poster present flips to ready", true, 0, string(mediaref.MediaStatusReady), true},
		{"late poster is still ready", true, 2 * time.Hour, string(mediaref.MediaStatusReady), true},
		{"poster pending inside timeout keeps processing", false, time.Minute, "", false},
		{"poster missing exactly at timeout keeps processing", false, timeout, "", false},
		{"poster missing past timeout flips to failed", false, timeout + time.Second, string(mediaref.MediaStatusFailed), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, flip := decideMediaTarget(tc.posterOK, tc.age, timeout)
			if got != tc.want || flip != tc.wantFlip {
				t.Errorf("decideMediaTarget(%v, %v) = (%q, %v), want (%q, %v)",
					tc.posterOK, tc.age, got, flip, tc.want, tc.wantFlip)
			}
		})
	}
}

func TestMediaReadinessWorker_DefaultConfig(t *testing.T) {
	cfg := DefaultMediaReadinessConfig()
	if cfg.PollInterval != 2*time.Minute {
		t.Errorf("PollInterval = %v, want 2m", cfg.PollInterval)
	}
	if cfg.GracePeriod != 3*time.Minute {
		t.Errorf("GracePeriod = %v, want 3m", cfg.GracePeriod)
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", cfg.Timeout)
	}
	if cfg.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", cfg.BatchSize)
	}
	if cfg.HeadTimeout != 10*time.Second {
		t.Errorf("HeadTimeout = %v, want 10s", cfg.HeadTimeout)
	}
}

func TestMediaReadinessWorker_Construction(t *testing.T) {
	w := NewMediaReadinessWorker(nil, zaptest.NewLogger(t), DefaultMediaReadinessConfig())
	if w == nil {
		t.Fatal("NewMediaReadinessWorker() returned nil")
	}
	if w.IsRunning() {
		t.Error("a freshly constructed worker must not report running")
	}
	if w.head == nil {
		t.Error("head probe must be wired at construction")
	}
	if w.pollInterval != DefaultMediaReadinessPollInterval ||
		w.gracePeriod != DefaultMediaReadinessGracePeriod ||
		w.timeout != DefaultMediaReadinessTimeout ||
		w.batchSize != DefaultMediaReadinessBatchSize ||
		w.headTimeout != DefaultMediaReadinessHeadTimeout {
		t.Errorf("constructed config = {%v %v %v %d %v}, want canonical defaults",
			w.pollInterval, w.gracePeriod, w.timeout, w.batchSize, w.headTimeout)
	}
}

func TestMediaReadinessWorker_ZeroConfigDefaults(t *testing.T) {
	w := NewMediaReadinessWorker(nil, zaptest.NewLogger(t), MediaReadinessConfig{})
	if w.pollInterval != DefaultMediaReadinessPollInterval ||
		w.gracePeriod != DefaultMediaReadinessGracePeriod ||
		w.timeout != DefaultMediaReadinessTimeout ||
		w.batchSize != DefaultMediaReadinessBatchSize ||
		w.headTimeout != DefaultMediaReadinessHeadTimeout {
		t.Errorf("zero config must fall back to canonical defaults, got {%v %v %v %d %v}",
			w.pollInterval, w.gracePeriod, w.timeout, w.batchSize, w.headTimeout)
	}
}

func TestMediaReadinessWorker_NilLogFallback(t *testing.T) {
	w := NewMediaReadinessWorker(nil, nil, DefaultMediaReadinessConfig())
	if w.log == nil {
		t.Fatal("nil logger must fall back to a no-op logger")
	}
}

func TestMediaReadinessWorker_HeadPosterEmptyURL(t *testing.T) {
	w := NewMediaReadinessWorker(nil, zaptest.NewLogger(t), DefaultMediaReadinessConfig())
	if w.headPoster(context.Background(), "") {
		t.Error("empty poster URL must mean not-ready without probing — never failed")
	}
}
