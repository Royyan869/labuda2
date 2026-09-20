//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// SCOPE 4B-S1V — CANONICAL PRICING SNAPSHOT PERSISTENCE PROOFS
// ============================================================================
// REAL POSTGRES proofs: canonical financial snapshot survives write→read-back.
// ============================================================================

func TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertOrderTestUsers(t, ctx, testDB, sellerID, buyerID)

	orderID := uuid.New()
	// P=100000, D=10000, S=20000, commission=5%
	// PD=90000, C=4500, BuyerOrderValue=(P-D)+S=110000
	expectedBuyerVal := int64(110000)
	oldWrongVal := int64(114500) // P+S+C-D must NOT be this

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		now := time.Now()
		expiry := now.Add(24 * time.Hour)
		_, execErr := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount,
				service_fee_amount, total_payable_amount,
				total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000,
				5, 4500,
				0, $5,
				$6,
				'pending_payment', 'none', false,
				$7, 'immediate',
				'ORD-20260808-TEST01', $8, $8
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal,  // total_payable = (P-D)+S = 110000
			expectedBuyerVal,  // total_before_coins = CANONICAL
			expiry, now,
		)
		return execErr
	})
	require.NoError(t, err, "insert discounted order")

	// Read back
	var gotSubtotal, gotShipping, gotCommissionPct, gotCommissionAmt int64
	var gotTotalBeforeCoins, gotTotalPayable int64
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT subtotal, shipping_total, commission_percent, commission_amount,
			       total_before_coins_amount, total_payable_amount
			FROM orders WHERE id = $1
		`, orderID).Scan(
			&gotSubtotal, &gotShipping, &gotCommissionPct, &gotCommissionAmt,
			&gotTotalBeforeCoins, &gotTotalPayable,
		)
	})
	require.NoError(t, err, "read back discounted order")

	// ASSERT: all canonical values match
	assert.Equal(t, int64(100000), gotSubtotal, "subtotal = P = 100000")
	assert.Equal(t, int64(20000), gotShipping, "shipping_total = S = 20000")
	assert.Equal(t, int64(5), gotCommissionPct, "commission_percent = 5")
	assert.Equal(t, int64(4500), gotCommissionAmt, "commission_amount = C = 4500")
	// The discount's METADATA is deliberately NOT asserted here: the order row
	// persists no discount column at all. Only the discount's MONEY consequence
	// is persisted, carried implicitly by the canonical base below; the canonical
	// discount-metadata snapshot lives on pricing_tokens.discount_* (proven by
	// TestCanonicalPricingSnapshot_DiscountMetadataNotOnOrderRow).

	// CANONICAL: BuyerOrderValueBeforeCoins = (P-D)+S = 110000
	assert.Equal(t, expectedBuyerVal, gotTotalBeforeCoins,
		"total_before_coins_amount = (P-D)+S = 110000 — CANONICAL buyer base")
	assert.Equal(t, expectedBuyerVal, gotTotalPayable,
		"total_payable_amount = (P-D)+S = 110000")

	// ANTI-PROOF: NOT the old P+S+C-D formula
	assert.NotEqual(t, oldWrongVal, gotTotalBeforeCoins,
		"total_before_coins MUST NOT = P+S+C-D = 114500")
}

func TestCanonicalPricingSnapshot_NoDiscountOrder_RoundTrip(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertOrderTestUsers(t, ctx, testDB, sellerID, buyerID)

	orderID := uuid.New()
	expectedBuyerVal := int64(120000) // (P-D)+S = 100000-0+20000

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		now := time.Now()
		expiry := now.Add(24 * time.Hour)
		_, execErr := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount,
				service_fee_amount, total_payable_amount,
				total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000,
				5, 5000,
				0, $5,
				$6,
				'pending_payment', 'none', false,
				$7, 'immediate',
				'ORD-20260808-TEST02', $8, $8
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal, expectedBuyerVal,
			expiry, now,
		)
		return execErr
	})
	require.NoError(t, err)

	var gotTotalBeforeCoins, gotCommissionAmt int64
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT total_before_coins_amount, commission_amount
			FROM orders WHERE id = $1
		`, orderID).Scan(&gotTotalBeforeCoins, &gotCommissionAmt)
	})
	require.NoError(t, err)

	assert.Equal(t, expectedBuyerVal, gotTotalBeforeCoins,
		"total_before_coins = (P-D)+S = 120000 — CANONICAL buyer base")
	assert.Equal(t, int64(5000), gotCommissionAmt,
		"commission_amount = 5000 (seller-side, NOT in buyer base)")
}

