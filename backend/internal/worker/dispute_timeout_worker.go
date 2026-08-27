package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/dispute/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// =============================================================================
// DISPUTE TIMEOUT WORKER — DEADLOCK PREVENTION & FAIRNESS
// =============================================================================
//
// HARD RULE: NO dispute can block escrow/money forever.
//
// This worker enforces the dispute timeout policy:
// - After 3 days: Mark dispute as overdue (for escalation)
// - After timeout_days (default 14): Escalate for manual resolution
//
// BUSINESS TRUTH:
// - Disputes in "under_review" for too long block escrow indefinitely
// - System MUST escalate to prevent money being locked forever
// - Fairness policy: ESCALATE to admin for manual decision (not auto-resolve)
//
// PROHIBITED:
// - Do NOT leave disputes unresolved past timeout
// - Do NOT auto-resolve without admin review (unfair to both parties)
// =============================================================================

const (
	// DefaultDisputeTimeoutPollInterval is how often to check for timed-out disputes
	DefaultDisputeTimeoutPollInterval = 1 * time.Hour
	// DefaultDisputeTimeoutBatchSize is the default number of disputes to process per batch
	DefaultDisputeTimeoutBatchSize = 50
)

// DisputeTimeoutRepository defines the interface for dispute timeout operations.
type DisputeTimeoutRepository interface {
	// FindOverdueCandidates finds disputes that should be marked as overdue.
	FindOverdueCandidates(ctx context.Context, tx db.Tx, limit int) ([]uuid.UUID, error)

	// FindTimeoutCandidates finds disputes that should be auto-resolved.
	FindTimeoutCandidates(ctx context.Context, tx db.Tx, limit int) ([]uuid.UUID, error)
}


// DisputeTimeoutWorker enforces dispute timeout policy to prevent escrow deadlock.
//
// This worker processes two types of disputes:
// 1. Overdue marking: Disputes > 3 days old are marked as overdue for escalation
// 2. Escalation: Disputes > timeout_days old are escalated for admin resolution (fairness policy)
type DisputeTimeoutWorker struct {
	db             Transactor
	repository     DisputeTimeoutRepository
	disputeService *application.DisputeService
	outbox         OutboxInserter
	log            *zap.Logger
	batchSize      int

	// Worker lifecycle
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// Worker identifier for logging
	workerID string

	// Poll interval for periodic processing
	pollInterval time.Duration

	// Policy: "escalate" means mark for manual admin review (fairer than auto-resolve)
	policy string
}

// NewDisputeTimeoutWorker creates a new dispute timeout worker.
func NewDisputeTimeoutWorker(
	db Transactor,
	repository DisputeTimeoutRepository,
	disputeService *application.DisputeService,
	outbox OutboxInserter,
	log *zap.Logger,
) *DisputeTimeoutWorker {
	if log == nil {
		log = zap.NewNop()
	}

	workerID := fmt.Sprintf("dispute-timeout-worker-%s", uuid.New().String()[:8])

	return &DisputeTimeoutWorker{
		db:             db,
		repository:     repository,
		disputeService: disputeService,
		outbox:         outbox,
		log:            log,
		batchSize:      DefaultDisputeTimeoutBatchSize,
		pollInterval:   DefaultDisputeTimeoutPollInterval,
		stopCh:         make(chan struct{}),
		workerID:       workerID,
		policy:         "escalate", // Fairness policy: Escalate for admin review instead of auto-resolve
	}
}

