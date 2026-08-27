package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	"go.uber.org/zap"
)

const (
	// DefaultNegotiationExpirePollInterval is how often the worker checks for negotiations to expire
	DefaultNegotiationExpirePollInterval = 1 * time.Minute

	// DefaultNegotiationExpireBatchSize is max negotiations to process per batch
	DefaultNegotiationExpireBatchSize = 50
)

// NegotiationExpireWorker automatically expires negotiation sessions that have passed their expires_at time.
//
// NEGOTIATION LIFECYCLE HARDENING:
// - Negotiation is a time-bound agreement layer, NOT a stock reservation system
// - Expiration does NOT affect inventory - stock is only reserved at order creation
// - Expired negotiations cannot proceed to order creation
//
// SAFETY GUARDS:
// - Uses FOR UPDATE SKIP LOCKED for concurrent worker support
// - Only processes negotiations with status IN ('active', 'accepted')
// - Only processes negotiations where expires_at < NOW()
// - Each negotiation is processed in its own transaction
// - Idempotent - safe to run multiple times on same session
//
// NEGOTIATION EXPIRY CONSISTENCY: Includes accepted negotiations to prevent
// "accepted but expired" state which allows checkout of stale agreements.
type NegotiationExpireWorker struct {
	negotiationService *negotiationApp.NegotiationService
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// NegotiationExpireConfig holds worker configuration
type NegotiationExpireConfig struct {
	PollInterval time.Duration // How often to check for negotiations to expire
	BatchSize    int           // Max negotiations to process per batch
}

// DefaultNegotiationExpireConfig returns default configuration
func DefaultNegotiationExpireConfig() NegotiationExpireConfig {
	return NegotiationExpireConfig{
		PollInterval: DefaultNegotiationExpirePollInterval,
		BatchSize:    DefaultNegotiationExpireBatchSize,
	}
}

// NewNegotiationExpireWorker creates a new negotiation expiration worker
func NewNegotiationExpireWorker(
	negotiationService *negotiationApp.NegotiationService,
	log *zap.Logger,
	cfg NegotiationExpireConfig,
) *NegotiationExpireWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultNegotiationExpirePollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultNegotiationExpireBatchSize
	}

	return &NegotiationExpireWorker{
		negotiationService: negotiationService,
		log:                log,
		pollInterval:       cfg.PollInterval,
		batchSize:          cfg.BatchSize,
		stopCh:             make(chan struct{}),
	}
}

// Start begins processing expired negotiations in the background
func (w *NegotiationExpireWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Negotiation expire worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Negotiation expire worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *NegotiationExpireWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping negotiation expire worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Negotiation expire worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Negotiation expire worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *NegotiationExpireWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *NegotiationExpireWorker) run() {
	defer w.wg.Done()

	w.processExpiredNegotiations()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.processExpiredNegotiations()

		case <-w.stopCh:
			return
		}
	}
}

// processExpiredNegotiations finds and expires negotiations that have passed their expires_at time.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker support.
func (w *NegotiationExpireWorker) processExpiredNegotiations() {
	ctx := context.Background()

	// Find negotiations to expire (already locked by the repository)
	sessions, err := w.negotiationService.GetExpiredSessions(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find expired negotiations", zap.Error(err))
		return
	}

	if len(sessions) == 0 {
		return
	}

	w.log.Debug("Processing expired negotiations", zap.Int("count", len(sessions)))

	// Process each session in its own transaction for isolation
	for _, session := range sessions {
		if err := w.processSession(ctx, session.ID); err != nil {
			w.log.Error("Failed to expire negotiation",
				zap.String("session_id", session.ID.String()),
				zap.Error(err),
			)
		}
	}
}

// processSession expires a single negotiation via the negotiation service.
// Runs within its own transaction for isolation.
func (w *NegotiationExpireWorker) processSession(ctx context.Context, sessionID uuid.UUID) error {
	// Use the service method which handles its own transaction
	if err := w.negotiationService.ExpireSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to expire negotiation: %w", err)
	}

	w.log.Debug("Negotiation expired successfully",
		zap.String("session_id", sessionID.String()),
	)

	return nil
}

// ManualProcess triggers immediate processing of expired negotiations.
// Useful for testing or manual intervention.
func (w *NegotiationExpireWorker) ManualProcess(ctx context.Context) error {
	sessions, err := w.negotiationService.GetExpiredSessions(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to find expired negotiations: %w", err)
	}

	if len(sessions) == 0 {
		w.log.Info("No expired negotiations found")
		return nil
	}

	w.log.Info("Manual negotiation expiration processing", zap.Int("count", len(sessions)))

	for _, session := range sessions {
		if err := w.processSession(ctx, session.ID); err != nil {
			w.log.Error("Failed to expire negotiation",
				zap.String("session_id", session.ID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}


