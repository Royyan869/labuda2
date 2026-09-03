package worker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultAuctionPollInterval is how often the worker checks for ending auctions
	DefaultAuctionPollInterval = 30 * time.Second

	// DefaultAuctionBatchSize is max auctions to end per poll
	DefaultAuctionBatchSize = 50
)

// AuctionEndWorker detects and ends expired active auctions.
// It processes each auction atomically within a transaction using SKIP LOCKED
// for concurrent-safe batch processing.
type AuctionEndWorker struct {
	db             Transactor
	auctionService *auctionApp.AuctionService
	log            *zap.Logger
	pollInterval   time.Duration
	batchSize      int

	metrics WorkerLivenessRecorder // optional sink; nil = no-op

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// AuctionEndWorkerConfig holds worker configuration
type AuctionEndWorkerConfig struct {
	PollInterval time.Duration // How often to check for ending auctions
	BatchSize    int           // Max auctions to process per poll
}

// DefaultAuctionEndWorkerConfig returns default configuration
func DefaultAuctionEndWorkerConfig() AuctionEndWorkerConfig {
	return AuctionEndWorkerConfig{
		PollInterval: DefaultAuctionPollInterval,
		BatchSize:    DefaultAuctionBatchSize,
	}
}

// NewAuctionEndWorker creates a new auction end worker
func NewAuctionEndWorker(
	db Transactor,
	auctionService *auctionApp.AuctionService,
	log *zap.Logger,
	cfg AuctionEndWorkerConfig,
) *AuctionEndWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultAuctionPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultAuctionBatchSize
	}

	return &AuctionEndWorker{
		db:             db,
		auctionService: auctionService,
		log:            log,
		pollInterval:   cfg.PollInterval,
		batchSize:      cfg.BatchSize,
		stopCh:         make(chan struct{}),
	}
}

// SetMetricsRecorder attaches an optional liveness sink. Must be called before
// Start(). The recorder is sink-only and never influences auction decisions.
func (w *AuctionEndWorker) SetMetricsRecorder(r WorkerLivenessRecorder) {
	w.metrics = r
}

// Start begins detecting ending auctions in the background
func (w *AuctionEndWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Auction end worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{}) // Always create a new stopCh

	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameAuctionEnd, true)
	}

	w.wg.Add(1)
	go w.run()

	w.log.Info("Auction end worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *AuctionEndWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping auction end worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Auction end worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Auction end worker shutdown timeout")
	}

	w.running = false
	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameAuctionEnd, false)
	}
}

// IsRunning returns true if the worker is currently running
func (w *AuctionEndWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *AuctionEndWorker) run() {
	defer w.wg.Done()

	w.checkEndingAuctions()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.checkEndingAuctions()

		case <-w.stopCh:
			return
		}
	}
}

// checkEndingAuctions finds and ends expired active auctions.
//
// PATTERN:
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED
// Phase 2: Process each auction in its own transaction
func (w *AuctionEndWorker) checkEndingAuctions() {
	ctx := context.Background()

	// Phase 1: Short tx just to fetch IDs with SKIP LOCKED
	// Locks are released immediately after this transaction commits
	auctionIDs, err := w.findEndingAuctionIDs(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find ending auctions", zap.Error(err))
		return
	}

	if len(auctionIDs) == 0 {
		return
	}

	w.log.Info("Found ending auctions", zap.Int("count", len(auctionIDs)))

	// Phase 2: Process each auction in its own transaction
	// Failure is isolated per entity - no cascade rollback
	for _, auctionID := range auctionIDs {
		if err := w.endAuction(ctx, auctionID); err != nil {
			w.log.Error("Failed to end auction",
				zap.String("auction_id", auctionID.String()),
				zap.Error(err),
			)
		}
	}

	// Heartbeat after a successful scan cycle (phase 1 fetch succeeded).
	// Intentionally placed here (not defer) so a phase 1 DB error that
	// returns early does NOT advance the heartbeat.
	if w.metrics != nil {
		w.metrics.RecordWorkerHeartbeat(WorkerNameAuctionEnd)
	}
}

// findEndingAuctionIDs returns IDs of active auctions that have ended.
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED.
func (w *AuctionEndWorker) findEndingAuctionIDs(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var auctionIDs []uuid.UUID

	// Short transaction to fetch and lock IDs
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM auctions
			WHERE status = 'active'
			  AND end_at <= NOW()
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

// endAuction ends a single auction within its own transaction.
// Phase 2: Each auction is processed in a separate transaction.
//
// For auctions with no bids: simply ends the auction.
// For auctions with bids: ends the auction, order creation is deferred to claim flow.
//
// The claim flow (winner claims auction) will handle:
// 1. Collecting shipping details from winner
// 2. Creating the order
//
// This is a two-phase approach to avoid needing shipping details at end time.
func (w *AuctionEndWorker) endAuction(
	ctx context.Context,
	auctionID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Call EndAuctionInternal with empty shipping details
		// The service will:
		// 1. End the auction
		// 2. If there's a winner, defer order creation to claim flow
		//
		// For this MVP, we're not implementing the full claim flow
		// Instead, the winner will need to manually initiate checkout

		// Use placeholder values for the service call
		// In production, this would be replaced by proper claim flow
		return w.auctionService.EndAuctionInternal(ctx, tx, auctionApp.EndAuctionInput{
			AuctionID:        auctionID,
			ShippingSetupID: uuid.Nil, // Will be provided in claim flow
			ProvinceCode:     "",       // Will be provided in claim flow
			CityCode:         "",       // Will be provided in claim flow
		})
	})
}


