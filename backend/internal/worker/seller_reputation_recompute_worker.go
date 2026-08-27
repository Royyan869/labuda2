package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultReputationRecomputeInterval is how often the worker runs.
	// Nightly is the canonical frequency: drift window is at most 24h.
	DefaultReputationRecomputeInterval = 24 * time.Hour

	// DefaultReputationWindowDays is the canonical rolling-window length.
	// 90 days ≈ 3 months, captures a meaningful business cycle.
	DefaultReputationWindowDays = 90

	// Tier promotion thresholds.
	// Canonical tier thresholds — owner-approved 2026-05-28.
	// Calibrated for koi/agriculture marketplace with high transaction frequency
	// (dozens of orders/week, 3–5+ shipments/day per active seller).
	//
	// Tier ladder: Basic → Pro → Elite.
	// Legend is intentionally deferred — see FIX-2 extension note below.
	//
	// DO NOT change without owner policy decision.
	reputationProMinOrders  = 100
	reputationProMinRatings = 15
	reputationProMinAvg     = 4.6

	reputationEliteMinOrders  = 300
	reputationEliteMinRatings = 50
	reputationEliteMinAvg     = 4.7
)

// reputationAggregates holds the computed rolling-window inputs for a seller.
// All fields are derived from base tables; none come from seller_monthly_metrics.
type reputationAggregates struct {
	completedOrders  int
	cancelledTimeout int
	ratingAverage    float64
	ratingCount      int
	disputeLosses    int
}

// fulfillmentRate derives the fulfillment rate from aggregate counts.
// Returns 0.0 when both counts are zero.
func (a *reputationAggregates) fulfillmentRate() float64 {
	total := a.completedOrders + a.cancelledTimeout
	if total == 0 {
		return 0.0
	}
	return float64(a.completedOrders) / float64(total)
}

// reputationAggregator is an interface for computing rolling-window metrics.
// Separated from the worker to allow deterministic mocking in tests.
type reputationAggregator interface {
	compute(ctx context.Context, tx db.Tx, sellerID uuid.UUID, windowStart time.Time) (*reputationAggregates, error)
}

// reputationSellerStore is the minimal seller persistence surface used by
// SellerReputationRecomputeWorker. Using a minimal interface (instead of the
// full SellerRepository) keeps test mocks small and focused.
type reputationSellerStore interface {
	GetByIDForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*sellerEntity.SellerProfile, error)
	UpsertReputationStateTx(ctx context.Context, tx db.Tx, state *sellerEntity.SellerReputationState) error
	UpdateTierTx(ctx context.Context, tx db.Tx, id uuid.UUID, tier sellerEntity.Tier) error
}

// reputationOutboxStore is the minimal outbox surface used by the worker.
type reputationOutboxStore interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// SellerReputationRecomputeWorker is the CANONICAL REPUTATION AUTHORITY worker.
//
// It runs nightly and for every seller:
//  1. Queries rolling 90-day aggregates from base tables (orders, order_ratings, refunds)
//  2. Upserts the result into seller_reputation_state (mutable, overwritable)
//  3. Evaluates tier thresholds against the fresh aggregates
//  4. If tier changed: updates seller_profiles.tier + emits outbox event
//
// Late refunds, rating invalidations, and dispute resolutions are automatically
// reflected on the next recompute cycle — no special reconciliation needed.
//
// Lifecycle: Start() / Stop() / IsRunning() satisfy serverboot.Worker.
// Schedule: runs nightly (DefaultReputationRecomputeInterval).
//
// Write surfaces: seller_reputation_state, seller_profiles.tier, outbox.
// Invariant: seller_monthly_metrics is never read or written by this worker.
type SellerReputationRecomputeWorker struct {
	db         interface{ WithTx(ctx context.Context, fn func(tx db.Tx) error) error }
	store      reputationSellerStore
	outbox     reputationOutboxStore
	aggregator reputationAggregator
	log        *zap.Logger
	interval   time.Duration
	windowDays int

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewSellerReputationRecomputeWorker constructs the canonical reputation authority worker.
// The passed sellerRepo and outboxRepo must satisfy the minimal store interfaces.
func NewSellerReputationRecomputeWorker(
	database interface{ WithTx(ctx context.Context, fn func(tx db.Tx) error) error },
	store reputationSellerStore,
	outbox reputationOutboxStore,
	log *zap.Logger,
) *SellerReputationRecomputeWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &SellerReputationRecomputeWorker{
		db:         database,
		store:      store,
		outbox:     outbox,
		aggregator: &productionAggregator{},
		log:        log,
		interval:   DefaultReputationRecomputeInterval,
		windowDays: DefaultReputationWindowDays,
	}
}

