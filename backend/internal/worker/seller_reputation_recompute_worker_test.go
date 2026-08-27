package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock types for SellerReputationRecomputeWorker
// =============================================================================

// mockReputationAggregator stubs the reputationAggregator interface.
type mockReputationAggregator struct {
	mu       sync.Mutex
	calls    int
	agg      *reputationAggregates
	err      error
}

func (m *mockReputationAggregator) compute(
	_ context.Context, _ db.Tx, _ uuid.UUID, _ time.Time,
) (*reputationAggregates, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.agg, m.err
}

// mockReputationStore stubs the reputationSellerStore interface.
type mockReputationStore struct {
	mu            sync.Mutex
	profile       *sellerEntity.SellerProfile
	getErr        error
	upsertCalls   int
	upsertErr     error
	updateTierCalls int
	updateTierErr error
	lastUpserted  *sellerEntity.SellerReputationState
	lastTier      sellerEntity.Tier
}

func (m *mockReputationStore) GetByIDForUpdate(
	_ context.Context, _ db.Tx, _ uuid.UUID,
) (*sellerEntity.SellerProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.profile, m.getErr
}

func (m *mockReputationStore) UpsertReputationStateTx(
	_ context.Context, _ db.Tx, state *sellerEntity.SellerReputationState,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls++
	m.lastUpserted = state
	return m.upsertErr
}

func (m *mockReputationStore) UpdateTierTx(
	_ context.Context, _ db.Tx, _ uuid.UUID, tier sellerEntity.Tier,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateTierCalls++
	m.lastTier = tier
	return m.updateTierErr
}

// mockReputationOutbox stubs the reputationOutboxStore interface.
type mockReputationOutbox struct {
	mu             sync.Mutex
	inserts        []reputationOutboxCall
	err            error
}

type reputationOutboxCall struct {
	EventType      string
	IdempotencyKey string
	Payload        any
}

func (m *mockReputationOutbox) InsertTx(
	_ context.Context, _ db.Tx, eventType string, payload any, idempotencyKey string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inserts = append(m.inserts, reputationOutboxCall{
		EventType:      eventType,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	})
	return m.err
}

// noSellersTx is a mockTx whose Query always returns empty rows
// (used to simulate fetchAllSellerIDs returning no sellers).
type noSellersTx struct {
	mockTx
}

func (t *noSellersTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
}

// singleSellerTx is a mockTx whose Query returns one seller UUID row.
type singleSellerTx struct {
	mockTx
	sellerID uuid.UUID
}

func (t *singleSellerTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockRows{rows: [][]any{{t.sellerID}}}, nil
}

// newReputationWorkerForTest creates a worker wired with test mocks.
func newReputationWorkerForTest(
	t *testing.T,
	mdb *mockDB,
	store *mockReputationStore,
	outbox *mockReputationOutbox,
	agg *mockReputationAggregator,
) *SellerReputationRecomputeWorker {
	t.Helper()
	w := &SellerReputationRecomputeWorker{
		db:         mdb,
		store:      store,
		outbox:     outbox,
		aggregator: agg,
		log:        zaptest.NewLogger(t),
		interval:   DefaultReputationRecomputeInterval,
		windowDays: DefaultReputationWindowDays,
	}
	return w
}

// =============================================================================
// TESTS: fulfillmentRate
// =============================================================================

func TestReputationAggregates_FulfillmentRate_BothZero(t *testing.T) {
	agg := &reputationAggregates{completedOrders: 0, cancelledTimeout: 0}
	if agg.fulfillmentRate() != 0.0 {
		t.Errorf("expected 0.0, got %f", agg.fulfillmentRate())
	}
}

func TestReputationAggregates_FulfillmentRate_AllCompleted(t *testing.T) {
	agg := &reputationAggregates{completedOrders: 10, cancelledTimeout: 0}
	if agg.fulfillmentRate() != 1.0 {
		t.Errorf("expected 1.0, got %f", agg.fulfillmentRate())
	}
}

func TestReputationAggregates_FulfillmentRate_Mixed(t *testing.T) {
	agg := &reputationAggregates{completedOrders: 7, cancelledTimeout: 3}
	r := agg.fulfillmentRate()
	if r < 0.699 || r > 0.701 {
		t.Errorf("expected ~0.7, got %f", r)
	}
}

// =============================================================================
// TESTS: isTierDowngrade
// =============================================================================

