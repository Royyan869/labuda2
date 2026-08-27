package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultSellerMetricsInterval is how often the metrics worker runs.
	// Daily is sufficient: snapshots are idempotent per (seller_id, year, month).
	DefaultSellerMetricsInterval = 24 * time.Hour
)

// SellerMonthlyMetricsWorker generates monthly performance snapshots for sellers.
//
// Lifecycle: Start() / Stop() / IsRunning() satisfy serverboot.Worker.
// Schedule: runs daily, generates snapshot for the previous month.
//
// This worker:
// - Aggregates order data (total_items_sold, fulfilled_count)
// - Aggregates cancelled_timeout orders (cancelled_timeout_count)
// - Aggregates rating data (average_rating)
// - Inserts immutable snapshots to seller_monthly_metrics
// - Idempotent per (seller_id, year, month) via UNIQUE constraint + existence check
//
// Write surface: seller_monthly_metrics table ONLY.
//
// Constraints:
// - Does NOT modify tier
// - Does NOT modify seller_profile
// - Does NOT modify subscription
// - Does NOT touch financial ledger
// - 1 seller = 1 transaction
//
// RATING DOMAIN BOUNDARY: worker domain CANNOT access order_ratings table directly
// All rating operations MUST go through RatingReader interface (read-only access)
type SellerMonthlyMetricsWorker struct {
	db            interface{ WithTx(ctx context.Context, fn func(tx db.Tx) error) error }
	sellerRepo    sellerRepo.SellerRepository
	orderRepo     *orderRepoImpl.OrderRepository
	ratingReader  ratingApp.RatingReader // Interface-based access (read-only)
	log           *zap.Logger
	pollInterval  time.Duration

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewSellerMonthlyMetricsWorker creates a new monthly metrics worker.
func NewSellerMonthlyMetricsWorker(
	db interface{ WithTx(ctx context.Context, fn func(tx db.Tx) error) error },
	sellerRepo sellerRepo.SellerRepository,
	orderRepo *orderRepoImpl.OrderRepository,
	log *zap.Logger,
) *SellerMonthlyMetricsWorker {
	if log == nil {
		log = zap.NewNop()
	}

	// RATING DOMAIN BOUNDARY: Use factory to get rating reader interface
	ratingFactory := ratingApp.NewRatingDomainFactory()

	return &SellerMonthlyMetricsWorker{
		db:           db,
		sellerRepo:   sellerRepo,
		orderRepo:    orderRepo,
		ratingReader: ratingFactory.GetReader(), // Interface-based: read-only access for metrics aggregation
		log:          log,
		pollInterval: DefaultSellerMetricsInterval,
	}
}

// Start begins the metrics worker loop in the background.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *SellerMonthlyMetricsWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("SellerMonthlyMetricsWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.log.Info("SellerMonthlyMetricsWorker started",
		zap.Duration("poll_interval", w.pollInterval),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
func (w *SellerMonthlyMetricsWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.log.Info("SellerMonthlyMetricsWorker stopped")
}

// IsRunning returns true if the worker loop is active.
func (w *SellerMonthlyMetricsWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main loop. Generates previous-month snapshot on each tick.
func (w *SellerMonthlyMetricsWorker) run() {
	defer w.wg.Done()

	// Run once immediately on startup.
	w.runSnapshot(w.shutdownCtx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.runSnapshot(w.shutdownCtx)
		}
	}
}

// runSnapshot generates the snapshot for the previous month.
// Previous month = last calendar month (e.g., if today is June 15, generates May).
func (w *SellerMonthlyMetricsWorker) runSnapshot(ctx context.Context) {
	now := time.Now().UTC()
	prevMonth := now.AddDate(0, -1, 0)
	year := prevMonth.Year()
	month := int(prevMonth.Month())

	if err := w.GenerateMonthlySnapshot(ctx, year, month); err != nil {
		w.log.Error("Monthly metrics snapshot failed",
			zap.Int("year", year),
			zap.Int("month", month),
			zap.Error(err),
		)
	}
}

// GenerateMonthlySnapshot creates monthly performance snapshots for all sellers.
//
// For each seller:
// 1. Check if snapshot exists for (seller_id, year, month)
// 2. If not exists:
//    - Calculate total_items_sold from completed orders
//    - Calculate average_rating from order_ratings (excluding invalidated)
//    - Insert new snapshot
//
// Each seller is processed in its own transaction.
// Idempotent: re-running for the same month will skip existing snapshots.
func (w *SellerMonthlyMetricsWorker) GenerateMonthlySnapshot(
	ctx context.Context,
	year int,
	month int,
) error {
	w.log.Info("Starting monthly metrics snapshot generation",
		zap.Int("year", year),
		zap.Int("month", month),
	)

	// Validate month range
	if month < 1 || month > 12 {
		return fmt.Errorf("invalid month: %d (must be 1-12)", month)
	}

	// Fetch all seller IDs
	sellerIDs, err := w.fetchAllSellerIDs(ctx)
	if err != nil {
		return fmt.Errorf("fetch seller IDs failed: %w", err)
	}

	if len(sellerIDs) == 0 {
		w.log.Info("No sellers found for snapshot generation")
		return nil
	}

	w.log.Info("Processing sellers for monthly snapshot",
		zap.Int("total_sellers", len(sellerIDs)),
	)

	// Process each seller in its own transaction
	var processed, skipped int
	for _, sellerID := range sellerIDs {
		snapshotExists, err := w.snapshotExists(ctx, sellerID, year, month)
		if err != nil {
			w.log.Error("Failed to check snapshot existence",
				zap.String("seller_id", sellerID.String()),
				zap.Error(err),
			)
			continue
		}

		if snapshotExists {
			skipped++
			w.log.Debug("Snapshot already exists, skipping",
				zap.String("seller_id", sellerID.String()),
				zap.Int("year", year),
				zap.Int("month", month),
			)
			continue
		}

		if err := w.generateSellerSnapshot(ctx, sellerID, year, month); err != nil {
			w.log.Error("Failed to generate snapshot for seller",
				zap.String("seller_id", sellerID.String()),
				zap.Error(err),
			)
			continue
		}

		processed++
	}

	w.log.Info("Monthly snapshot generation completed",
		zap.Int("year", year),
		zap.Int("month", month),
		zap.Int("processed", processed),
		zap.Int("skipped", skipped),
	)

	return nil
}

// fetchAllSellerIDs retrieves all seller profile IDs from the database.
func (w *SellerMonthlyMetricsWorker) fetchAllSellerIDs(ctx context.Context) ([]uuid.UUID, error) {
	var sellerIDs []uuid.UUID

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM seller_profiles
			ORDER BY id
		`

		rows, err := tx.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			sellerIDs = append(sellerIDs, id)
		}

		return rows.Err()
	})

	return sellerIDs, err
}

// snapshotExists checks if a snapshot already exists for the given seller and period.
func (w *SellerMonthlyMetricsWorker) snapshotExists(
	ctx context.Context,
	sellerID uuid.UUID,
	year int,
	month int,
) (bool, error) {
	var exists bool

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT EXISTS(
				SELECT 1
				FROM seller_monthly_metrics
				WHERE seller_id = $1 AND year = $2 AND month = $3
				LIMIT 1
			)
		`
		return tx.QueryRow(ctx, query, sellerID, year, month).Scan(&exists)
	})

	return exists, err
}

