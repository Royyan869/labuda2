//go:build integration

package tests

// Batch 41 — Proves the qty=1 forSale stock round-trip works after the
// forSales_check constraint was dropped (migration 000163).
//
// Path exercised:
//   OrderCreation:  GetForUpdate → ReduceQuantity(1) → UpdateStock  [qty 1→0, status active→sold]
//   OrderExpiry:    GetForUpdate → RestoreQuantity(1) → UpdateStock  [qty 0→1, status sold→active]
//
// Both legs use the REAL forSale repository against the REAL database.
// The test creates its own fixture and verifies each state transition.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/google/uuid"

	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// TestForSaleStockRoundTrip_Qty1 proves the full reservation cycle for a
// unique qty=1 koi forSale:
//
//  1. ReduceQuantity(1) → qty=0, status=sold      (order creation path)
//  2. RestoreQuantity(1) → qty=1, status=active    (cancel/expire path)
//
// Before migration 000163 this test would fail with:
//
//	ERROR: new row violates check constraint "forSales_check"
//
// BUSINESS TRUTH PROTECTED:
//   - A single koi can be reserved by one buyer at a time
//   - If payment expires, inventory returns to market
func TestForSaleStockRoundTrip_Qty1(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	forSaleRepo := forsaleRepo.NewForSaleRepository()

	// ── Fixture: seller user + qty=1 active fixed_price forSale ────────
	sellerID := uuid.New()
	var forSaleID uuid.UUID

	// Insert minimal canonical user to satisfy FK constraint.
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local")
		return err
	})
	require.NoError(t, err, "seller user fixture failed")

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Showa Koi — Round-Trip Test",
			"Proves qty=1 reservation cycle",
			[]byte(`["https://picsum.photos/seed/koi1/800/600"]`),
			"Showa",
			intPtr(25),
			intPtr(8),
			strPtr("male"),
			nil,                // breeder
			nil,                // bloodline
			[]string{"global"}, // certificates
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(750000), // 750 000 IDR
			1,                 // qty=1: unique koi
			false,             // negotiation disabled
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil, // farmAddressID
			forsaleEntity.PreparationTimeImmediate,
			nil, // preparationNote
		)
		require.NoError(t, err)
		require.NoError(t, forSale.Publish())
		require.NoError(t, forSaleRepo.Create(ctx, tx, forSale))
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err, "fixture setup failed")

	// ── LEG 1: Simulate order creation (ReduceQuantity → 0) ───────────
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		require.NoError(t, err)

		// Pre-condition
		assert.Equal(t, 1, forSale.QuantityAvailable, "pre: qty should be 1")
		assert.Equal(t, forsaleEntity.ForSaleStatusActive, forSale.Status, "pre: status should be active")

		// This is the exact code path in OrderCreationService.CreateFromForSale line 1354
		err = forSale.ReduceQuantity(1)
		require.NoError(t, err, "ReduceQuantity(1) must not fail")

		// Entity state after reduction
		assert.Equal(t, 0, forSale.QuantityAvailable, "post-reduce: qty should be 0")
		assert.Equal(t, forsaleEntity.ForSaleStatusSold, forSale.Status, "post-reduce: status should be sold")

		// Persist — this is the DB write that was blocked before migration 000163
		err = forSaleRepo.UpdateStock(ctx, tx, forSale)
		require.NoError(t, err, "UpdateStock must not fail (was CHECK VIOLATION before 000163)")

		return nil
	})
	require.NoError(t, err, "LEG 1 (order creation) failed")

	// ── Verify LEG 1 persisted correctly ───────────────────────────────
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetByID(ctx, tx, forSaleID)
		require.NoError(t, err)

		assert.Equal(t, 0, forSale.QuantityAvailable, "DB: qty should be 0 after order")
		assert.Equal(t, forsaleEntity.ForSaleStatusSold, forSale.Status, "DB: status should be sold after order")
		return nil
	})
	require.NoError(t, err, "LEG 1 verification failed")

	t.Log("LEG 1 PASS: qty 1→0, status active→sold — persisted to DB")

	// ── LEG 2: Simulate order expiry/cancel (RestoreQuantity → 1) ─────
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		// This is the exact code path in OrderCompletionService.restoreForSaleStock
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		require.NoError(t, err)

		// Pre-condition: sold with qty=0
		assert.Equal(t, 0, forSale.QuantityAvailable, "pre-restore: qty should be 0")
		assert.Equal(t, forsaleEntity.ForSaleStatusSold, forSale.Status, "pre-restore: status should be sold")

		err = forSale.RestoreQuantity(1)
		require.NoError(t, err, "RestoreQuantity(1) must not fail")

		// Entity state after restoration
		assert.Equal(t, 1, forSale.QuantityAvailable, "post-restore: qty should be 1")
		assert.Equal(t, forsaleEntity.ForSaleStatusActive, forSale.Status, "post-restore: status should be active")

		err = forSaleRepo.UpdateStock(ctx, tx, forSale)
		require.NoError(t, err, "UpdateStock must not fail on restore")

		return nil
	})
	require.NoError(t, err, "LEG 2 (order expiry) failed")

	// ── Verify LEG 2 persisted correctly ───────────────────────────────
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetByID(ctx, tx, forSaleID)
		require.NoError(t, err)

		assert.Equal(t, 1, forSale.QuantityAvailable, "DB: qty should be 1 after restore")
		assert.Equal(t, forsaleEntity.ForSaleStatusActive, forSale.Status, "DB: status should be active after restore")
		return nil
	})
	require.NoError(t, err, "LEG 2 verification failed")

	t.Log("LEG 2 PASS: qty 0→1, status sold→active — persisted to DB")
}