func TestIsTierDowngrade_UpgradeReturnsFalse(t *testing.T) {
	if isTierDowngrade(sellerEntity.TierBasic, sellerEntity.TierPro) {
		t.Error("basic→pro is an upgrade, not a downgrade")
	}
	if isTierDowngrade(sellerEntity.TierPro, sellerEntity.TierElite) {
		t.Error("pro→elite is an upgrade, not a downgrade")
	}
}

func TestIsTierDowngrade_DowngradeReturnsTrue(t *testing.T) {
	if !isTierDowngrade(sellerEntity.TierElite, sellerEntity.TierPro) {
		t.Error("elite→pro is a downgrade")
	}
	if !isTierDowngrade(sellerEntity.TierPro, sellerEntity.TierBasic) {
		t.Error("pro→basic is a downgrade")
	}
	if !isTierDowngrade(sellerEntity.TierElite, sellerEntity.TierBasic) {
		t.Error("elite→basic is a downgrade")
	}
}

func TestIsTierDowngrade_SameTierReturnsFalse(t *testing.T) {
	if isTierDowngrade(sellerEntity.TierPro, sellerEntity.TierPro) {
		t.Error("same tier is not a downgrade")
	}
}

// =============================================================================
// TESTS: qualifiesForPro / qualifiesForElite
// =============================================================================

func TestQualifiesForPro_AllMet(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders,
		ratingCount:     reputationProMinRatings,
		ratingAverage:   reputationProMinAvg,
	}
	if !qualifiesForPro(agg) {
		t.Error("should qualify for Pro when exactly at threshold")
	}
}

func TestQualifiesForPro_InsufficientOrders(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders - 1,
		ratingCount:     reputationProMinRatings,
		ratingAverage:   reputationProMinAvg,
	}
	if qualifiesForPro(agg) {
		t.Error("should not qualify for Pro with insufficient orders")
	}
}

func TestQualifiesForPro_InsufficientRatings(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders,
		ratingCount:     reputationProMinRatings - 1,
		ratingAverage:   reputationProMinAvg,
	}
	if qualifiesForPro(agg) {
		t.Error("should not qualify for Pro with insufficient ratings")
	}
}

func TestQualifiesForPro_AverageBelowThreshold(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders,
		ratingCount:     reputationProMinRatings,
		ratingAverage:   reputationProMinAvg - 0.01,
	}
	if qualifiesForPro(agg) {
		t.Error("should not qualify for Pro with average below threshold")
	}
}

func TestQualifiesForElite_AllMet(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationEliteMinOrders,
		ratingCount:     reputationEliteMinRatings,
		ratingAverage:   reputationEliteMinAvg,
	}
	if !qualifiesForElite(agg) {
		t.Error("should qualify for Elite when exactly at threshold")
	}
}

func TestQualifiesForElite_InsufficientOrders(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationEliteMinOrders - 1,
		ratingCount:     reputationEliteMinRatings,
		ratingAverage:   reputationEliteMinAvg,
	}
	if qualifiesForElite(agg) {
		t.Error("should not qualify for Elite with insufficient orders")
	}
}

// =============================================================================
// TESTS: evaluateTierFromAggregates (pure function)
// =============================================================================

func TestEvaluateTier_NilAggKeepsTier(t *testing.T) {
	result := evaluateTierFromAggregates(sellerEntity.TierPro, nil)
	if result != sellerEntity.TierPro {
		t.Errorf("nil agg should preserve current tier, got %s", result)
	}
}

func TestEvaluateTier_BelowProStaysBasic(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 5,
		ratingCount:     2,
		ratingAverage:   4.0,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierBasic, agg)
	if result != sellerEntity.TierBasic {
		t.Errorf("expected Basic, got %s", result)
	}
}

func TestEvaluateTier_MeetsProFromBasic(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders,
		ratingCount:     reputationProMinRatings,
		ratingAverage:   reputationProMinAvg,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierBasic, agg)
	if result != sellerEntity.TierPro {
		t.Errorf("expected Pro, got %s", result)
	}
}

func TestEvaluateTier_MeetsEliteFromPro(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationEliteMinOrders,
		ratingCount:     reputationEliteMinRatings,
		ratingAverage:   reputationEliteMinAvg,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierPro, agg)
	if result != sellerEntity.TierElite {
		t.Errorf("expected Elite, got %s", result)
	}
}

