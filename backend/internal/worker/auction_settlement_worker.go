package worker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	commercegov "github.com/labuda/backend/internal/commerce/governance/commercegov"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultAuctionSettlementPollInterval is how often the worker checks for
	// expired settlements.
	DefaultAuctionSettlementPollInterval = 5 * time.Minute

	// DefaultAuctionSettlementBatchSize is max auctions to process per poll.
	DefaultAuctionSettlementBatchSize = 50
)

// AuctionSettlementWorker detects auctions whose canonical settlement
// shipping deadline (auction.end_at + 24h) has passed without shipping being
// resolved, classifies the defaulting party, records the canonical commerce
// violation + restriction, and returns the auction to DRAFT.
//
// SETTLEMENT DEADLINE LOGIC:
//  1. Find auctions in waiting_settlement with end_at + 24h <= NOW().
//  2. Per auction (own transaction, FOR UPDATE):
//     a. Skip if no longer waiting_settlement (already settled/claimed/cancelled).
//     b. Skip if shipping_resolved_at IS NOT NULL (shipping phase resolved;
//     payment phase is handled by the payment-expiry machinery).
//     c. If seller_action_required = true AND seller_quote_provided = false:
//     seller violation (seller_shipping_default) -> auction DRAFT.
//     d. Else: buyer shipping violation (buyer_shipping_timeout) -> auction DRAFT.
//  3. Violation insert + restriction upsert + DRAFT transition + outbox event
//     are committed atomically in the auction's own transaction.
//
// WORKER PATTERN:
//   - Phase 1: Short transaction to fetch IDs with SKIP LOCKED
//   - Phase 2: Process each auction in its own transaction
type AuctionSettlementWorker struct {
	db            *db.DB
	auctionSvc    *auctionApp.AuctionService
	auctionRepo   *auctionRepo.AuctionRepository
	violationRepo commercegov.Repository
	outboxRepo    *outboxrepo.OutboxRepository
	log           *zap.Logger
	pollInterval  time.Duration
	batchSize     int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// AuctionSettlementWorkerConfig holds worker configuration.
type AuctionSettlementWorkerConfig struct {
	PollInterval time.Duration // How often to check for expired settlements
	BatchSize    int           // Max auctions to process per poll
}

// DefaultAuctionSettlementWorkerConfig returns default configuration.
func DefaultAuctionSettlementWorkerConfig() AuctionSettlementWorkerConfig {
	return AuctionSettlementWorkerConfig{
		PollInterval: DefaultAuctionSettlementPollInterval,
		BatchSize:    DefaultAuctionSettlementBatchSize,
	}
}

// NewAuctionSettlementWorker creates a new auction settlement worker.
func NewAuctionSettlementWorker(
	db *db.DB,
	auctionService *auctionApp.AuctionService,
	violationRepo commercegov.Repository,
	outboxRepo *outboxrepo.OutboxRepository,
	log *zap.Logger,
	cfg AuctionSettlementWorkerConfig,
) *AuctionSettlementWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultAuctionSettlementPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultAuctionSettlementBatchSize
	}

	return &AuctionSettlementWorker{
		db:            db,
		auctionSvc:    auctionService,
		auctionRepo:   auctionRepo.NewAuctionRepository(),
		violationRepo: violationRepo,
		outboxRepo:    outboxRepo,
		log:           log,
		pollInterval:  cfg.PollInterval,
		batchSize:     cfg.BatchSize,
		stopCh:        make(chan struct{}),
	}
}

// Start begins detecting expired settlements in the background.
func (w *AuctionSettlementWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Auction settlement worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Auction settlement worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker.
func (w *AuctionSettlementWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping auction settlement worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Auction settlement worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Auction settlement worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running.
func (w *AuctionSettlementWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *AuctionSettlementWorker) run() {
	defer w.wg.Done()

	w.checkExpiredSettlements()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.checkExpiredSettlements()

		case <-w.stopCh:
			return
		}
	}
}

// checkExpiredSettlements finds and processes expired auction settlements.
//
// PATTERN:
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED
// Phase 2: Process each auction in its own transaction
func (w *AuctionSettlementWorker) checkExpiredSettlements() {
	ctx := context.Background()

	// Phase 1: Short tx just to fetch IDs with SKIP LOCKED.
	auctionIDs, err := w.findExpiredSettlementAuctionIDs(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find expired settlement auction IDs", zap.Error(err))
		return
	}

	if len(auctionIDs) == 0 {
		return
	}

	w.log.Info("Found expired settlements", zap.Int("count", len(auctionIDs)))

	// Phase 2: Process each auction in its own transaction.
	for _, auctionID := range auctionIDs {
		if err := w.processExpiredSettlement(ctx, auctionID); err != nil {
			w.log.Error("Failed to process expired settlement",
				zap.String("auction_id", auctionID.String()),
				zap.Error(err),
			)
		}
	}
}