// Start begins processing dispute timeouts in the background.
func (w *DisputeTimeoutWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("DisputeTimeoutWorker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.running = true

	w.wg.Add(1)
	go w.run()

	w.log.Info("DisputeTimeoutWorker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.String("policy", w.policy),
	)
}

// Stop gracefully shuts down the worker.
func (w *DisputeTimeoutWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("DisputeTimeoutWorker stopping",
		zap.String("worker_id", w.workerID),
	)

	// Signal shutdown
	w.cancelFn()
	close(w.stopCh)

	// Wait for run loop to exit
	w.wg.Wait()

	w.running = false

	w.log.Info("DisputeTimeoutWorker stopped",
		zap.String("worker_id", w.workerID),
	)
}

// IsRunning returns true if the worker is currently running.
func (w *DisputeTimeoutWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *DisputeTimeoutWorker) run() {
	defer w.wg.Done()

	// Create ticker for periodic sweeps
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do immediate sweep on startup
	w.processSweep()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Debug("DisputeTimeoutWorker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-ticker.C:
			w.processSweep()

		case <-w.stopCh:
			w.log.Debug("DisputeTimeoutWorker stop signal received",
				zap.String("worker_id", w.workerID),
			)
			return
		}
	}
}

// processSweep executes one sweep of both overdue marking and timeout resolution.
func (w *DisputeTimeoutWorker) processSweep() {
	w.log.Debug("DisputeTimeoutWorker starting sweep",
		zap.String("worker_id", w.workerID),
	)

	startTime := time.Now()

	ctx := w.shutdownCtx

	// Process overdue marking
	if err := w.ProcessOverdueMarking(ctx); err != nil {
		w.log.Error("Overdue marking processing failed", zap.Error(err))
	}

	// Process timeout resolution
	if err := w.ProcessTimeoutResolution(ctx); err != nil {
		w.log.Error("Timeout resolution processing failed", zap.Error(err))
	}

	duration := time.Since(startTime)

	w.log.Debug("DisputeTimeoutWorker sweep completed",
		zap.String("worker_id", w.workerID),
		zap.Duration("duration", duration),
	)
}

// ProcessOverdueMarking marks disputes as overdue (> 3 days old).
func (w *DisputeTimeoutWorker) ProcessOverdueMarking(ctx context.Context) error {
	// Find disputes that need to be marked overdue
	var disputeIDs []uuid.UUID
	var err error

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		disputeIDs, err = w.repository.FindOverdueCandidates(ctx, tx, w.batchSize)
		return err
	})
	if err != nil {
		return fmt.Errorf("find overdue candidates failed: %w", err)
	}

	if len(disputeIDs) == 0 {
		return nil // No work to do
	}

	w.log.Info("Processing overdue dispute marking",
		zap.Int("count", len(disputeIDs)),
	)

	// Process each dispute in its own transaction
	for _, disputeID := range disputeIDs {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			return w.markOverdueOne(ctx, tx, disputeID)
		}); err != nil {
			w.log.Error("Failed to mark dispute as overdue",
				zap.String("dispute_id", disputeID.String()),
				zap.Error(err),
			)
			// Continue processing other disputes
		}
	}

	return nil
}

// markOverdueOne marks a single dispute as overdue.
func (w *DisputeTimeoutWorker) markOverdueOne(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) error {
	// Mark as overdue through service
	if err := w.disputeService.MarkOverdue(ctx, tx, disputeID); err != nil {
		return err
	}

	// Get updated dispute for notification payload
	dispute, err := w.disputeService.GetDispute(ctx, tx, disputeID)
	if err != nil {
		// Already marked, just log
		return nil
	}

	// Emit outbox event for notification
	idempotencyKey := fmt.Sprintf("dispute.overdue.%s", disputeID)
	payload := map[string]any{
		"dispute_id": disputeID,
		"order_id":   dispute.OrderID,
		"buyer_id":   dispute.BuyerID,
		"seller_id":  dispute.SellerID,
		"days_open":  dispute.DaysOpen(),
		"reason":     "Dispute has exceeded escalation threshold (3 days)",
	}

	if err := w.outbox.InsertTx(ctx, tx, "dispute.overdue", payload, idempotencyKey); err != nil {
		return fmt.Errorf("insert outbox event failed: %w", err)
	}

	w.log.Debug("Dispute marked as overdue",
		zap.String("dispute_id", disputeID.String()),
		zap.Int("days_open", dispute.DaysOpen()),
	)

	return nil
}