// No-skip-tier: Basic cannot jump directly to Elite.
func TestEvaluateTier_BasicCannotSkipToElite(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationEliteMinOrders,
		ratingCount:     reputationEliteMinRatings,
		ratingAverage:   reputationEliteMinAvg,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierBasic, agg)
	// Must be Pro, not Elite — no skip-tier promotion.
	if result != sellerEntity.TierPro {
		t.Errorf("Basic seller meeting Elite thresholds should land on Pro (no skip tier), got %s", result)
	}
}

// Elite stays Elite when still meeting Elite thresholds.
func TestEvaluateTier_EliteStaysElite(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationEliteMinOrders,
		ratingCount:     reputationEliteMinRatings,
		ratingAverage:   reputationEliteMinAvg,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierElite, agg)
	if result != sellerEntity.TierElite {
		t.Errorf("Elite seller meeting Elite thresholds should stay Elite, got %s", result)
	}
}

// Elite drops to Pro when below Elite but above Pro thresholds.
func TestEvaluateTier_EliteDropsToProWhenBelowEliteThreshold(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders, // above Pro, below Elite
		ratingCount:     reputationProMinRatings,
		ratingAverage:   reputationProMinAvg,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierElite, agg)
	if result != sellerEntity.TierPro {
		t.Errorf("Elite seller meeting only Pro thresholds should drop to Pro, got %s", result)
	}
}

// Pro drops to Basic when below Pro thresholds.
func TestEvaluateTier_ProDropsToBasic(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: reputationProMinOrders - 1,
		ratingCount:     reputationProMinRatings - 1,
		ratingAverage:   reputationProMinAvg - 0.5,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierPro, agg)
	if result != sellerEntity.TierBasic {
		t.Errorf("Pro seller below Pro thresholds should drop to Basic, got %s", result)
	}
}

// =============================================================================
// TESTS: processOneSeller
// =============================================================================

func TestProcessOneSeller_TierUnchanged_NoOutboxEvent(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	agg := &mockReputationAggregator{
		agg: &reputationAggregates{
			completedOrders: 5, // below Pro threshold
			ratingCount:     2,
			ratingAverage:   4.0,
		},
	}
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{
			ID:   sellerID,
			Tier: sellerEntity.TierBasic,
		},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	changed, err := w.processOneSeller(context.Background(), sellerID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("tier should not have changed")
	}

	// Outbox must NOT be called when tier unchanged.
	outbox.mu.Lock()
	insertCount := len(outbox.inserts)
	outbox.mu.Unlock()
	if insertCount != 0 {
		t.Errorf("expected 0 outbox events, got %d", insertCount)
	}

	// State must still be upserted (state is always written).
	store.mu.Lock()
	upserts := store.upsertCalls
	store.mu.Unlock()
	if upserts != 1 {
		t.Errorf("expected 1 upsert, got %d", upserts)
	}
}

func TestProcessOneSeller_TierUpgrade_EmitsUpgradedEvent(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	agg := &mockReputationAggregator{
		agg: &reputationAggregates{
			completedOrders: reputationProMinOrders,
			ratingCount:     reputationProMinRatings,
			ratingAverage:   reputationProMinAvg,
		},
	}
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{
			ID:   sellerID,
			Tier: sellerEntity.TierBasic, // current: Basic
		},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	changed, err := w.processOneSeller(context.Background(), sellerID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("tier should have changed (Basic → Pro)")
	}

	outbox.mu.Lock()
	inserts := outbox.inserts
	outbox.mu.Unlock()
	if len(inserts) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(inserts))
	}
	if inserts[0].EventType != "seller.tier.upgraded" {
		t.Errorf("expected event 'seller.tier.upgraded', got '%s'", inserts[0].EventType)
	}

	store.mu.Lock()
	newTier := store.lastTier
	store.mu.Unlock()
	if newTier != sellerEntity.TierPro {
		t.Errorf("expected stored tier Pro, got %s", newTier)
	}
}

func TestProcessOneSeller_TierDowngrade_EmitsDowngradedEvent(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	agg := &mockReputationAggregator{
		agg: &reputationAggregates{
			completedOrders: 2, // way below Pro threshold
			ratingCount:     1,
			ratingAverage:   3.5,
		},
	}
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{
			ID:   sellerID,
			Tier: sellerEntity.TierPro, // current: Pro
		},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	changed, err := w.processOneSeller(context.Background(), sellerID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("tier should have changed (Pro → Basic)")
	}

	outbox.mu.Lock()
	inserts := outbox.inserts
	outbox.mu.Unlock()
	if len(inserts) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(inserts))
	}
	if inserts[0].EventType != "seller.tier.downgraded" {
		t.Errorf("expected event 'seller.tier.downgraded', got '%s'", inserts[0].EventType)
	}
}

