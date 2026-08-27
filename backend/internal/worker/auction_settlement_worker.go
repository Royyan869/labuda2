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
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultAuctionSettlementPollInterval is how often the worker checks for expired settlements
	DefaultAuctionSettlementPollInterval = 5 * time.Minute

	// DefaultAuctionSettlementBatchSize is max auctions to process per poll
	DefaultAuctionSettlementBatchSize = 50
)

// AuctionSettlementWorker detects auctions whose settlement deadline has expired.
//
// BNR SETTLEMENT TIMEOUT LOGIC:
// 1. Find auctions in waiting_settlement with settlement_deadline <= NOW()
// 2. Verify they haven't been processed before (idempotency via status check)
// 3. Transition to expired_bnr status
// 4. Emit auction_bnr_detected event for trust scoring
//
// WORKER PATTERN:
// - Phase 1: Short transaction to fetch IDs with SKIP LOCKED
// - Phase 2: Process each auction in its own transaction
type AuctionSettlementWorker struct {
	db           *db.DB
	auctionSvc   *auctionApp.AuctionService
	auctionRepo  *auctionRepo.AuctionRepository
	outboxRepo   *outboxrepo.OutboxRepository
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

// AuctionSettlementWorkerConfig holds worker configuration
type AuctionSettlementWorkerConfig struct {
	PollInterval time.Duration // How often to check for expired settlements
	BatchSize    int           // Max auctions to process per poll
}

// DefaultAuctionSettlementWorkerConfig returns default configuration
func DefaultAuctionSettlementWorkerConfig() AuctionSettlementWorkerConfig {
	return AuctionSettlementWorkerConfig{
		PollInterval: DefaultAuctionSettlementPollInterval,
		BatchSize:    DefaultAuctionSettlementBatchSize,
	}
}

// NewAuctionSettlementWorker creates a new auction settlement worker
func NewAuctionSettlementWorker(
	db *db.DB,
	auctionService *auctionApp.AuctionService,
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
		db:          db,
		auctionSvc:  auctionService,
		auctionRepo: auctionRepo.NewAuctionRepository(),
		outboxRepo:  outboxRepo,
		log:         log,
		pollInterval: cfg.PollInterval,
		batchSize:   cfg.BatchSize,
		stopCh:      make(chan struct{}),
	}
}

// Start begins detecting expired settlements in the background
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

// Stop gracefully shuts down the worker
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

// IsRunning returns true if the worker is currently running
func (w *AuctionSettlementWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
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

	// Phase 1: Short tx just to fetch IDs with SKIP LOCKED
	auctionIDs, err := w.findExpiredSettlementAuctionIDs(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find expired settlement auction IDs", zap.Error(err))
		return
	}

	if len(auctionIDs) == 0 {
		return
	}

	w.log.Info("Found expired settlements", zap.Int("count", len(auctionIDs)))

	// Phase 2: Process each auction in its own transaction
	for _, auctionID := range auctionIDs {
		if err := w.processExpiredSettlement(ctx, auctionID); err != nil {
			w.log.Error("Failed to process expired settlement",
				zap.String("auction_id", auctionID.String()),
				zap.Error(err),
			)
		}
	}
}

// findExpiredSettlementAuctionIDs returns IDs of auctions in waiting_settlement
// whose settlement deadline has passed.
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED.
func (w *AuctionSettlementWorker) findExpiredSettlementAuctionIDs(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var auctionIDs []uuid.UUID

	// Short transaction to fetch and lock IDs
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM auctions
			WHERE status = 'waiting_settlement'
			  AND settlement_deadline <= NOW()
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

// processExpiredSettlement processes a single expired settlement within its own transaction.
// Phase 2: Each auction is processed in a separate transaction.
func (w *AuctionSettlementWorker) processExpiredSettlement(
	ctx context.Context,
	auctionID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Get auction for update
		auction, err := w.auctionSvc.GetAuction(ctx, tx, auctionID)
		if err != nil {
			return err
		}

		// Double-check status (may have been claimed by now)
		if auction.Status != entity.StatusWaitingSettlement {
			w.log.Info("Auction no longer in waiting_settlement, skipping",
				zap.String("auction_id", auctionID.String()),
				zap.String("current_status", string(auction.Status)),
			)
			return nil
		}

		// Verify deadline has passed
		now := time.Now()
		if auction.SettlementDeadline == nil || now.Before(*auction.SettlementDeadline) {
			w.log.Info("Auction settlement deadline not yet reached, skipping",
				zap.String("auction_id", auctionID.String()),
			)
			return nil
		}

		// Get auction details for event payload before transitioning
		sellerID := auction.SellerID
		var winnerID uuid.UUID
		if auction.CurrentWinnerID != nil {
			winnerID = *auction.CurrentWinnerID
		}

		// Transition to expired_bnr
		// Use EndAuctionInternal with ExpireSettlement flag
		if err := w.expireAuctionSettlement(ctx, tx, auction); err != nil {
			return err
		}

		// Emit trust event
		if err := w.emitBNREvent(ctx, tx, auctionID, winnerID, sellerID); err != nil {
			return err
		}

		w.log.Warn("BNR detected - auction settlement expired",
			zap.String("auction_id", auctionID.String()),
			zap.String("winner_id", winnerID.String()),
			zap.String("seller_id", sellerID.String()),
		)

		return nil
	})
}

// expireAuctionSettlement transitions the auction to expired_bnr status
func (w *AuctionSettlementWorker) expireAuctionSettlement(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) error {
	// Transition to expired_bnr
	if err := auction.TransitionToExpiredBNR(); err != nil {
		return err
	}

	// Persist the status change
	if err := w.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	return nil
}

// emitBNREvent emits the auction_bnr_detected event for fraud score impact
func (w *AuctionSettlementWorker) emitBNREvent(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
	winnerID uuid.UUID,
	sellerID uuid.UUID,
) error {
	payload := map[string]interface{}{
		"auction_id": auctionID.String(),
		"winner_id":  winnerID.String(),
		"seller_id":  sellerID.String(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	payloadBytes, _ := json.Marshal(payload)

	// Emit outbox event
	if err := w.outboxRepo.InsertEvent(
		ctx, tx,
		"auction_bnr_detected",
		auctionID,
		payloadBytes,
	); err != nil {
		return err
	}

	return nil
}

// AuctionBNRDetectedEvent represents the payload of auction_bnr_detected events.
type AuctionBNRDetectedEvent struct {
	AuctionID uuid.UUID `json:"auction_id"`
	WinnerID  uuid.UUID `json:"winner_id"`
	SellerID  uuid.UUID `json:"seller_id"`
	Timestamp string    `json:"timestamp"`
}


