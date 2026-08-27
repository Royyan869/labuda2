package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// =============================================================================
// SLA ESCALATION WORKER — TIMELY RESPONSE NOTIFICATIONS
// =============================================================================
//
// This worker enforces SLA policies for support tickets and disputes:
// - First Response: Warning at 45min, Breach at 60min
// - Resolution: Warning at 75% SLA, Breach at 100% SLA
//
// IDEMPOTENCY VIA EVENT LAYER:
// - Checks outbox table for existing events before emitting
// - Only 1 warning per ticket/dispute per stage
// - Only 1 breach per ticket/dispute per stage
// - No database state in domain entities
//
// EVENTS EMITTED:
// - ticket.sla.warning.first_response
// - ticket.sla.breach.first_response
// - ticket.sla.warning.resolution
// - ticket.sla.breach.resolution
// - dispute.sla.warning.first_response
// - dispute.sla.breach.first_response
// - dispute.sla.warning.resolution
// - dispute.sla.breach.resolution
// =============================================================================

const (
	// DefaultSLAEscalationPollInterval is how often to check for SLA violations
	DefaultSLAEscalationPollInterval = 5 * time.Minute
	// DefaultSLAEscalationBatchSize is the default number of tickets/disputes to process per batch
	DefaultSLAEscalationBatchSize = 50
)

// SLA thresholds for support tickets
const (
	// First response SLA thresholds
	TicketFirstResponseWarningThreshold = 45 * time.Minute
	TicketFirstResponseBreachThreshold  = 60 * time.Minute

	// Resolution SLA thresholds (24 hour resolution SLA)
	TicketResolutionWarningThreshold = 18 * time.Hour // 75% of 24 hours
	TicketResolutionBreachThreshold  = 24 * time.Hour // 100% of 24 hours
)

// SLA thresholds for disputes
const (
	// First response SLA thresholds
	DisputeFirstResponseWarningThreshold = 90 * time.Minute  // 1.5 hours
	DisputeFirstResponseBreachThreshold  = 120 * time.Minute // 2 hours

	// Resolution SLA thresholds (48 hour resolution SLA)
	DisputeResolutionWarningThreshold = 36 * time.Hour // 75% of 48 hours
	DisputeResolutionBreachThreshold  = 48 * time.Hour // 100% of 48 hours
)

// SLAEscalationRepository defines the interface for SLA escalation operations.
//
// TYPED BOUNDARY: returns strongly-typed rows (TicketSLARow / DisputeSLARow)
// rather than map[string]interface{}. This eliminates an entire class of
// runtime panic on type drift: the worker can no longer be crashed by a
// scan that returns uuid.UUID where the worker expected string, or
// *time.Time where it expected time.Time.
type SLAEscalationRepository interface {
	// FindTicketsForSLACheck finds tickets that need SLA checking.
	// Returns tickets that are in open states and haven't been resolved/closed.
	FindTicketsForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.TicketSLARow, error)

	// FindDisputesForSLACheck finds disputes that need SLA checking.
	// Returns disputes that are under review and haven't been resolved.
	FindDisputesForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.DisputeSLARow, error)
}

// SLAEscalationWorker enforces SLA policies for tickets and disputes.
//
// This worker processes both support tickets and disputes:
// 1. First response warnings and breaches
// 2. Resolution warnings and breaches
//
// All events are emitted to the outbox for notification processing.
// IDEMPOTENCY: Checks outbox table for existing events before emitting.
type SLAEscalationWorker struct {
	db        Transactor
	repo      SLAEscalationRepository
	outbox    OutboxInserter
	log       *zap.Logger
	batchSize int

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
}

// NewSLAEscalationWorker creates a new SLA escalation worker.
func NewSLAEscalationWorker(
	db Transactor,
	repo SLAEscalationRepository,
	outbox OutboxInserter,
	log *zap.Logger,
) *SLAEscalationWorker {
	if log == nil {
		log = zap.NewNop()
	}

	workerID := fmt.Sprintf("sla-escalation-worker-%s", uuid.New().String()[:8])

	return &SLAEscalationWorker{
		db:           db,
		repo:         repo,
		outbox:       outbox,
		log:          log,
		batchSize:    DefaultSLAEscalationBatchSize,
		pollInterval: DefaultSLAEscalationPollInterval,
		stopCh:       make(chan struct{}),
		workerID:     workerID,
	}
}

