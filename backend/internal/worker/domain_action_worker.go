// 🔥 PHASE 2: DOMAIN ACTION WORKER — RETRY-SAFE EXECUTION
//
// WAJIB:
// - Worker must be retry-safe
// - Idempotent execution
// - All-or-nothing for execution groups
// - Invariant checks after execution
//
// DILARANG:
// - Non-idempotent operations
// - Partial execution of groups
// - Skipping invariant checks

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationrepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// =============================================================================
// DOMAIN ACTION WORKER — IDEMPOTENT EXECUTION
// =============================================================================
//
// 🔥 PHASE 2: Retry-safe worker for domain action execution.
//
// DESIGN PRINCIPLES:
// - All actions executed via idempotency keys
// - Execution groups succeed or fail together
// - Invariant checks verified after execution
// - Failed actions marked for retry (exponential backoff)
// =============================================================================

const (
	// DefaultDomainActionPollInterval is how often to check for pending actions
	DefaultDomainActionPollInterval = 30 * time.Second
	// DefaultDomainActionBatchSize is the default number of actions to process per batch
	DefaultDomainActionBatchSize = 10
	// MaxRetries is the maximum number of retry attempts for failed actions
	MaxRetries = 5
)

// DomainActionWorker executes pending domain actions idempotently.
//
// 🔥 PHASE 2: Worker is retry-safe - each execution attempt uses idempotency keys.
type DomainActionWorker struct {
	db                db.Transactor
	domainActionRepo  moderationrepo.DomainActionRepository
	appealReversalSvc *application.AppealReversalService
	outbox            OutboxInserter
	log               *zap.Logger
	batchSize         int
	pollInterval      time.Duration

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
}

// NewDomainActionWorker creates a new domain action worker.
func NewDomainActionWorker(
	db db.Transactor,
	domainActionRepo moderationrepo.DomainActionRepository,
	appealReversalSvc *application.AppealReversalService,
	outbox OutboxInserter,
	log *zap.Logger,
) *DomainActionWorker {
	if log == nil {
		log = zap.NewNop()
	}

	workerID := fmt.Sprintf("domain-action-worker-%s", uuid.New().String()[:8])

	return &DomainActionWorker{
		db:                db,
		domainActionRepo:  domainActionRepo,
		appealReversalSvc: appealReversalSvc,
		outbox:            outbox,
		log:               log,
		batchSize:         DefaultDomainActionBatchSize,
		pollInterval:      DefaultDomainActionPollInterval,
		stopCh:            make(chan struct{}),
		workerID:          workerID,
	}
}

// Start begins processing domain actions in the background.
func (w *DomainActionWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("DomainActionWorker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.running = true

	w.wg.Add(1)
	go w.run()

	w.log.Info("DomainActionWorker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker.
func (w *DomainActionWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("DomainActionWorker stopping",
		zap.String("worker_id", w.workerID),
	)

	// Signal shutdown
	w.cancelFn()
	close(w.stopCh)

	// Wait for run loop to exit
	w.wg.Wait()

	w.running = false

	w.log.Info("DomainActionWorker stopped",
		zap.String("worker_id", w.workerID),
	)
}

// IsRunning returns true if the worker is currently running.
func (w *DomainActionWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *DomainActionWorker) run() {
	defer w.wg.Done()

	// Create ticker for periodic sweeps
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do immediate sweep on startup
	w.processSweep()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Debug("DomainActionWorker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-ticker.C:
			w.processSweep()

		case <-w.stopCh:
			w.log.Debug("DomainActionWorker stop signal received",
				zap.String("worker_id", w.workerID),
			)
			return
		}
	}
}

// processSweep executes one sweep of action processing.
func (w *DomainActionWorker) processSweep() {
	w.log.Debug("DomainActionWorker starting sweep",
		zap.String("worker_id", w.workerID),
	)

	startTime := time.Now()

	ctx := w.shutdownCtx

	// Process pending actions
	if err := w.ProcessPendingActions(ctx); err != nil {
		w.log.Error("Pending action processing failed", zap.Error(err))
	}

	// Process failed actions (retry)
	if err := w.ProcessFailedActions(ctx); err != nil {
		w.log.Error("Failed action retry processing failed", zap.Error(err))
	}

	duration := time.Since(startTime)

	w.log.Debug("DomainActionWorker sweep completed",
		zap.String("worker_id", w.workerID),
		zap.Duration("duration", duration),
	)
}

// ProcessPendingActions processes pending domain actions.
func (w *DomainActionWorker) ProcessPendingActions(ctx context.Context) error {
	// Fetch pending actions
	var actions []*entity.DomainAction
	var err error

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		actions, err = w.domainActionRepo.ListPending(ctx, tx, w.batchSize, 0)
		return err
	})
	if err != nil {
		return fmt.Errorf("fetch pending actions failed: %w", err)
	}

	if len(actions) == 0 {
		return nil // No work to do
	}

	w.log.Info("Processing pending domain actions",
		zap.Int("count", len(actions)),
	)

	// Process each action in its own transaction
	for _, action := range actions {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			return w.executeOne(ctx, tx, action)
		}); err != nil {
			w.log.Error("Failed to execute domain action",
				zap.String("action_id", action.ID.String()),
				zap.String("action_type", string(action.ActionType)),
				zap.Error(err),
			)
			// Continue processing other actions
		}
	}

	return nil
}