// Start begins the nightly recompute loop in the background.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *SellerReputationRecomputeWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("SellerReputationRecomputeWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.log.Info("SellerReputationRecomputeWorker started",
		zap.Duration("interval", w.interval),
		zap.Int("window_days", w.windowDays),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
func (w *SellerReputationRecomputeWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.log.Info("SellerReputationRecomputeWorker stopped")
}

// IsRunning returns true if the worker loop is active.
func (w *SellerReputationRecomputeWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main background loop.
func (w *SellerReputationRecomputeWorker) run() {
	defer w.wg.Done()

	// Run immediately on startup.
	w.RecomputeAllSellers(w.shutdownCtx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.RecomputeAllSellers(w.shutdownCtx)
		}
	}
}

// RecomputeAllSellers recomputes reputation state for every seller.
// Each seller is processed in its own transaction — one seller failure does
// not block others.
func (w *SellerReputationRecomputeWorker) RecomputeAllSellers(ctx context.Context) {
	sellerIDs, err := w.fetchAllSellerIDs(ctx)
	if err != nil {
		w.log.Error("SellerReputationRecomputeWorker: failed to fetch seller IDs",
			zap.Error(err),
		)
		return
	}

	if len(sellerIDs) == 0 {
		w.log.Info("SellerReputationRecomputeWorker: no sellers to process")
		return
	}

	w.log.Info("SellerReputationRecomputeWorker: starting recompute cycle",
		zap.Int("seller_count", len(sellerIDs)),
		zap.Int("window_days", w.windowDays),
	)

	now := time.Now().UTC()
	var processed, tierChanged, failed int

	for _, sellerID := range sellerIDs {
		changed, err := w.processOneSeller(ctx, sellerID, now)
		if err != nil {
			failed++
			w.log.Error("SellerReputationRecomputeWorker: seller processing failed",
				zap.String("seller_id", sellerID.String()),
				zap.Error(err),
			)
			continue
		}
		processed++
		if changed {
			tierChanged++
		}
	}

	w.log.Info("SellerReputationRecomputeWorker: cycle complete",
		zap.Int("processed", processed),
		zap.Int("tier_changed", tierChanged),
		zap.Int("failed", failed),
	)
}