// findExpiredSettlementAuctionIDs returns IDs of auctions in
// waiting_settlement whose canonical shipping deadline (end_at + 24h) has
// passed and whose shipping has NOT yet been resolved.
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED.
func (w *AuctionSettlementWorker) findExpiredSettlementAuctionIDs(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var auctionIDs []uuid.UUID

	// Short transaction to fetch and lock IDs.
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM auctions
			WHERE status = 'waiting_settlement'
			  AND shipping_resolved_at IS NULL
			  AND end_at + INTERVAL '24 hours' <= NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		`

		rows, err := tx.Query(ctx, query, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			auctionIDs = append(auctionIDs, id)
		}

		return rows.Err()
	})

	return auctionIDs, err
}

// processExpiredSettlement processes a single expired settlement within its
// own transaction. Phase 2: each auction is processed in a separate
// transaction. Deterministic and idempotent: only one transaction can
// transition the auction because the row is locked FOR UPDATE and the status
// is re-verified after the lock is acquired.
func (w *AuctionSettlementWorker) processExpiredSettlement(
	ctx context.Context,
	auctionID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Load the auction FOR UPDATE (serializes against a concurrent claim /
		// shipping resolution / duplicate worker).
		auction, err := w.auctionRepo.GetForUpdate(ctx, tx, auctionID)
		if err != nil {
			return err
		}

		// Double-check status (may have been claimed by now).
		if auction.Status != entity.StatusWaitingSettlement {
			w.log.Info("Auction no longer in waiting_settlement, skipping",
				zap.String("auction_id", auctionID.String()),
				zap.String("current_status", string(auction.Status)),
			)
			return nil
		}

		// Shipping phase already resolved — the payment phase owns the auction
		// now (payment expiry returns it to DRAFT). Do NOT classify a shipping
		// failure.
		if auction.ShippingResolvedAt != nil {
			w.log.Info("Auction shipping already resolved, skipping deadline enforcement",
				zap.String("auction_id", auctionID.String()),
				zap.String("shipping_resolved_at", auction.ShippingResolvedAt.Format(time.RFC3339)),
			)
			return nil
		}

		// Verify the canonical deadline has actually passed (belt-and-
		// suspenders; the phase-1 query already filtered on it).
		now := time.Now()
		if now.Before(auction.SettlementDeadline()) {
			w.log.Info("Auction settlement deadline not yet reached, skipping",
				zap.String("auction_id", auctionID.String()),
			)
			return nil
		}

		// Classify the defaulting party from the canonical seller/buyer flags.
		var violatedUserID uuid.UUID
		var violationType commercegov.ViolationType
		var reason string
		switch {
		case auction.SellerActionRequired && !auction.SellerQuoteProvided:
			// Case A: seller was required to provide a private quote and did
			// not within end_at + 24h.
			violatedUserID = auction.SellerID
			violationType = commercegov.ViolationSellerShippingDefault
			reason = "seller failed to provide required private shipping quote before settlement deadline"
		default:
			// Case B: buyer failed to resolve shipping within end_at + 24h
			// (or Case A where the seller has provided a quote and the buyer
			// failed to accept/resolve).
			if auction.WinnerID() == nil {
				w.log.Warn("Auction in waiting_settlement without winner; treating as buyer timeout but no winner to restrict",
					zap.String("auction_id", auctionID.String()),
				)
				violatedUserID = auction.SellerID
				violationType = commercegov.ViolationSellerShippingDefault
				reason = "auction in waiting_settlement with no winner; returned to draft"
				break
			}
			violatedUserID = *auction.WinnerID()
			violationType = commercegov.ViolationBuyerShippingTimeout
			reason = "buyer failed to resolve shipping before settlement deadline"
		}

		// Record the canonical violation + apply/extend the restriction
		// (atomic with the auction transition below).
		violation, restriction, err := commercegov.RecordViolationAndRestrict(
			ctx, tx, w.violationRepo, commercegov.RecordInput{
				UserID:        violatedUserID,
				ViolationType: violationType,
				SourceType:    commercegov.SourceTypeAuction,
				SourceID:      auction.ID,
				Reason:        reason,
				Metadata: map[string]any{
					"deadline":               auction.SettlementDeadline().Format(time.RFC3339),
					"end_at":                 auction.EndAt.Format(time.RFC3339),
					"seller_action_required": auction.SellerActionRequired,
					"seller_quote_provided":  auction.SellerQuoteProvided,
				},
			},
		)
		if err != nil {
			return err
		}

		// Capture pre-transition identity for the event payload (the DRAFT
		// transition clears CurrentWinnerID).
		var priorWinnerID uuid.UUID
		if auction.WinnerID() != nil {
			priorWinnerID = *auction.WinnerID()
		}

		// Transition the auction to DRAFT (clears order binding, shipping
		// resolution, seller flags, current bid + winner) and persist.
		if err := w.auctionSvc.ReturnToDraftOnSettlementFailure(ctx, tx, auction); err != nil {
			return err
		}

		// Emit the canonical settlement-failed event for downstream
		// notification consumers.
		if err := w.emitSettlementFailedEvent(ctx, tx, auction, violation, restriction, priorWinnerID); err != nil {
			return err
		}

		w.log.Warn("Auction settlement deadline reached - auction returned to draft",
			zap.String("auction_id", auctionID.String()),
			zap.String("violated_user_id", violatedUserID.String()),
			zap.String("violation_type", string(violationType)),
			zap.String("restricted_until", restriction.RestrictedUntil.Format(time.RFC3339)),
		)

		return nil
	})
}

// emitSettlementFailedEvent emits the auction.settlement_failed outbox event.
// payload shape:
//
//	{ "auction_id", "violated_user_id", "violation_type", "seller_id",
//	  "winner_id", "violation_id", "restricted_until", "timestamp" }
func (w *AuctionSettlementWorker) emitSettlementFailedEvent(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
	violation *commercegov.Violation,
	restriction *commercegov.Restriction,
	winnerID uuid.UUID,
) error {
	payload := map[string]interface{}{
		"auction_id":       auction.ID.String(),
		"violated_user_id": violation.UserID.String(),
		"violation_type":   string(violation.ViolationType),
		"seller_id":        auction.SellerID.String(),
		"winner_id":        winnerID.String(),
		"violation_id":     violation.ID.String(),
		"restricted_until": restriction.RestrictedUntil.Format(time.RFC3339),
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}
	payloadBytes, _ := json.Marshal(payload)

	return w.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.settlement_failed",
		auction.ID,
		payloadBytes,
	)
}