// Start begins processing SLA escalations in the background.
func (w *SLAEscalationWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("SLAEscalationWorker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.running = true

	w.wg.Add(1)
	go w.run()

	w.log.Info("SLAEscalationWorker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker.
func (w *SLAEscalationWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("SLAEscalationWorker stopping",
		zap.String("worker_id", w.workerID),
	)

	// Signal shutdown
	w.cancelFn()
	close(w.stopCh)

	// Wait for run loop to exit
	w.wg.Wait()

	w.running = false

	w.log.Info("SLAEscalationWorker stopped",
		zap.String("worker_id", w.workerID),
	)
}

// IsRunning returns true if the worker is currently running.
func (w *SLAEscalationWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *SLAEscalationWorker) run() {
	defer w.wg.Done()

	// Create ticker for periodic sweeps
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do immediate sweep on startup
	w.processSweep()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Debug("SLAEscalationWorker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-ticker.C:
			w.processSweep()

		case <-w.stopCh:
			w.log.Debug("SLAEscalationWorker stop signal received",
				zap.String("worker_id", w.workerID),
			)
			return
		}
	}
}

// processSweep executes one sweep of all SLA checks.
func (w *SLAEscalationWorker) processSweep() {
	w.log.Debug("SLAEscalationWorker starting sweep",
		zap.String("worker_id", w.workerID),
	)

	startTime := time.Now()

	ctx := w.shutdownCtx

	// Process support tickets
	if err := w.ProcessTicketsSLA(ctx); err != nil {
		w.log.Error("Ticket SLA processing failed", zap.Error(err))
	}

	// Process disputes
	if err := w.ProcessDisputesSLA(ctx); err != nil {
		w.log.Error("Dispute SLA processing failed", zap.Error(err))
	}

	duration := time.Since(startTime)

	w.log.Debug("SLAEscalationWorker sweep completed",
		zap.String("worker_id", w.workerID),
		zap.Duration("duration", duration),
	)
}

// ProcessTicketsSLA checks SLA compliance for support tickets.
func (w *SLAEscalationWorker) ProcessTicketsSLA(ctx context.Context) error {
	// Find tickets that need SLA checking
	var tickets []supportRepo.TicketSLARow
	var err error

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		tickets, err = w.repo.FindTicketsForSLACheck(ctx, tx, w.batchSize)
		return err
	})
	if err != nil {
		return fmt.Errorf("find tickets for SLA check failed: %w", err)
	}

	if len(tickets) == 0 {
		return nil // No work to do
	}

	w.log.Debug("Checking SLA for tickets",
		zap.Int("count", len(tickets)),
	)

	// Process each ticket
	for _, ticket := range tickets {
		if err := w.checkTicketSLA(ctx, ticket); err != nil {
			w.log.Error("Failed to check ticket SLA",
				zap.String("ticket_id", ticket.ID.String()),
				zap.Error(err),
			)
			// Continue processing other tickets
		}
	}

	return nil
}

// ProcessDisputesSLA checks SLA compliance for disputes.
func (w *SLAEscalationWorker) ProcessDisputesSLA(ctx context.Context) error {
	// Find disputes that need SLA checking
	var disputes []supportRepo.DisputeSLARow
	var err error

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		disputes, err = w.repo.FindDisputesForSLACheck(ctx, tx, w.batchSize)
		return err
	})
	if err != nil {
		return fmt.Errorf("find disputes for SLA check failed: %w", err)
	}

	if len(disputes) == 0 {
		return nil // No work to do
	}

	w.log.Debug("Checking SLA for disputes",
		zap.Int("count", len(disputes)),
	)

	// Process each dispute
	for _, dispute := range disputes {
		if err := w.checkDisputeSLA(ctx, dispute); err != nil {
			w.log.Error("Failed to check dispute SLA",
				zap.String("dispute_id", dispute.ID.String()),
				zap.Error(err),
			)
			// Continue processing other disputes
		}
	}

	return nil
}

