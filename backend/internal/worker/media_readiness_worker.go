package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultMediaReadinessPollInterval is how often the worker reconciles
	// processing video rows. Two minutes keeps the processing→ready flip
	// near-instant from a user's perspective without HEAD-spamming the CDN.
	DefaultMediaReadinessPollInterval = 2 * time.Minute

	// DefaultMediaReadinessGracePeriod skips brand-new rows: the remux
	// Lambda needs a head start before the first poster probe means
	// anything. Probing earlier only burns CDN requests.
	DefaultMediaReadinessGracePeriod = 3 * time.Minute

	// DefaultMediaReadinessTimeout marks a processing video failed when its
	// poster still doesn't exist this long after creation. Covers Lambda
	// crashes, poison files, and lost S3 events — the sweeper is the only
	// failure detector by design (Lambda never touches the DB).
	DefaultMediaReadinessTimeout = 30 * time.Minute

	// DefaultMediaReadinessBatchSize bounds one sweep per table.
	DefaultMediaReadinessBatchSize = 50

	// DefaultMediaReadinessHeadTimeout bounds a single poster probe.
	DefaultMediaReadinessHeadTimeout = 10 * time.Second
)

// headFunc probes a poster URL: true means the frame exists (HTTP 200).
// Injected for tests; production uses an HTTP HEAD against the CDN URL.
type headFunc func(ctx context.Context, url string) bool

// MediaReadinessConfig holds worker configuration.
type MediaReadinessConfig struct {
	PollInterval time.Duration
	GracePeriod  time.Duration
	Timeout      time.Duration
	BatchSize    int
	HeadTimeout  time.Duration
}

// DefaultMediaReadinessConfig returns default configuration.
func DefaultMediaReadinessConfig() MediaReadinessConfig {
	return MediaReadinessConfig{
		PollInterval: DefaultMediaReadinessPollInterval,
		GracePeriod:  DefaultMediaReadinessGracePeriod,
		Timeout:      DefaultMediaReadinessTimeout,
		BatchSize:    DefaultMediaReadinessBatchSize,
		HeadTimeout:  DefaultMediaReadinessHeadTimeout,
	}
}

// MediaReadinessWorker flips processing video rows to ready once the remux
// Lambda's poster frame exists, or failed past the timeout.
//
// WHY A SWEEPER (not a Lambda callback): the video Lambda is S3-triggered
// and has no DB access, no SQS consumer exists in this codebase, and every
// worker here is already DB-polling on this exact Start/run shape. The
// poster probe is an HTTP HEAD on the derived CloudFront poster URL — the
// same derivation every read surface uses (commerceshared) — so ready means
// exactly "what clients render now 200s". Zero new infrastructure, zero new
// dependencies (net/http only).
//
// SAFETY: only rows with status='processing' are ever touched; ready/failed
// are terminal and never revisited. Products are rewritten only when at
// least one item actually flips.
type MediaReadinessWorker struct {
	db           *db.DB
	log          *zap.Logger
	pollInterval time.Duration
	gracePeriod  time.Duration
	timeout      time.Duration
	batchSize    int
	headTimeout  time.Duration
	head         headFunc

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// NewMediaReadinessWorker creates a new media readiness worker.
func NewMediaReadinessWorker(
	db *db.DB,
	log *zap.Logger,
	cfg MediaReadinessConfig,
) *MediaReadinessWorker {
	if log == nil {
		log = zap.NewNop()
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultMediaReadinessPollInterval
	}
	if cfg.GracePeriod == 0 {
		cfg.GracePeriod = DefaultMediaReadinessGracePeriod
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultMediaReadinessTimeout
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultMediaReadinessBatchSize
	}
	if cfg.HeadTimeout == 0 {
		cfg.HeadTimeout = DefaultMediaReadinessHeadTimeout
	}
	w := &MediaReadinessWorker{
		db:           db,
		log:          log,
		pollInterval: cfg.PollInterval,
		gracePeriod:  cfg.GracePeriod,
		timeout:      cfg.Timeout,
		batchSize:    cfg.BatchSize,
		headTimeout:  cfg.HeadTimeout,
		stopCh:       make(chan struct{}),
	}
	w.head = w.headPoster
	return w
}

// Start begins the worker in the background.
func (w *MediaReadinessWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Media readiness worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Media readiness worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Duration("grace_period", w.gracePeriod),
		zap.Duration("timeout", w.timeout),
	)
}

// Stop gracefully shuts down the worker.
func (w *MediaReadinessWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping media readiness worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Media readiness worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Media readiness worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running.
func (w *MediaReadinessWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *MediaReadinessWorker) run() {
	defer w.wg.Done()

	w.sweep()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.sweep()

		case <-w.stopCh:
			return
		}
	}
}

// decideMediaTarget is the pure transition rule, unit-tested:
//   - poster exists → ready (even past the timeout: late is still ready).
//   - poster missing + older than timeout → failed.
//   - otherwise → keep processing ("", false).
func decideMediaTarget(posterOK bool, age, timeout time.Duration) (string, bool) {
	if posterOK {
		return string(mediaref.MediaStatusReady), true
	}
	if age > timeout {
		return string(mediaref.MediaStatusFailed), true
	}
	return "", false
}