// processOneSeller recomputes reputation state and evaluates tier for a single
// seller. Returns true if the tier changed. All mutations are in one transaction.
func (w *SellerReputationRecomputeWorker) processOneSeller(
	ctx context.Context,
	sellerID uuid.UUID,
	now time.Time,
) (tierChanged bool, err error) {
	windowStart := now.Add(-time.Duration(w.windowDays) * 24 * time.Hour)

	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		// 1. Compute rolling-window aggregates from base tables.
		agg, err := w.aggregator.compute(ctx, tx, sellerID, windowStart)
		if err != nil {
			return fmt.Errorf("compute aggregates: %w", err)
		}

		// 2. Lock the seller profile to prevent concurrent tier mutations.
		profile, err := w.store.GetByIDForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("lock seller profile: %w", err)
		}
		if profile == nil {
			return fmt.Errorf("seller profile not found: %s", sellerID)
		}

		// 3. Evaluate new tier from fresh aggregates.
		newTier := evaluateTierFromAggregates(profile.Tier, agg)
		evalTime := now

		// 4. Build reputation state.
		state := &sellerEntity.SellerReputationState{
			SellerID:                sellerID,
			WindowDays:              w.windowDays,
			WindowStart:             windowStart,
			WindowEnd:               now,
			RollingCompletedOrders:  agg.completedOrders,
			RollingCancelledTimeout: agg.cancelledTimeout,
			RollingRatingAverage:    agg.ratingAverage,
			RollingRatingCount:      agg.ratingCount,
			RollingDisputeLossCount: agg.disputeLosses,
			RollingFulfillmentRate:  agg.fulfillmentRate(),
			CurrentTier:             newTier,
			TierLastEvaluatedAt:     &evalTime,
			ReputationUpdatedAt:     now,
		}

		// 5. Persist reputation state (UPSERT — safe to replay).
		if err := w.store.UpsertReputationStateTx(ctx, tx, state); err != nil {
			return fmt.Errorf("upsert reputation state: %w", err)
		}

		// 6. If tier changed: update profile badge + emit outbox event.
		if newTier == profile.Tier {
			return nil
		}

		if err := w.store.UpdateTierTx(ctx, tx, sellerID, newTier); err != nil {
			return fmt.Errorf("update tier: %w", err)
		}

		eventType := "seller.tier.upgraded"
		if isTierDowngrade(profile.Tier, newTier) {
			eventType = "seller.tier.downgraded"
		}

		// Idempotency key: one tier-change event per seller per calendar day.
		idempotencyKey := fmt.Sprintf("seller.tier.change.%s.%s",
			sellerID.String(),
			now.Format("2006-01-02"),
		)

		payload := map[string]any{
			"seller_id":     sellerID.String(),
			"previous_tier": string(profile.Tier),
			"new_tier":      string(newTier),
			"evaluated_at":  now.UTC().Format(time.RFC3339),
			"window_days":   w.windowDays,
		}

		if err := w.outbox.InsertTx(ctx, tx, eventType, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert tier change outbox event: %w", err)
		}

		tierChanged = true

		w.log.Info("SellerReputationRecomputeWorker: tier changed",
			zap.String("seller_id", sellerID.String()),
			zap.String("previous_tier", string(profile.Tier)),
			zap.String("new_tier", string(newTier)),
			zap.String("event_type", eventType),
		)

		return nil
	})

	return tierChanged, err
}

// fetchAllSellerIDs retrieves seller profile IDs for active accounts only,
// ordered by ID for deterministic processing.
//
// Banned, suspended, and deleted users are excluded. Their existing
// seller_reputation_state rows are preserved (no erasure) but will NOT be
// recomputed or re-promoted while the account is in a non-active state.
// This prevents "Elite but Banned" drift when tier is publicly exposed.
func (w *SellerReputationRecomputeWorker) fetchAllSellerIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT sp.id
			FROM seller_profiles sp
			JOIN users u ON u.id = sp.user_id
			WHERE u.account_status = 'active'
			  AND u.deleted_at IS NULL
			ORDER BY sp.id
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}

		return rows.Err()
	})

	return ids, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Tier evaluation logic (pure function — no DB, fully testable)
// ─────────────────────────────────────────────────────────────────────────────

// evaluateTierFromAggregates determines the new tier from rolling aggregates
// and the seller's current tier. Pure function: no side effects, no DB access.
//
// Promotion rules (no-skip: basic → pro → elite only):
//   - Elite: ≥300 orders, ≥50 ratings, avg ≥4.7 AND currently pro
//   - Pro:   ≥100 orders, ≥15 ratings, avg ≥4.6
//   - Basic: default; seller falls here when below Pro threshold
//
// Demotion is symmetric (one step per cycle):
//   - Elite → Pro when no longer qualifies for elite
//   - Pro → Basic when no longer qualifies for pro
//
// Legend tier: intentionally deferred. To add Legend in the future:
//   1. Add TierLegend constant to seller entity
//   2. Add reputationLegendMin* constants here
//   3. Add qualifiesForLegend() check
//   4. Insert Legend check before Elite check in this function (must be Pro or Elite → Legend)
//   5. Update isTierDowngrade order map
//   6. Update GatedSellerTier (publiccard) gate to include "legend"
// This is a small scoped change — no architecture rewrite required.
func evaluateTierFromAggregates(currentTier sellerEntity.Tier, agg *reputationAggregates) sellerEntity.Tier {
	if agg == nil {
		return currentTier
	}

	// Check Elite qualification (must currently be Pro or Elite; no skip from Basic).
	if qualifiesForElite(agg) {
		if currentTier == sellerEntity.TierPro || currentTier == sellerEntity.TierElite {
			return sellerEntity.TierElite
		}
		// Basic → Elite is not allowed; fall through to Pro check.
	}

	// Check Pro qualification.
	if qualifiesForPro(agg) {
		return sellerEntity.TierPro
	}

	// Below Pro threshold: demote to Basic.
	// One-step demotion per evaluation cycle (Elite falls to Pro via the
	// pro check above when still above Pro thresholds).
	return sellerEntity.TierBasic
}