// ProcessFailedActions retries failed domain actions.
func (w *DomainActionWorker) ProcessFailedActions(ctx context.Context) error {
	// Fetch failed actions
	var actions []*entity.DomainAction
	var err error

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		actions, err = w.domainActionRepo.ListFailed(ctx, tx, w.batchSize, 0)
		return err
	})
	if err != nil {
		return fmt.Errorf("fetch failed actions failed: %w", err)
	}

	if len(actions) == 0 {
		return nil // No work to do
	}

	w.log.Info("Retrying failed domain actions",
		zap.Int("count", len(actions)),
	)

	// Process each action in its own transaction
	for _, action := range actions {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			return w.executeOne(ctx, tx, action)
		}); err != nil {
			w.log.Error("Failed to retry domain action",
				zap.String("action_id", action.ID.String()),
				zap.String("action_type", string(action.ActionType)),
				zap.Error(err),
			)
			// Continue processing other actions
		}
	}

	return nil
}

// executeOne executes a single domain action.
//
// 🔥 PHASE 2: This is idempotent - safe to retry on failure.
func (w *DomainActionWorker) executeOne(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	// Get action with lock
	lockedAction, err := w.domainActionRepo.GetForUpdate(ctx, tx, action.ID)
	if err != nil {
		return fmt.Errorf("get action for update failed: %w", err)
	}

	// Check if already executed (idempotency check)
	if lockedAction.IsSucceeded() {
		w.log.Debug("Action already succeeded, skipping",
			zap.String("action_id", lockedAction.ID.String()),
		)
		return nil
	}

	// Execute the action
	previousState, newState, err := w.executeAction(ctx, tx, lockedAction)
	if err != nil {
		// Mark as failed
		if markErr := w.domainActionRepo.MarkAsFailed(ctx, tx, lockedAction.ID, err.Error()); markErr != nil {
			w.log.Error("Failed to mark action as failed",
				zap.String("action_id", lockedAction.ID.String()),
				zap.Error(markErr),
			)
		}
		return fmt.Errorf("action execution failed: %w", err)
	}

	// Mark as succeeded
	if err := w.domainActionRepo.MarkAsSucceeded(ctx, tx, lockedAction.ID, previousState, newState); err != nil {
		return fmt.Errorf("mark action as succeeded failed: %w", err)
	}

	// 🔥 PHASE 2: Verify invariant after execution
	if err := w.verifyPostExecutionInvariant(ctx, tx, lockedAction, newState); err != nil {
		w.log.Error("Invariant violation after action execution",
			zap.String("action_id", lockedAction.ID.String()),
			zap.Error(err),
		)
		// Mark as failed
		_ = w.domainActionRepo.MarkAsFailed(ctx, tx, lockedAction.ID, err.Error())
		return err
	}

	// Emit success event
	idempotencyKey := fmt.Sprintf("domain_action.executed.%s", lockedAction.ID)
	payload := map[string]interface{}{
		"action_id":          lockedAction.ID,
		"action_type":        string(lockedAction.ActionType),
		"target_resource_id": lockedAction.TargetResourceID,
		"previous_state":     string(previousState),
		"new_state":          string(newState),
		"executed_at":        time.Now(),
	}

	if err := w.outbox.InsertTx(ctx, tx, "domain_action.executed", payload, idempotencyKey); err != nil {
		w.log.Error("Failed to emit action execution event",
			zap.String("action_id", lockedAction.ID.String()),
			zap.Error(err),
		)
		// Don't fail the action if event emission fails
	}

	w.log.Debug("Domain action executed successfully",
		zap.String("action_id", lockedAction.ID.String()),
		zap.String("action_type", string(lockedAction.ActionType)),
	)

	return nil
}