// Replay safety: calling processOneSeller twice produces the same upsert
// without a second outbox event on the second call (tier already updated).
func TestProcessOneSeller_ReplaySafe_NoDoubleOutboxEvent(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	agg := &mockReputationAggregator{
		agg: &reputationAggregates{
			completedOrders: reputationProMinOrders,
			ratingCount:     reputationProMinRatings,
			ratingAverage:   reputationProMinAvg,
		},
	}
	// Simulate: after first call the profile's tier has been updated to Pro.
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{
			ID:   sellerID,
			Tier: sellerEntity.TierBasic,
		},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}
	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)

	// First call: upgrades Basic → Pro.
	changed1, err := w.processOneSeller(context.Background(), sellerID, now)
	if err != nil || !changed1 {
		t.Fatalf("first call: err=%v changed=%v", err, changed1)
	}

	// Simulate the profile now reflects the new tier (as DB would after first call).
	store.mu.Lock()
	store.profile.Tier = sellerEntity.TierPro
	store.mu.Unlock()

	// Second call (same aggregates, tier already Pro): no change.
	changed2, err := w.processOneSeller(context.Background(), sellerID, now)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if changed2 {
		t.Error("second call should report no tier change (tier already Pro)")
	}

	// Only one outbox event total — the second call is idempotent.
	outbox.mu.Lock()
	total := len(outbox.inserts)
	outbox.mu.Unlock()
	if total != 1 {
		t.Errorf("expected exactly 1 outbox event across both calls, got %d", total)
	}
}

// Aggregate compute error causes processOneSeller to fail.
func TestProcessOneSeller_AggregateError_PropagatesError(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	agg := &mockReputationAggregator{err: errors.New("db timeout")}
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{ID: sellerID, Tier: sellerEntity.TierBasic},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	_, err := w.processOneSeller(context.Background(), sellerID, now)
	if err == nil {
		t.Error("expected error from aggregate failure")
	}
}

// Nil profile (seller deleted between ID fetch and profile lock) → error.
func TestProcessOneSeller_ProfileNotFound_ReturnsError(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	agg := &mockReputationAggregator{
		agg: &reputationAggregates{completedOrders: 50, ratingCount: 20, ratingAverage: 4.7},
	}
	store := &mockReputationStore{profile: nil} // nil = not found
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	_, err := w.processOneSeller(context.Background(), sellerID, now)
	if err == nil {
		t.Error("expected error when seller profile not found")
	}
}

// =============================================================================
// TESTS: idempotency key format
// =============================================================================

func TestProcessOneSeller_IdempotencyKey_ContainsSellerIDAndDate(t *testing.T) {
	sellerID := uuid.New()
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)

	agg := &mockReputationAggregator{
		agg: &reputationAggregates{
			completedOrders: reputationProMinOrders,
			ratingCount:     reputationProMinRatings,
			ratingAverage:   reputationProMinAvg,
		},
	}
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{ID: sellerID, Tier: sellerEntity.TierBasic},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	_, _ = w.processOneSeller(context.Background(), sellerID, now)

	outbox.mu.Lock()
	inserts := outbox.inserts
	outbox.mu.Unlock()

	if len(inserts) == 0 {
		t.Fatal("expected outbox event")
	}
	key := inserts[0].IdempotencyKey
	if !strings.Contains(key, sellerID.String()) {
		t.Errorf("idempotency key must contain seller ID, got: %s", key)
	}
	if !strings.Contains(key, "2026-05-28") {
		t.Errorf("idempotency key must contain date 2026-05-28, got: %s", key)
	}
}

// =============================================================================
// TESTS: state upsert fields
// =============================================================================