// generateSellerSnapshot generates a monthly snapshot for a single seller.
// Runs within its own transaction - 1 seller = 1 tx.
func (w *SellerMonthlyMetricsWorker) generateSellerSnapshot(
	ctx context.Context,
	sellerID uuid.UUID,
	year int,
	month int,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Calculate month boundaries using half-open interval [start, end)
		monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		monthEnd := monthStart.AddDate(0, 1, 0)

		// Aggregate total items sold from completed orders
		totalItemsSold, err := w.aggregateTotalItemsSold(ctx, tx, sellerID, monthStart, monthEnd)
		if err != nil {
			return fmt.Errorf("aggregate total items sold failed: %w", err)
		}

		// Aggregate average rating from order_ratings (excluding invalidated)
		averageRating, err := w.aggregateAverageRating(ctx, tx, sellerID, monthStart, monthEnd)
		if err != nil {
			return fmt.Errorf("aggregate average rating failed: %w", err)
		}

		// Aggregate fulfilled order count (completed orders)
		fulfilledCount, err := w.aggregateFulfilledCount(ctx, tx, sellerID, monthStart, monthEnd)
		if err != nil {
			return fmt.Errorf("aggregate fulfilled count failed: %w", err)
		}

		// Aggregate cancelled_timeout order count
		cancelledTimeoutCount, err := w.aggregateCancelledTimeoutCount(ctx, tx, sellerID, monthStart, monthEnd)
		if err != nil {
			return fmt.Errorf("aggregate cancelled timeout count failed: %w", err)
		}

		// Create the monthly metric snapshot
		metric := &sellerEntity.SellerMonthlyMetric{
			ID:                    uuid.New(),
			SellerID:              sellerID,
			Year:                  year,
			Month:                 month,
			TotalItemsSold:        totalItemsSold,
			AverageRating:         averageRating,
			FulfilledCount:        fulfilledCount,
			CancelledTimeoutCount: cancelledTimeoutCount,
			CreatedAt:             time.Now().UTC(),
		}

		// Insert the snapshot
		if err := w.sellerRepo.InsertMonthlyMetricTx(ctx, tx, metric); err != nil {
			return fmt.Errorf("insert monthly metric failed: %w", err)
		}

		w.log.Debug("Created monthly snapshot for seller",
			zap.String("seller_id", sellerID.String()),
			zap.Int("year", year),
			zap.Int("month", month),
			zap.Int("total_items_sold", totalItemsSold),
			zap.Float64("average_rating", averageRating),
			zap.Int("fulfilled_count", fulfilledCount),
			zap.Int("cancelled_timeout_count", cancelledTimeoutCount),
		)

		return nil
	})
}