// ProcessTimeoutResolution escalates disputes that have exceeded their timeout period for admin review.
func (w *DisputeTimeoutWorker) ProcessTimeoutResolution(ctx context.Context) error {
	// Find disputes that need auto-resolution
	var disputeIDs []uuid.UUID
	var err error

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		disputeIDs, err = w.repository.FindTimeoutCandidates(ctx, tx, w.batchSize)
		return err
	})
	if err != nil {
		return fmt.Errorf("find timeout candidates failed: %w", err)
	}

	if len(disputeIDs) == 0 {
		return nil // No work to do
	}

	w.log.Info("Processing dispute timeout escalation",
		zap.Int("count", len(disputeIDs)),
		zap.String("policy", w.policy),
	)

	// Process each dispute in its own transaction
	for _, disputeID := range disputeIDs {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			return w.autoResolveOne(ctx, tx, disputeID)
		}); err != nil {
			w.log.Error("Failed to auto-resolve dispute",
				zap.String("dispute_id", disputeID.String()),
				zap.Error(err),
			)
			// Continue processing other disputes
		}
	}

	return nil
}

// autoResolveOne escalates a single dispute for admin resolution.
// 🔥 TASK 2: Changed from auto-resolve to escalate-only policy for fairness.
func (w *DisputeTimeoutWorker) autoResolveOne(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) error {
	// Get dispute with lock
	dispute, err := w.disputeService.GetDispute(ctx, tx, disputeID)
	if err != nil {
		return fmt.Errorf("get dispute failed: %w", err)
	}

	// Verify still eligible
	if !dispute.ShouldAutoResolve() {
		return nil // No longer eligible, skip
	}

	now := time.Now()

	// 🔥 TASK 2: ESCALATE-ONLY POLICY
	// Instead of auto-resolving (which is unfair), mark dispute as critical escalation
	// This ensures admin reviews the case instead of system deciding who wins
	//
	// RATIONALE:
	// - Auto-resolving to buyer punishes seller who may be right
	// - Auto-resolving to seller punishes buyer who may be right
	// - Only admin can properly evaluate evidence and make fair decision
	//
	// DEADLOCK PREVENTION:
	// - Dispute is marked as "critical_overdue" for prioritization
	// - Admins get alert notifications
	// - System still prevents indefinite blocking

	// Mark dispute as critically overdue for admin priority
	if !dispute.IsOverdue {
		if err := dispute.MarkAsOverdue(now); err != nil {
			return fmt.Errorf("mark as critical overdue failed: %w", err)
		}

		// Update dispute record
		if err := w.disputeService.UpdateDispute(ctx, tx, dispute); err != nil {
			return fmt.Errorf("update dispute failed: %w", err)
		}
	}

	// Emit escalation event for admin notification
	idempotencyKey := fmt.Sprintf("dispute.timeout_escalation.%s", disputeID)
	payload := map[string]any{
		"dispute_id":       disputeID,
		"order_id":         dispute.OrderID,
		"buyer_id":         dispute.BuyerID,
		"seller_id":        dispute.SellerID,
		"days_open":        dispute.DaysOpen(),
		"timeout_days":     dispute.TimeoutDays,
		"escalation_level": "critical",
		"escalated_at":     now,
		"reason":           "Dispute exceeded timeout period - requires immediate admin review",
		"policy":           w.policy,
	}

	if err := w.outbox.InsertTx(ctx, tx, "dispute.timeout_escalation", payload, idempotencyKey); err != nil {
		return fmt.Errorf("insert outbox event failed: %w", err)
	}

	w.log.Info("Dispute escalated for admin review (timeout exceeded)",
		zap.String("dispute_id", disputeID.String()),
		zap.String("order_id", dispute.OrderID.String()),
		zap.Int("days_open", dispute.DaysOpen()),
		zap.String("policy", w.policy),
	)

	return nil
}

// SetBatchSize sets the batch size for processing.
func (w *DisputeTimeoutWorker) SetBatchSize(size int) {
	if size > 0 {
		w.batchSize = size
	}
}