func TestProcessOneSeller_UpsertedState_ReflectsAggregates(t *testing.T) {
	sellerID := uuid.New()
	now := time.Now().UTC()

	inputAgg := &reputationAggregates{
		completedOrders:  45,
		cancelledTimeout: 5,
		ratingAverage:    4.8,
		ratingCount:      22,
		disputeLosses:    1,
	}
	agg := &mockReputationAggregator{agg: inputAgg}
	store := &mockReputationStore{
		profile: &sellerEntity.SellerProfile{ID: sellerID, Tier: sellerEntity.TierPro},
	}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	_, _ = w.processOneSeller(context.Background(), sellerID, now)

	store.mu.Lock()
	state := store.lastUpserted
	store.mu.Unlock()

	if state == nil {
		t.Fatal("expected upserted state, got nil")
	}
	if state.SellerID != sellerID {
		t.Errorf("seller ID mismatch: got %s", state.SellerID)
	}
	if state.RollingCompletedOrders != 45 {
		t.Errorf("expected RollingCompletedOrders=45, got %d", state.RollingCompletedOrders)
	}
	if state.RollingCancelledTimeout != 5 {
		t.Errorf("expected RollingCancelledTimeout=5, got %d", state.RollingCancelledTimeout)
	}
	if state.RollingRatingAverage != 4.8 {
		t.Errorf("expected RollingRatingAverage=4.8, got %f", state.RollingRatingAverage)
	}
	if state.RollingDisputeLossCount != 1 {
		t.Errorf("expected RollingDisputeLossCount=1, got %d", state.RollingDisputeLossCount)
	}
	if state.WindowDays != DefaultReputationWindowDays {
		t.Errorf("expected WindowDays=%d, got %d", DefaultReputationWindowDays, state.WindowDays)
	}
	// FulfillmentRate = 45 / (45+5) = 0.9
	expectedRate := float64(45) / float64(50)
	if state.RollingFulfillmentRate < expectedRate-0.001 || state.RollingFulfillmentRate > expectedRate+0.001 {
		t.Errorf("expected FulfillmentRate≈%f, got %f", expectedRate, state.RollingFulfillmentRate)
	}
}

// =============================================================================
// TESTS: RecomputeAllSellers — no sellers
// =============================================================================

func TestRecomputeAllSellers_NoSellers_EarlyReturn(t *testing.T) {
	agg := &mockReputationAggregator{}
	store := &mockReputationStore{}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{
		WithTxFunc: func(_ context.Context, fn func(db.Tx) error) error {
			return fn(&noSellersTx{})
		},
	}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	w.RecomputeAllSellers(context.Background())

	// Aggregator must not have been called (no sellers to process).
	agg.mu.Lock()
	calls := agg.calls
	agg.mu.Unlock()
	if calls != 0 {
		t.Errorf("expected 0 aggregator calls, got %d", calls)
	}
}

// =============================================================================
// TESTS: RecomputeAllSellers — fetch error is logged, not panicked
// =============================================================================

func TestRecomputeAllSellers_FetchError_DoesNotPanic(t *testing.T) {
	agg := &mockReputationAggregator{}
	store := &mockReputationStore{}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{
		WithTxFunc: func(_ context.Context, _ func(db.Tx) error) error {
			return errors.New("connection lost")
		},
	}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	// Must not panic.
	w.RecomputeAllSellers(context.Background())
}

// =============================================================================
// TESTS: Lifecycle — Start / Stop / IsRunning
// =============================================================================

func TestReputationWorker_Lifecycle_StartStopIsRunning(t *testing.T) {
	agg := &mockReputationAggregator{agg: &reputationAggregates{}}
	store := &mockReputationStore{}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{
		WithTxFunc: func(_ context.Context, fn func(db.Tx) error) error {
			return fn(&noSellersTx{})
		},
	}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	w.interval = 10 * time.Second // slow interval to avoid fast cycles in test

	if w.IsRunning() {
		t.Error("worker should not be running before Start()")
	}

	w.Start()
	if !w.IsRunning() {
		t.Error("worker should be running after Start()")
	}

	w.Stop()
	if w.IsRunning() {
		t.Error("worker should not be running after Stop()")
	}
}

// DoubleStart: calling Start twice is a no-op (idempotent).
func TestReputationWorker_DoubleStart_IsNoop(t *testing.T) {
	agg := &mockReputationAggregator{agg: &reputationAggregates{}}
	store := &mockReputationStore{}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{
		WithTxFunc: func(_ context.Context, fn func(db.Tx) error) error {
			return fn(&noSellersTx{})
		},
	}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	w.interval = 10 * time.Second

	w.Start()
	w.Start() // second call must not panic or add extra goroutines
	defer w.Stop()

	if !w.IsRunning() {
		t.Error("worker should be running after double Start()")
	}
}