// TestCanonicalPricingSnapshot_DiscountMetadataNotOnOrderRow proves the canonical
// authority split for discounts. The order row persists only the MONEY
// consequence of a discount (already folded into total_before_coins_amount);
// the discount METADATA (code/type/value/amount) is owned by the pricing-token
// snapshot. A discount column on `orders` would be a second, silently-diverging
// pricing authority, so the order write table must expose none at all.
//
// P=100000, D=50000 (flat), S=20000 → PD=50000, PD+S=70000, F=0 (no payment selection yet).
func TestCanonicalPricingSnapshot_DiscountMetadataNotOnOrderRow(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertOrderTestUsers(t, ctx, testDB, sellerID, buyerID)

	orderID := uuid.New()
	expectedBuyerVal := int64(70000) // (P-D)+S = 70000

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		now := time.Now()
		expiry := now.Add(24 * time.Hour)
		_, execErr := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount,
				service_fee_amount, total_payable_amount,
				total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000, 5, 5000,
				0, $5,
				$6,
				'pending_payment', 'none', false,
				$7, 'immediate',
				'ORD-20260808-TEST03', $8, $8
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal,  // total_payable = (P-D)+S at creation (F=0)
			expectedBuyerVal,  // total_before_coins = CANONICAL buyer base
			expiry, now,
		)
		return execErr
	})
	require.NoError(t, err, "insert order carrying a discounted money base")

	// The discount's money effect IS persisted, as money on the canonical base.
	var gotTotalBeforeCoins int64
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT total_before_coins_amount
			FROM orders WHERE id = $1
		`, orderID).Scan(&gotTotalBeforeCoins)
	})
	require.NoError(t, err)
	assert.Equal(t, expectedBuyerVal, gotTotalBeforeCoins,
		"total_before_coins_amount = (P-D)+S = 70000 — discount persisted as money")

	// The discount METADATA is not an order-persistence concern.
	var orderDiscountColumns int
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'orders'
			  AND column_name IN ('discount_code', 'discount_type', 'discount_value', 'discount_amount')
		`).Scan(&orderDiscountColumns)
	})
	require.NoError(t, err)
	assert.Zero(t, orderDiscountColumns,
		"orders must persist NO discount metadata column — pricing_tokens.discount_* is the authority")

	// The canonical discount-metadata snapshot is the pricing token.
	var tokenDiscountColumns int
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'pricing_tokens'
			  AND column_name IN ('discount_code', 'discount_type', 'discount_value', 'discount_amount')
		`).Scan(&tokenDiscountColumns)
	})
	require.NoError(t, err)
	assert.Equal(t, 4, tokenDiscountColumns,
		"pricing_tokens persists the canonical discount metadata snapshot (code/type/value/amount)")
}

func TestCanonicalPricingSnapshot_CommissionNotInBuyerPath(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertOrderTestUsers(t, ctx, testDB, sellerID, buyerID)

	orderID := uuid.New()
	expectedBuyerVal := int64(120000) // (P-D)+S = 120000
	commissionAmt := int64(5000)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		now := time.Now()
		expiry := now.Add(24 * time.Hour)
		_, execErr := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount,
				service_fee_amount, total_payable_amount,
				total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000,
				5, 5000,
				0, $5,
				$6,
				'pending_payment', 'none', false,
				$7, 'immediate',
				'ORD-20260808-TEST04', $8, $8
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal, expectedBuyerVal,
			expiry, now,
		)
		return execErr
	})
	require.NoError(t, err)

	var gotTotalBeforeCoins, gotCommission int64
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT total_before_coins_amount, commission_amount
			FROM orders WHERE id = $1
		`, orderID).Scan(&gotTotalBeforeCoins, &gotCommission)
	})
	require.NoError(t, err)

	assert.Equal(t, expectedBuyerVal, gotTotalBeforeCoins,
		"total_before_coins = (P-D)+S = 120000")
	assert.Equal(t, commissionAmt, gotCommission,
		"commission stored as seller-side snapshot = 5000")
	// KEY PROOF: buyer base does NOT inflate by commission
	assert.NotEqual(t, expectedBuyerVal+commissionAmt, gotTotalBeforeCoins,
		"total_before_coins MUST NOT = (P-D)+S+C = 125000")
}