// headPoster probes the derived poster frame. Any error or non-200 means
// "not ready yet" — never failed; the timeout owns the failed verdict.
func (w *MediaReadinessWorker) headPoster(ctx context.Context, posterURL string) bool {
	if posterURL == "" {
		return false
	}
	reqCtx, cancel := context.WithTimeout(ctx, w.headTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, posterURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// sweep reconciles one batch per media table. Errors are logged, never
// fatal — the next tick retries.
func (w *MediaReadinessWorker) sweep() {
	ctx := context.Background()
	now := time.Now().UTC()

	graceCutoff := now.Add(-w.gracePeriod)
	ready, failed := w.sweepTable(ctx,
		`SELECT id, media_url, created_at FROM content_media
		 WHERE status = 'processing' AND media_type = 'video' AND created_at < $1
		 ORDER BY created_at LIMIT $2`,
		`UPDATE content_media SET status = $2 WHERE id = $1 AND status = 'processing'`,
		graceCutoff,
	)
	cReady, cFailed := w.sweepTable(ctx,
		`SELECT id, media_url, created_at FROM comment_media
		 WHERE status = 'processing' AND media_type = 'video' AND created_at < $1
		 ORDER BY created_at LIMIT $2`,
		`UPDATE comment_media SET status = $2 WHERE id = $1 AND status = 'processing'`,
		graceCutoff,
	)
	pReady, pFailed := w.sweepTableProducts(ctx, now)
	w.log.Debug("Media readiness sweep done",
		zap.Int("ready", ready+cReady+pReady),
		zap.Int("failed", failed+cFailed+pFailed),
	)
}

// sweepTable reconciles content_media / comment_media rows shaped
// (id, media_url, created_at). Returns ready/failed counts.
func (w *MediaReadinessWorker) sweepTable(ctx context.Context, selectSQL, updateSQL string, graceCutoff time.Time) (ready, failed int) {
	now := time.Now().UTC()
	rows, err := w.db.Pool().Query(ctx, selectSQL, graceCutoff, w.batchSize)
	if err != nil {
		w.log.Warn("Media readiness select failed", zap.Error(err))
		return 0, 0
	}

	type pendingRow struct {
		id        string
		mediaURL  string
		createdAt time.Time
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.mediaURL, &r.createdAt); err != nil {
			w.log.Warn("Media readiness scan failed", zap.Error(err))
			rows.Close()
			return ready, failed
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		w.log.Warn("Media readiness rows failed", zap.Error(err))
		return ready, failed
	}

	for _, r := range pending {
		posterURL := commerceshared.ResolveReadablePosterURL(r.mediaURL)
		target, flip := decideMediaTarget(w.head(ctx, posterURL), now.Sub(r.createdAt), w.timeout)
		if !flip {
			continue
		}
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, updateSQL, r.id, target)
			return err
		}); err != nil {
			w.log.Warn("Media readiness update failed",
				zap.String("id", r.id), zap.String("target", target), zap.Error(err))
			continue
		}
		if target == string(mediaref.MediaStatusReady) {
			ready++
		} else {
			failed++
		}
	}
	return ready, failed
}

// sweepTableProducts reconciles products.media_urls jsonb items. Item age is
// approximated by the product's updated_at (items carry no timestamps; any
// media rewrite bumps the row). Only rows whose items actually flip are
// rewritten. Returns ready/failed counts.
func (w *MediaReadinessWorker) sweepTableProducts(ctx context.Context, now time.Time) (ready, failed int) {
	rows, err := w.db.Pool().Query(ctx, `
		SELECT id, media_urls, updated_at FROM products
		WHERE media_urls @> '[{"status":"processing"}]' AND updated_at < $1
		ORDER BY updated_at LIMIT $2`,
		now.Add(-w.gracePeriod), w.batchSize)
	if err != nil {
		w.log.Warn("Media readiness products select failed", zap.Error(err))
		return 0, 0
	}

	type pendingProduct struct {
		id        string
		raw       []byte
		updatedAt time.Time
	}
	var pending []pendingProduct
	for rows.Next() {
		var p pendingProduct
		if err := rows.Scan(&p.id, &p.raw, &p.updatedAt); err != nil {
			w.log.Warn("Media readiness products scan failed", zap.Error(err))
			rows.Close()
			return ready, failed
		}
		pending = append(pending, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		w.log.Warn("Media readiness products rows failed", zap.Error(err))
		return ready, failed
	}

	for _, p := range pending {
		var items []productentity.ProductMedia
		if err := json.Unmarshal(p.raw, &items); err != nil {
			w.log.Warn("Media readiness products unmarshal failed",
				zap.String("id", p.id), zap.Error(err))
			continue
		}
		changed := false
		for i := range items {
			if mediaref.NormalizeStatus(items[i].Status) != mediaref.MediaStatusProcessing {
				continue
			}
			if mediaentity.InferMediaType(items[i].URL) == mediaentity.MediaTypeVideo {
				posterURL := commerceshared.ResolveReadablePosterURL(items[i].URL)
				target, flip := decideMediaTarget(w.head(ctx, posterURL), now.Sub(p.updatedAt), w.timeout)
				if !flip {
					continue
				}
				items[i].Status = target
				changed = true
				if target == string(mediaref.MediaStatusReady) {
					ready++
				} else {
					failed++
				}
			} else {
				// Non-video processing slots (should not happen: images
				// are born ready) heal forward instead of sticking.
				items[i].Status = string(mediaref.MediaStatusReady)
				changed = true
				ready++
			}
		}
		if !changed {
			continue
		}
		raw, err := json.Marshal(items)
		if err != nil {
			w.log.Warn("Media readiness products marshal failed",
				zap.String("id", p.id), zap.Error(err))
			continue
		}
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE products SET media_urls = $2 WHERE id = $1`, p.id, string(raw))
			return err
		}); err != nil {
			w.log.Warn("Media readiness products update failed",
				zap.String("id", p.id), zap.Error(err))
		}
	}
	return ready, failed
}