// Stop before Start is a no-op (no panic).
func TestReputationWorker_StopBeforeStart_IsNoop(t *testing.T) {
	agg := &mockReputationAggregator{agg: &reputationAggregates{}}
	store := &mockReputationStore{}
	outbox := &mockReputationOutbox{}
	mdb := &mockDB{}

	w := newReputationWorkerForTest(t, mdb, store, outbox, agg)
	w.Stop() // must not panic
}

// =============================================================================
// TESTS: productionAggregator SQL semantics (query shape verification)
// =============================================================================

// TestProductionAggregator_CompletedOrders_FiltersByStatusAndCompletedAt verifies
// the SQL uses status='completed' and completed_at for the rolling window.
func TestProductionAggregator_CompletedOrders_FiltersByStatusAndCompletedAt(t *testing.T) {
	capturedSQLs := make([]string, 0, 4)
	tx := &mockTx{
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			capturedSQLs = append(capturedSQLs, sql)
			// Return (0, 0.0) pair for the rating query and 0 for count queries.
			return &mockRow{values: []any{0, 0.0}}
		},
	}

	// Override to handle the COUNT-only queries (1 value).
	callNum := 0
	tx.QueryRowFunc = func(_ context.Context, sql string, _ ...any) pgx.Row {
		capturedSQLs = append(capturedSQLs, sql)
		callNum++
		switch callNum {
		case 1, 2, 4: // completed count, cancelled count, dispute count
			return &mockRow{values: []any{0}}
		case 3: // rating avg + count
			return &mockRow{values: []any{0.0, 0}}
		default:
			return &mockRow{err: errors.New("unexpected call")}
		}
	}

	a := &productionAggregator{}
	_, err := a.compute(context.Background(), tx, uuid.New(), time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedSQLs) < 1 {
		t.Fatal("expected at least 1 SQL query")
	}

	// First query: completed orders.
	completedSQL := capturedSQLs[0]
	if !strings.Contains(completedSQL, "status = 'completed'") {
		t.Errorf("completed orders query must filter by status='completed', got: %s", completedSQL)
	}
	if !strings.Contains(completedSQL, "completed_at") {
		t.Errorf("completed orders query must use completed_at, got: %s", completedSQL)
	}

	// Second query: cancelled timeout.
	if len(capturedSQLs) >= 2 {
		cancelSQL := capturedSQLs[1]
		if !strings.Contains(cancelSQL, "'cancelled_timeout'") {
			t.Errorf("cancelled timeout query must use 'cancelled_timeout', got: %s", cancelSQL)
		}
	}
}

// TestProductionAggregator_RatingQuery_JoinsOrdersAndFiltersInvalidated verifies
// the rating SQL JOINs orders (for completed_at clock) and filters invalidated_at IS NULL.
func TestProductionAggregator_RatingQuery_JoinsOrdersAndFiltersInvalidated(t *testing.T) {
	var ratingSQL string
	callNum := 0
	tx := &mockTx{
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			callNum++
			if callNum == 3 {
				ratingSQL = sql
				return &mockRow{values: []any{0.0, 0}}
			}
			return &mockRow{values: []any{0}}
		},
	}

	a := &productionAggregator{}
	_, _ = a.compute(context.Background(), tx, uuid.New(), time.Now().Add(-90*24*time.Hour))

	if !strings.Contains(ratingSQL, "JOIN orders") {
		t.Errorf("rating query must JOIN orders (for completed_at clock), got: %s", ratingSQL)
	}
	if !strings.Contains(ratingSQL, "invalidated_at IS NULL") {
		t.Errorf("rating query must filter invalidated_at IS NULL (refund-safe), got: %s", ratingSQL)
	}
	if !strings.Contains(ratingSQL, "completed_at") {
		t.Errorf("rating query must use completed_at from orders, got: %s", ratingSQL)
	}
}

// TestProductionAggregator_DisputeQuery_FiltersAdminRefunded verifies the dispute
// loss query uses status='admin_refunded' (seller-at-fault authority).
// =============================================================================
// TESTS: Threshold recalibration regression (2026-05-28)
//
// These tests validate behaviour after Pro 30→100, Elite 100→300 recalibration.
// They use hardcoded values deliberately (not constants) so they FAIL if
// thresholds are accidentally reverted.
// =============================================================================