// checkTicketSLA checks SLA for a single ticket and sends events.
func (w *SLAEscalationWorker) checkTicketSLA(ctx context.Context, ticket supportRepo.TicketSLARow) error {
	// Defensive validation: a zero UUID indicates upstream scan drift; bail
	// out deterministically instead of emitting events keyed to the nil UUID.
	if ticket.ID == uuid.Nil {
		return fmt.Errorf("ticket row has nil UUID — refusing to process")
	}

	ticketID := ticket.ID
	createdAt := ticket.CreatedAt
	hasFirstResponse := ticket.AssignedAt != nil && !ticket.AssignedAt.IsZero()
	isResolved := ticket.ResolvedAt != nil && !ticket.ResolvedAt.IsZero()

	// Check first response SLA (only if not yet responded)
	if !hasFirstResponse {
		elapsed := time.Since(createdAt)

		// Check for breach
		if elapsed >= TicketFirstResponseBreachThreshold {
			if !w.eventExists(ctx, "ticket.sla.breach.first_response", ticketID) {
				w.log.Info("Ticket first response SLA breach",
					zap.String("ticket_id", ticketID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.breach.first_response", map[string]any{
					"ticket_id": ticketID,
					"user_id":   ticket.UserID,
					"elapsed":   elapsed.String(),
					"threshold": TicketFirstResponseBreachThreshold.String(),
				}, ticketID.String()); err != nil {
					return err
				}
			}
		} else if elapsed >= TicketFirstResponseWarningThreshold {
			// Check for warning
			if !w.eventExists(ctx, "ticket.sla.warning.first_response", ticketID) {
				w.log.Info("Ticket first response SLA warning",
					zap.String("ticket_id", ticketID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.warning.first_response", map[string]any{
					"ticket_id": ticketID,
					"user_id":   ticket.UserID,
					"elapsed":   elapsed.String(),
					"threshold": TicketFirstResponseWarningThreshold.String(),
				}, ticketID.String()); err != nil {
					return err
				}
			}
		}
	}

	// Check resolution SLA (only if not yet resolved)
	if !isResolved {
		elapsed := time.Since(createdAt)

		// Check for breach
		if elapsed >= TicketResolutionBreachThreshold {
			if !w.eventExists(ctx, "ticket.sla.breach.resolution", ticketID) {
				w.log.Info("Ticket resolution SLA breach",
					zap.String("ticket_id", ticketID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.breach.resolution", map[string]any{
					"ticket_id": ticketID,
					"user_id":   ticket.UserID,
					"elapsed":   elapsed.String(),
					"threshold": TicketResolutionBreachThreshold.String(),
				}, ticketID.String()); err != nil {
					return err
				}
			}
		} else if elapsed >= TicketResolutionWarningThreshold {
			// Check for warning
			if !w.eventExists(ctx, "ticket.sla.warning.resolution", ticketID) {
				w.log.Info("Ticket resolution SLA warning",
					zap.String("ticket_id", ticketID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.warning.resolution", map[string]any{
					"ticket_id": ticketID,
					"user_id":   ticket.UserID,
					"elapsed":   elapsed.String(),
					"threshold": TicketResolutionWarningThreshold.String(),
				}, ticketID.String()); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// checkDisputeSLA checks SLA for a single dispute and sends events.
func (w *SLAEscalationWorker) checkDisputeSLA(ctx context.Context, dispute supportRepo.DisputeSLARow) error {
	// Defensive validation: a zero UUID indicates upstream scan drift; bail
	// out deterministically instead of emitting events keyed to the nil UUID.
	if dispute.ID == uuid.Nil {
		return fmt.Errorf("dispute row has nil UUID — refusing to process")
	}

	disputeID := dispute.ID
	openedAt := dispute.OpenedAt
	isResolved := dispute.ResolvedAt != nil && !dispute.ResolvedAt.IsZero()

	// Check first response SLA (only if not yet resolved)
	if !isResolved {
		elapsed := time.Since(openedAt)

		// Check for breach
		if elapsed >= DisputeFirstResponseBreachThreshold {
			if !w.eventExists(ctx, "dispute.sla.breach.first_response", disputeID) {
				w.log.Info("Dispute first response SLA breach",
					zap.String("dispute_id", disputeID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.breach.first_response", map[string]any{
					"dispute_id": disputeID,
					"order_id":   dispute.OrderID,
					"buyer_id":   dispute.BuyerID,
					"seller_id":  dispute.SellerID,
					"elapsed":    elapsed.String(),
					"threshold":  DisputeFirstResponseBreachThreshold.String(),
				}, disputeID.String()); err != nil {
					return err
				}
			}
		} else if elapsed >= DisputeFirstResponseWarningThreshold {
			// Check for warning
			if !w.eventExists(ctx, "dispute.sla.warning.first_response", disputeID) {
				w.log.Info("Dispute first response SLA warning",
					zap.String("dispute_id", disputeID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.warning.first_response", map[string]any{
					"dispute_id": disputeID,
					"order_id":   dispute.OrderID,
					"buyer_id":   dispute.BuyerID,
					"seller_id":  dispute.SellerID,
					"elapsed":    elapsed.String(),
					"threshold":  DisputeFirstResponseWarningThreshold.String(),
				}, disputeID.String()); err != nil {
					return err
				}
			}
		}
	}

	// Check resolution SLA (only if not yet resolved)
	if !isResolved {
		elapsed := time.Since(openedAt)

		// Check for breach
		if elapsed >= DisputeResolutionBreachThreshold {
			if !w.eventExists(ctx, "dispute.sla.breach.resolution", disputeID) {
				w.log.Info("Dispute resolution SLA breach",
					zap.String("dispute_id", disputeID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.breach.resolution", map[string]any{
					"dispute_id": disputeID,
					"order_id":   dispute.OrderID,
					"buyer_id":   dispute.BuyerID,
					"seller_id":  dispute.SellerID,
					"elapsed":    elapsed.String(),
					"threshold":  DisputeResolutionBreachThreshold.String(),
				}, disputeID.String()); err != nil {
					return err
				}
			}
		} else if elapsed >= DisputeResolutionWarningThreshold {
			// Check for warning
			if !w.eventExists(ctx, "dispute.sla.warning.resolution", disputeID) {
				w.log.Info("Dispute resolution SLA warning",
					zap.String("dispute_id", disputeID.String()),
					zap.Duration("elapsed", elapsed),
				)

				if err := w.emitEvent(ctx, "dispute.sla.warning.resolution", map[string]any{
					"dispute_id": disputeID,
					"order_id":   dispute.OrderID,
					"buyer_id":   dispute.BuyerID,
					"seller_id":  dispute.SellerID,
					"elapsed":    elapsed.String(),
					"threshold":  DisputeResolutionWarningThreshold.String(),
				}, disputeID.String()); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// eventExists checks if an SLA event has already been emitted for a given entity.
// Uses the outbox table as the source of truth for idempotency.
//
// RACE-SAFE: Uses exact idempotency_key match instead of LIKE pattern.
// PERFORMANT: Limits query to events from the last 7 days.
func (w *SLAEscalationWorker) eventExists(ctx context.Context, eventType string, entityID uuid.UUID) bool {
	var exists bool

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		// Check outbox table for existing event with exact idempotency key match
		// Uses exact match for race-safe idempotency check
		// Limits to last 7 days for performance
		query := `
			SELECT EXISTS(
				SELECT 1
				FROM outbox
				WHERE idempotency_key = $1
				AND created_at >= NOW() - INTERVAL '7 days'
				LIMIT 1
			)
		`

		// Build exact idempotency key: {event_type}.{entity_id}
		// For example: "ticket.sla.warning.first_response.123e4567-e89b-12d3-a456-426614174000"
		idempotencyKey := fmt.Sprintf("%s.%s", eventType, entityID.String())

		return tx.QueryRow(ctx, query, idempotencyKey).Scan(&exists)
	})

	if err != nil {
		w.log.Warn("Failed to check event existence, assuming not exists",
			zap.String("event_type", eventType),
			zap.String("entity_id", entityID.String()),
			zap.Error(err),
		)
		return false
	}

	return exists
}

// emitEvent emits an event to the outbox for notification processing.
func (w *SLAEscalationWorker) emitEvent(ctx context.Context, eventType string, payload map[string]any, idempotencyKey string) error {
	// Emit to outbox
	if err := w.db.WithTx(ctx, func(tx db.Tx) error {
		return w.outbox.InsertTx(ctx, tx, eventType, payload, idempotencyKey)
	}); err != nil {
		return fmt.Errorf("insert outbox event failed: %w", err)
	}
	w.log.Debug("SLA event emitted",
		zap.String("event_type", eventType),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}


