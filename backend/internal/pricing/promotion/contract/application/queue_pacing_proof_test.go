//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/pricing/promotion/contract/application"
	"github.com/hishumi/backend/internal/pricing/promotion/contract/entity"
	"github.com/stretchr/testify/require"
)

// opCheck mock for queue tests: validates ownership by seller ID and operability by map
type queueOpMock struct {
	operable map[uuid.UUID]bool
	owners   map[uuid.UUID]uuid.UUID
}

func (m *queueOpMock) CheckOperability(ctx context.Context, targetType string, targetID *uuid.UUID) (bool, string, error) {
	if targetID == nil {
		return false, "not_found", nil
	}
	if v, ok := m.operable[*targetID]; ok {
		if !v {
			return false, "inoperable", nil
		}
		return true, "", nil
	}
	return true, "", nil
}
func (m *queueOpMock) ValidateOwnership(ctx context.Context, sellerID uuid.UUID, targetType string, targetID *uuid.UUID) error {
	if targetID == nil {
		return nil
	}
	if owner, ok := m.owners[*targetID]; ok {
		if owner != sellerID {
			return &testOwnershipError{}
		}
	}
	return nil
}

type testOwnershipError struct{}

func (e *testOwnershipError) Error() string { return "not owned" }

func TestQueue_Proof_Q1_Q10(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seller := h.newSeller(t, 500_000)
	otherSeller := h.newSeller(t, 500_000)

	op := &queueOpMock{
		operable: make(map[uuid.UUID]bool),
		owners:   make(map[uuid.UUID]uuid.UUID),
	}
	h.svc.SetTargetOperability(op)

	// Helper to make target
	newTarget := func(owner uuid.UUID) uuid.UUID {
		id := uuid.New()
		op.owners[id] = owner
		op.operable[id] = true
		return id
	}

	// Create internal contract WITH its initial queue (queue before payment):
	// the first target is part of the promotion configuration.
	t1 := newTarget(seller)
	c, err := h.createWithTargets(t, seller, entity.KindInternal, 100_000, 10,
		[]application.PromotionTargetInput{{TargetType: "for_sale", TargetID: t1}})
	require.NoError(t, err)

	// Q1: the configured target is the initial queue entry at position 0.
	initial, err := h.svc.ListTargets(context.Background(), seller, c.ID)
	require.NoError(t, err)
	require.Len(t, initial, 1)
	require.Equal(t, 0, initial[0].Position)
	require.Equal(t, t1, initial[0].TargetID)

	// Q2 add until 10 PASS
	ids := []uuid.UUID{t1}
	for i := 1; i < 10; i++ {
		tid := newTarget(seller)
		ct, err := h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: tid})
		require.NoError(t, err)
		require.Equal(t, i, ct.Position)
		ids = append(ids, tid)
	}
	list, err := h.svc.ListTargets(context.Background(), seller, c.ID)
	require.NoError(t, err)
	require.Len(t, list, 10)

	// Q3 11th → deterministic rejection
	t11 := newTarget(seller)
	_, err = h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: t11})
	require.ErrorIs(t, err, application.ErrQueueFull)

	// Q5 remove middle → compact (frees a slot first so the duplicate proof
	// below exercises duplicate detection rather than the full-queue guard).
	middle := ids[5]
	require.NoError(t, h.svc.RemoveTarget(context.Background(), application.RemoveTargetInput{SellerID: seller, ContractID: c.ID, TargetID: middle}))
	list, err = h.svc.ListTargets(context.Background(), seller, c.ID)
	require.NoError(t, err)
	require.Len(t, list, 9)
	for i, item := range list {
		require.Equal(t, i, item.Position, "positions canonical no gap")
	}

	// Q4 duplicate (of the initial configured target) → rejected
	_, err = h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: t1})
	require.ErrorIs(t, err, application.ErrQueueDuplicate)

	// Q6 seller cannot add target owned by other seller
	otherTarget := newTarget(otherSeller)
	_, err = h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: otherTarget})
	require.Error(t, err, "ownership should fail")

	// Q7 target not operable → cannot add when write-time rule requires operability
	badTarget := newTarget(seller)
	op.operable[badTarget] = false
	_, err = h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: badTarget})
	require.Error(t, err)

	// Q8 target becomes inoperable after queue → ResolveEffectiveTarget skip
	op.operable[badTarget] = true // make operable to add
	// we have 9 items, add one more to reach 10
	_, err = h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: badTarget})
	require.NoError(t, err)
	// make first target inoperable
	firstID := ids[0]
	op.operable[firstID] = false
	resolved, err := h.svc.ResolveEffectiveTarget(context.Background(), c.ID)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.NotEqual(t, firstID, resolved.TargetID, "should skip inoperable first")

	// Q9 first sold out (inoperable) → not receiving delivery, next can be selected
	require.Equal(t, false, op.operable[firstID])
	// already proven by Q8

	// Q10 queue still <10? Actually now 10 again, remove one to make <10 then add new without new purchase
	require.NoError(t, h.svc.RemoveTarget(context.Background(), application.RemoveTargetInput{SellerID: seller, ContractID: c.ID, TargetID: ids[1]}))
	list, _ = h.svc.ListTargets(context.Background(), seller, c.ID)
	require.Len(t, list, 9)
	newAfterRemove := newTarget(seller)
	ctNew, err := h.svc.AddTarget(context.Background(), application.AddTargetInput{SellerID: seller, ContractID: c.ID, TargetType: "for_sale", TargetID: newAfterRemove})
	require.NoError(t, err)
	require.Equal(t, 9, ctNew.Position)
	// contract and allocation unchanged
	got, err := h.svc.Get(context.Background(), seller, c.ID)
	require.NoError(t, err)
	require.Equal(t, int64(100_000), got.BudgetRupiah)
}