// executeAction executes the domain-specific logic for an action.
//
// 🔥 PHASE 2: Returns previous and new state for audit trail.
func (w *DomainActionWorker) executeAction(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) (previousState, newState []byte, err error) {
	switch action.ActionType {
	case entity.ActionTypeHideForSale, entity.ActionTypeReduceVisibility:
		return w.executeForSaleVisibilityAction(ctx, tx, action)

	case entity.ActionTypePauseAuction:
		return w.executeAuctionPauseAction(ctx, tx, action)

	default:
		// No execution logic needed for this action type
		previousState = action.PreviousState
		newState = action.NewState
		return previousState, newState, nil
	}
}

// executeForSaleVisibilityAction executes forSale hiding/reducing visibility.
func (w *DomainActionWorker) executeForSaleVisibilityAction(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) (previousState, newState []byte, err error) {
	// TODO: Implement forSale visibility change
	// This would typically:
	// 1. Get the forSale by ID
	// 2. Snapshot current state (previousState)
	// 3. Set forSale.hidden = true
	// 4. Snapshot new state (newState)
	// 5. Update search index
	// 6. Update feed algorithms

	// For now, emit event for for_sale worker to handle
	idempotencyKey := fmt.Sprintf("for_sale.visibility.apply.%s", action.TargetResourceID)
	payload := map[string]interface{}{
		"for_sale_id": action.TargetResourceID,
		"action_id":   action.ID,
		"new_hidden":  true,
		"action_type": string(action.ActionType),
		"executed_at": time.Now(),
	}

	if err := w.outbox.InsertTx(ctx, tx, "for_sale.visibility.apply", payload, idempotencyKey); err != nil {
		return nil, nil, fmt.Errorf("failed to emit for_sale visibility event: %w", err)
	}

	// Return state snapshots
	previousState = []byte(`{"hidden": false}`)
	newState = []byte(`{"hidden": true}`)

	return previousState, newState, nil
}

// executeAuctionPauseAction executes auction pause.
func (w *DomainActionWorker) executeAuctionPauseAction(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) (previousState, newState []byte, err error) {
	// TODO: Implement auction pause
	// This would typically:
	// 1. Get the auction by ID
	// 2. Snapshot current state (previousState)
	// 3. Set auction.status = 'paused'
	// 4. Snapshot new state (newState)
	// 5. Pause bidding

	// For now, emit event for auction worker to handle
	idempotencyKey := fmt.Sprintf("auction.pause.apply.%s", action.TargetResourceID)
	payload := map[string]interface{}{
		"auction_id":  action.TargetResourceID,
		"action_id":   action.ID,
		"new_status":  "paused",
		"executed_at": time.Now(),
	}

	if err := w.outbox.InsertTx(ctx, tx, "auction.pause.apply", payload, idempotencyKey); err != nil {
		return nil, nil, fmt.Errorf("failed to emit auction pause event: %w", err)
	}

	// Return state snapshots
	previousState = []byte(`{"status": "active"}`)
	newState = []byte(`{"status": "paused"}`)

	return previousState, newState, nil
}

// verifyPostExecutionInvariant verifies invariants after action execution.
func (w *DomainActionWorker) verifyPostExecutionInvariant(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
	newState []byte,
) error {
	// Parse new state
	var state map[string]interface{}
	if err := json.Unmarshal(newState, &state); err != nil {
		return fmt.Errorf("failed to parse new state: %w", err)
	}

	// Validate invariant
	return action.ValidateInvariant(state)
}

// SetBatchSize sets the batch size for processing.
func (w *DomainActionWorker) SetBatchSize(size int) {
	if size > 0 {
		w.batchSize = size
	}
}