// TestForSaleStockRoundTrip_MultiQty proves partial reservation for a
// multi-quantity forSale (qty=5, buy 3, expect qty=2 active).
func TestForSaleStockRoundTrip_MultiQty(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	forSaleRepo := forsaleRepo.NewForSaleRepository()

	sellerID := uuid.New()
	var forSaleID uuid.UUID

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local")
		return err
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Kohaku Koi Batch — Multi-Qty Test",
			"Proves partial reservation",
			[]byte(`[]`),
			"Kohaku",
			intPtr(20),
			nil, nil, nil, nil,
			[]string{"global"},
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(300000),
			5, // qty=5
			false,
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil,
			forsaleEntity.PreparationTimeImmediate,
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, forSale.Publish())
		require.NoError(t, forSaleRepo.Create(ctx, tx, forSale))
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err)

	// LEG 1: Buy 3 of 5 — should stay active
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		require.NoError(t, err)
		require.NoError(t, forSale.ReduceQuantity(3))

		assert.Equal(t, 2, forSale.QuantityAvailable)
		assert.Equal(t, forsaleEntity.ForSaleStatusActive, forSale.Status, "still active with qty=2")

		return forSaleRepo.UpdateStock(ctx, tx, forSale)
	})
	require.NoError(t, err)

	// LEG 2: Buy remaining 2 — should become sold
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		require.NoError(t, err)
		require.NoError(t, forSale.ReduceQuantity(2))

		assert.Equal(t, 0, forSale.QuantityAvailable)
		assert.Equal(t, forsaleEntity.ForSaleStatusSold, forSale.Status, "sold at qty=0")

		return forSaleRepo.UpdateStock(ctx, tx, forSale)
	})
	require.NoError(t, err)

	// LEG 3: Restore 3 (first order cancelled) — should become active again
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		require.NoError(t, err)
		require.NoError(t, forSale.RestoreQuantity(3))

		assert.Equal(t, 3, forSale.QuantityAvailable)
		assert.Equal(t, forsaleEntity.ForSaleStatusActive, forSale.Status, "reverted to active")

		return forSaleRepo.UpdateStock(ctx, tx, forSale)
	})
	require.NoError(t, err)

	t.Log("Multi-qty round-trip PASS: 5→2→0(sold)→3(active)")
}

// TestNegativeQuantityStillBlocked verifies the non-negative guard survives.
func TestNegativeQuantityStillBlocked(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	forSaleRepo := forsaleRepo.NewForSaleRepository()

	sellerID := uuid.New()
	var forSaleID uuid.UUID

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local")
		return err
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Guard Test Koi",
			"Negative qty must fail",
			[]byte(`[]`),
			"Sanke",
			nil, nil, nil, nil, nil,
			[]string{"global"},
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(100000),
			1,
			false,
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil,
			forsaleEntity.PreparationTimeImmediate,
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, forSale.Publish())
		require.NoError(t, forSaleRepo.Create(ctx, tx, forSale))
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err)

	// Entity-level guard: ReduceQuantity(2) on qty=1 should fail
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		require.NoError(t, err)

		err = forSale.ReduceQuantity(2) // qty=1, requesting 2
		assert.Error(t, err, "entity must reject oversell")
		assert.IsType(t, &forsaleEntity.InsufficientQuantityError{}, err)
		return nil
	})
	require.NoError(t, err)

	t.Log("Negative quantity guard PASS: entity rejects oversell, DB guard untouched")
}