// Seller who was Pro under old thresholds (30 orders) is now Basic.
func TestRecalibration_OldProNowBasic(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 30,
		ratingCount:     15,
		ratingAverage:   4.6,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierPro, agg)
	if result != sellerEntity.TierBasic {
		t.Errorf("seller with 30 orders should be Basic under new thresholds, got %s", result)
	}
}

// Seller who was Elite under old thresholds (100 orders) is now Pro at best.
func TestRecalibration_OldEliteNowPro(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 100,
		ratingCount:     50,
		ratingAverage:   4.7,
	}
	result := evaluateTierFromAggregates(sellerEntity.TierElite, agg)
	if result != sellerEntity.TierPro {
		t.Errorf("seller with 100 orders should be Pro (not Elite) under new thresholds, got %s", result)
	}
}

// Exact new Pro boundary: 100 orders, 15 ratings, 4.6 avg.
func TestRecalibration_ExactNewProBoundary(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 100,
		ratingCount:     15,
		ratingAverage:   4.6,
	}
	if !qualifiesForPro(agg) {
		t.Error("100 orders + 15 ratings + 4.6 avg should qualify for Pro")
	}
	if qualifiesForElite(agg) {
		t.Error("100 orders should NOT qualify for Elite (need 300)")
	}
}

// Exact new Elite boundary: 300 orders, 50 ratings, 4.7 avg.
func TestRecalibration_ExactNewEliteBoundary(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 300,
		ratingCount:     50,
		ratingAverage:   4.7,
	}
	if !qualifiesForElite(agg) {
		t.Error("300 orders + 50 ratings + 4.7 avg should qualify for Elite")
	}
}

// 299 orders does not qualify for Elite.
func TestRecalibration_JustBelowElite(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 299,
		ratingCount:     50,
		ratingAverage:   4.7,
	}
	if qualifiesForElite(agg) {
		t.Error("299 orders should NOT qualify for Elite")
	}
}

// 99 orders does not qualify for Pro.
func TestRecalibration_JustBelowPro(t *testing.T) {
	agg := &reputationAggregates{
		completedOrders: 99,
		ratingCount:     15,
		ratingAverage:   4.6,
	}
	if qualifiesForPro(agg) {
		t.Error("99 orders should NOT qualify for Pro")
	}
}

// Legend tier does not leak: unknown tier in isTierDowngrade returns 0 (Basic level).
func TestLegendTierDoesNotLeak(t *testing.T) {
	// "legend" is not a valid tier value — should be treated as unknown.
	unknownTier := sellerEntity.Tier("legend")
	// isTierDowngrade uses a map; absent key returns 0 (same as Basic).
	// From Pro to "legend" should NOT be a downgrade because legend=0 < pro=1.
	if !isTierDowngrade(sellerEntity.TierPro, unknownTier) {
		t.Error("unknown tier should have order=0 (like Basic), so Pro→unknown is a downgrade")
	}
}

// Constant values are what we expect (guard against accidental revert).
func TestRecalibration_ConstantsAreCorrect(t *testing.T) {
	if reputationProMinOrders != 100 {
		t.Errorf("reputationProMinOrders should be 100, got %d", reputationProMinOrders)
	}
	if reputationEliteMinOrders != 300 {
		t.Errorf("reputationEliteMinOrders should be 300, got %d", reputationEliteMinOrders)
	}
	if reputationProMinRatings != 15 {
		t.Errorf("reputationProMinRatings should be 15, got %d", reputationProMinRatings)
	}
	if reputationEliteMinRatings != 50 {
		t.Errorf("reputationEliteMinRatings should be 50, got %d", reputationEliteMinRatings)
	}
}

func TestProductionAggregator_DisputeQuery_FiltersAdminRefunded(t *testing.T) {
	var disputeSQL string
	callNum := 0
	tx := &mockTx{
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			callNum++
			if callNum == 4 {
				disputeSQL = sql
				return &mockRow{values: []any{0}}
			}
			if callNum == 3 {
				return &mockRow{values: []any{0.0, 0}}
			}
			return &mockRow{values: []any{0}}
		},
	}

	a := &productionAggregator{}
	_, _ = a.compute(context.Background(), tx, uuid.New(), time.Now().Add(-90*24*time.Hour))

	if !strings.Contains(disputeSQL, "'admin_refunded'") {
		t.Errorf("dispute loss query must filter by 'admin_refunded', got: %s", disputeSQL)
	}
}