// aggregateTotalItemsSold sums the quantity of completed orders for a seller in the given period.
func (w *SellerMonthlyMetricsWorker) aggregateTotalItemsSold(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	monthStart time.Time,
	monthEnd time.Time,
) (int, error) {
	var totalItemsSold int

	query := `
		SELECT COALESCE(SUM(quantity), 0)
		FROM orders
		WHERE seller_id = $1
		  AND status = 'completed'
		  AND completed_at >= $2
		  AND completed_at < $3
	`

	err := tx.QueryRow(ctx, query, sellerID, monthStart, monthEnd).Scan(&totalItemsSold)
	if err != nil {
		return 0, fmt.Errorf("query total items sold failed: %w", err)
	}

	return totalItemsSold, nil
}

// aggregateFulfilledCount counts distinct completed orders for a seller in the given period.
// Uses completed_at for time-bucketing (set when order transitions to completed).
func (w *SellerMonthlyMetricsWorker) aggregateFulfilledCount(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	monthStart time.Time,
	monthEnd time.Time,
) (int, error) {
	var count int

	query := `
		SELECT COUNT(*)
		FROM orders
		WHERE seller_id = $1
		  AND status = 'completed'
		  AND completed_at >= $2
		  AND completed_at < $3
	`

	err := tx.QueryRow(ctx, query, sellerID, monthStart, monthEnd).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query fulfilled count failed: %w", err)
	}

	return count, nil
}

// aggregateCancelledTimeoutCount counts orders auto-cancelled due to shipment timeout
// for a seller in the given period.
// Uses updated_at for time-bucketing because cancelled_timeout orders do NOT set completed_at.
// Safe: cancelled_timeout is a terminal state so updated_at is frozen after transition.
func (w *SellerMonthlyMetricsWorker) aggregateCancelledTimeoutCount(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	monthStart time.Time,
	monthEnd time.Time,
) (int, error) {
	var count int

	query := `
		SELECT COUNT(*)
		FROM orders
		WHERE seller_id = $1
		  AND status = 'cancelled_timeout'
		  AND updated_at >= $2
		  AND updated_at < $3
	`

	err := tx.QueryRow(ctx, query, sellerID, monthStart, monthEnd).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query cancelled timeout count failed: %w", err)
	}

	return count, nil
}

// aggregateAverageRating calculates the average rating for a seller in the given period.
// Returns 0.0 if no ratings exist for the period.
//
// RATING INVALIDATION: Only counts valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from aggregation.
func (w *SellerMonthlyMetricsWorker) aggregateAverageRating(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	monthStart time.Time,
	monthEnd time.Time,
) (float64, error) {
	// RATING DOMAIN BOUNDARY: Delegates to RatingReader interface instead of direct SQL access
	return w.ratingReader.GetAverageRatingForPeriod(ctx, tx, sellerID, monthStart, monthEnd)
}

// SellerMetricsAggregation represents the aggregated metrics for a seller in a period.
// Used for testing and future extensions.
type SellerMetricsAggregation struct {
	SellerID              uuid.UUID
	Year                  int
	Month                 int
	TotalItemsSold        int
	AverageRating         float64
	FulfilledCount        int
	CancelledTimeoutCount int
}

// GetSellerMetricsForPeriod retrieves the snapshot for a specific seller and period.
// Returns nil if no snapshot exists.
func (w *SellerMonthlyMetricsWorker) GetSellerMetricsForPeriod(
	ctx context.Context,
	sellerID uuid.UUID,
	year int,
	month int,
) (*SellerMetricsAggregation, error) {
	var result SellerMetricsAggregation

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT seller_id, year, month,
			       total_items_sold, average_rating,
			       fulfilled_count, cancelled_timeout_count
			FROM seller_monthly_metrics
			WHERE seller_id = $1 AND year = $2 AND month = $3
		`

		err := tx.QueryRow(ctx, query, sellerID, year, month).Scan(
			&result.SellerID,
			&result.Year,
			&result.Month,
			&result.TotalItemsSold,
			&result.AverageRating,
			&result.FulfilledCount,
			&result.CancelledTimeoutCount,
		)

		return err
	})

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get seller metrics for period failed: %w", err)
	}

	return &result, nil
}