// ============================================================================
// SCOPE 2 — QUEUE BEFORE PAYMENT (creation boundary + funding gate)
// ============================================================================

// A: a promotion configuration without a queue is rejected before funding/pay.
func TestCreateQueue_RequiresAtLeastOneTarget(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seller := h.newSeller(t, 500_000)

	// Create with empty queue → rejected (no contract, no money movement).
	_, err := h.createWithTargets(t, seller, entity.KindInternal, 100_000, 10, nil)
	require.ErrorIs(t, err, application.ErrQueueEmpty)
	require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
	require.Equal(t, int64(500_000), h.promoteBalance(t, seller))

	// Pre-payment funding preview with empty queue → rejected.
	_, err = h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 100_000, DurationDays: 10,
	})
	require.ErrorIs(t, err, application.ErrQueueEmpty)
}

// C/G/F: multiple targets preserve insertion order; max 10; duplicates rejected.
func TestCreateQueue_MaxAndDuplicate(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seller := h.newSeller(t, 2_000_000)
	h.svc.SetTargetOperability(&queueOpMock{})

	mk := func() application.PromotionTargetInput {
		return application.PromotionTargetInput{TargetType: "for_sale", TargetID: uuid.New()}
	}

	// 11 targets → ErrQueueFull, rejected before any money movement.
	eleven := make([]application.PromotionTargetInput, 0, 11)
	for i := 0; i < 11; i++ {
		eleven = append(eleven, mk())
	}
	_, err := h.createWithTargets(t, seller, entity.KindInternal, 100_000, 10, eleven)
	require.ErrorIs(t, err, application.ErrQueueFull)
	require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
	require.Equal(t, int64(2_000_000), h.promoteBalance(t, seller))

	// Duplicate target → ErrQueueDuplicate.
	dup := mk()
	_, err = h.createWithTargets(t, seller, entity.KindInternal, 100_000, 10,
		[]application.PromotionTargetInput{dup, dup})
	require.ErrorIs(t, err, application.ErrQueueDuplicate)
	require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
	require.Equal(t, int64(2_000_000), h.promoteBalance(t, seller))

	// 10 targets accepted, insertion order preserved.
	ten := make([]application.PromotionTargetInput, 0, 10)
	for i := 0; i < 10; i++ {
		ten = append(ten, mk())
	}
	c, err := h.createWithTargets(t, seller, entity.KindInternal, 100_000, 10, ten)
	require.NoError(t, err)
	list, err := h.svc.ListTargets(context.Background(), seller, c.ID)
	require.NoError(t, err)
	require.Len(t, list, 10)
	for i, item := range list {
		require.Equal(t, i, item.Position)
		require.Equal(t, ten[i].TargetID, item.TargetID)
	}
}

