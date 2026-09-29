package worker

// ChatMediaCleanupWorker — TTL sweep for the chat media pipeline.
//
// Chat media is register → upload → attach: the register step
// (POST /chat/rooms/:room_id/media) creates a PENDING chat_media_assets row
// before any byte reaches S3, and the message that references the asset flips it
// to finalized INSIDE the send transaction. That leaves exactly one class of
// garbage: an upload that was registered — and possibly PUT to S3 — but never
// attached (user cancelled, app killed, message rejected).
//
// This worker marks those rows deleted once the 24h pending window closes, so
// the table never accumulates unattached authority and an abandoned upload can
// never be attached later.
//
// FINALIZED assets are message content and are NEVER swept: the TTL applies to
// the pending window only.
//
// S3 objects: a presigned PUT that DID happen for an expired pending asset may
// leave an object in the bucket. Object removal is owned by the bucket lifecycle
// policy (the same rule every other presigned upload in the platform relies on);
// this worker owns the DATABASE authority, which is what every read path
// consults — a swept row makes the object unreachable by construction.
//
// Lifecycle: Start() / Stop() / IsRunning() satisfy serverboot.Worker.

import (
	"context"
	"fmt"
	"sync"
	"time"

	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultChatMediaCleanupInterval is how often the sweep runs. The pending
	// window is 24h, so hourly resolution is more than enough.
	DefaultChatMediaCleanupInterval = 1 * time.Hour

	// chatMediaCleanupTimeout bounds a single sweep cycle.
	chatMediaCleanupTimeout = 5 * time.Minute
)

// ChatMediaCleanupWorker sweeps expired PENDING chat media assets.
type ChatMediaCleanupWorker struct {
	db  *dbpkg.DB
	log *zap.Logger

	interval time.Duration

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewChatMediaCleanupWorker creates the worker with the default interval.
func NewChatMediaCleanupWorker(db *dbpkg.DB, log *zap.Logger) *ChatMediaCleanupWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &ChatMediaCleanupWorker{
		db:       db,
		log:      log,
		interval: DefaultChatMediaCleanupInterval,
	}
}

// SetInterval overrides how often the sweep runs. Call before Start.
func (w *ChatMediaCleanupWorker) SetInterval(interval time.Duration) {
	if interval > 0 {
		w.interval = interval
	}
}

// Start begins the sweep loop in the background. Idempotent.
func (w *ChatMediaCleanupWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("ChatMediaCleanupWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.log.Info("ChatMediaCleanupWorker started",
		zap.Duration("interval", w.interval),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
func (w *ChatMediaCleanupWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.log.Info("ChatMediaCleanupWorker stopped")
}

// IsRunning reports whether the sweep loop is active.
func (w *ChatMediaCleanupWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *ChatMediaCleanupWorker) run() {
	defer w.wg.Done()

	// Sweep once immediately: a deploy after downtime should not wait a full
	// interval to clean up.
	w.runCleanup(w.shutdownCtx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.runCleanup(w.shutdownCtx)
		}
	}
}

func (w *ChatMediaCleanupWorker) runCleanup(ctx context.Context) {
	start := time.Now()

	cleanupCtx, cancel := context.WithTimeout(ctx, chatMediaCleanupTimeout)
	defer cancel()

	swept, err := w.SweepExpiredPendingAssets(cleanupCtx)
	if err != nil {
		w.log.Error("ChatMediaCleanupWorker: sweep failed", zap.Error(err))
		return
	}

	w.log.Info("ChatMediaCleanupWorker cycle complete",
		zap.Int64("expired_pending_assets_swept", swept),
		zap.Duration("elapsed", time.Since(start)),
	)
}

// SweepExpiredPendingAssets marks every PENDING asset whose upload window closed
// as deleted and returns how many rows were swept.
//
// Only `status = 'pending'` rows are eligible: a finalized asset is content
// referenced by a message and must never be swept by TTL. Exported so a single
// deterministic cycle can be driven by tests and ops tooling.
func (w *ChatMediaCleanupWorker) SweepExpiredPendingAssets(ctx context.Context) (int64, error) {
	result, err := w.db.Pool().Exec(ctx, `
		UPDATE chat_media_assets
		SET status = $1, deleted_at = NOW(), deletion_reason = 'expired'
		WHERE status = $2 AND expires_at <= NOW()
	`,
		string(chatEntity.ChatMediaAssetStatusDeleted),
		string(chatEntity.ChatMediaAssetStatusPending),
	)
	if err != nil {
		return 0, fmt.Errorf("sweep expired chat media assets failed: %w", err)
	}

	return result.RowsAffected(), nil
}