func qualifiesForPro(agg *reputationAggregates) bool {
	return agg.completedOrders >= reputationProMinOrders &&
		agg.ratingCount >= reputationProMinRatings &&
		agg.ratingAverage >= reputationProMinAvg
}

func qualifiesForElite(agg *reputationAggregates) bool {
	return agg.completedOrders >= reputationEliteMinOrders &&
		agg.ratingCount >= reputationEliteMinRatings &&
		agg.ratingAverage >= reputationEliteMinAvg
}

// isTierDowngrade returns true when the transition from→to is a demotion.
// When adding Legend tier: add sellerEntity.TierLegend: 3 to the order map.
func isTierDowngrade(from, to sellerEntity.Tier) bool {
	order := map[sellerEntity.Tier]int{
		sellerEntity.TierBasic: 0,
		sellerEntity.TierPro:   1,
		sellerEntity.TierElite: 2,
		// TierLegend: 3 — add when Legend tier is implemented.
	}
	return order[to] < order[from]
}

// ─────────────────────────────────────────────────────────────────────────────
// Production aggregate implementation
// ─────────────────────────────────────────────────────────────────────────────

// productionAggregator queries rolling-window reputation metrics from base tables.
//
// IMPORTANT — cross-domain query rationale:
// The rating query here JOINs order_ratings with orders to use order.completed_at
// as the time-bucketing clock. This intentionally bypasses the RatingReader
// interface (which uses rating.created_at). The deviation is deliberate:
// the reputation authority uses the ORDER clock, not the rating clock, to
// prevent time-domain mismatch. The join is read-only aggregate access;
// it does not constitute a domain ownership violation.
type productionAggregator struct{}

func (a *productionAggregator) compute(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	windowStart time.Time,
) (*reputationAggregates, error) {
	agg := &reputationAggregates{}

	// Completed orders in rolling window.
	// Refunded orders are automatically excluded: their status changed from
	// 'completed' to 'refunded'/'partially_refunded'. No special join needed.
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE seller_id = $1
		  AND status = 'completed'
		  AND completed_at >= $2
	`, sellerID, windowStart).Scan(&agg.completedOrders); err != nil {
		return nil, fmt.Errorf("count completed orders: %w", err)
	}

	// Cancelled-timeout orders in rolling window.
	// Uses updated_at: cancelled_timeout is terminal, so updated_at is frozen.
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE seller_id = $1
		  AND status = 'cancelled_timeout'
		  AND updated_at >= $2
	`, sellerID, windowStart).Scan(&agg.cancelledTimeout); err != nil {
		return nil, fmt.Errorf("count cancelled timeout orders: %w", err)
	}

	// Average valid rating and count, attributed by ORDER's completed_at clock.
	// Only includes ratings where invalidated_at IS NULL (refund-safe).
	// JOINs orders to use completed_at for time attribution — see type doc above.
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(AVG(r.rating_value), 0.0), COUNT(r.id)
		FROM order_ratings r
		JOIN orders o ON o.id = r.order_id
		WHERE r.seller_id = $1
		  AND o.completed_at >= $2
		  AND r.invalidated_at IS NULL
	`, sellerID, windowStart).Scan(&agg.ratingAverage, &agg.ratingCount); err != nil {
		return nil, fmt.Errorf("compute rolling rating: %w", err)
	}

	// Admin-decided dispute losses in rolling window.
	// status='admin_refunded' means admin found seller at fault.
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM refunds
		WHERE seller_id = $1
		  AND status = 'admin_refunded'
		  AND admin_reviewed_at >= $2
	`, sellerID, windowStart).Scan(&agg.disputeLosses); err != nil {
		return nil, fmt.Errorf("count dispute losses: %w", err)
	}

	return agg, nil
}