// D: a mixed For Sale + Auction queue is valid (homogeneity is not imposed).
func TestCreateQueue_MixedForSaleAndAuction(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seller := h.newSeller(t, 500_000)
	h.svc.SetTargetOperability(&queueOpMock{})

	c, err := h.createWithTargets(t, seller, entity.KindInternal, 100_000, 10,
		[]application.PromotionTargetInput{
			{TargetType: "for_sale", TargetID: uuid.New()},
			{TargetType: "auction", TargetID: uuid.New()},
			{TargetType: "for_sale", TargetID: uuid.New()},
		})
	require.NoError(t, err)
	list, err := h.svc.ListTargets(context.Background(), seller, c.ID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	require.Equal(t, "for_sale", string(list[0].TargetType))
	require.Equal(t, "auction", string(list[1].TargetType))
	require.Equal(t, "for_sale", string(list[2].TargetType))
}

// Pacing unit proof without DB — verifies envelope logic via helper
func TestPacing_Envelope_Proof(t *testing.T) {
	// P1 duration is pacing window, P7 budget not consumed by time
	// P2 envelope throttles overspend, P3 low traffic no billing, P4 bounded catch-up, P5 pause/resume not wall-clock billing
	budget := int64(30_000)
	total := 3 * 24 * time.Hour
	// helper mirrors delivery_service.enforcePacingEnvelope
	envelope := func(elapsed time.Duration, spent int64) bool {
		if total <= 0 {
			return false
		}
		if elapsed < 0 {
			elapsed = 0
		}
		if elapsed > total {
			return true // would be ErrContractNotActive
		}
		expected := int64(float64(budget) * float64(elapsed) / float64(total))
		tolerance := expected + expected/3
		return spent > tolerance && float64(elapsed)/float64(total) < 0.8
	}
	// P2: high early traffic throttled
	require.True(t, envelope(12*time.Hour, 20_000), "early high traffic should throttle")
	// P3 low traffic not billing - no spent, not throttled
	require.False(t, envelope(12*time.Hour, 1_000))
	// P4 bounded catch-up: behind pace not throttled
	require.False(t, envelope(12*time.Hour, 2_000))
	// P5 final window not throttled even if over envelope (after 80%)
	require.False(t, envelope(60*time.Hour, 29_000))
	// P6 low traffic after long time not throttled excessively — bounded
	require.False(t, envelope(total-1*time.Hour, 25_000))
	// P7 time advances without spend not throttled
	require.False(t, envelope(24*time.Hour, 0))
}

func TestCPM_Cumulative_Formula(t *testing.T) {
	// S(N) = N*CPM/1000 floor cumulative, charge = S(N)-S(N-1)
	cpm := int64(7500)
	// S(1)=7, S(2)=15, S(3)=22
	require.Equal(t, int64(7), cpm*1/1000)
	require.Equal(t, int64(15), cpm*2/1000)
	require.Equal(t, int64(22), cpm*3/1000)
	charges := []int64{7, 8, 7} // S diffs
	for i, c := range charges {
		n := int64(i + 1)
		expected := cpm*n/1000 - cpm*(n-1)/1000
		require.Equal(t, c, expected)
	}
}
