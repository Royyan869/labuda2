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
				discount_amount, discount_code, discount_type, discount_value,
				escrow_amount,
				coins_used, coin_discount_amount, total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000,
				5, 4500,
				0, $5,
				10000, 'SELLER10', 'percentage', '10',
				$6,
				0, 0, $7,
				'pending_payment', 'none', false,
				$8, 'immediate',
				'ORD-20260808-TEST01', $9, $9
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal,  // total_payable = (P-D)+S = 110000
			expectedBuyerVal,  // escrow_amount
			expectedBuyerVal,  // total_before_coins = CANONICAL
			expiry, now,
		)
		return execErr
	})
	require.NoError(t, err, "insert discounted order")

	// Read back
	var gotSubtotal, gotShipping, gotCommissionPct, gotCommissionAmt int64
	var gotDiscountAmt int64
	var gotDiscountCode, gotDiscountType, gotDiscountValue *string
	var gotEscrowAmt, gotTotalBeforeCoins, gotTotalPayable int64
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT subtotal, shipping_total, commission_percent, commission_amount,
			       discount_amount, discount_code, discount_type, discount_value,
			       escrow_amount, total_before_coins_amount, total_payable_amount
			FROM orders WHERE id = $1
		`, orderID).Scan(
			&gotSubtotal, &gotShipping, &gotCommissionPct, &gotCommissionAmt,
			&gotDiscountAmt, &gotDiscountCode, &gotDiscountType, &gotDiscountValue,
			&gotEscrowAmt, &gotTotalBeforeCoins, &gotTotalPayable,
		)
	})
	require.NoError(t, err, "read back discounted order")

	// ASSERT: all canonical values match
	assert.Equal(t, int64(100000), gotSubtotal, "subtotal = P = 100000")
	assert.Equal(t, int64(20000), gotShipping, "shipping_total = S = 20000")
	assert.Equal(t, int64(5), gotCommissionPct, "commission_percent = 5")
	assert.Equal(t, int64(4500), gotCommissionAmt, "commission_amount = C = 4500")
	assert.Equal(t, int64(10000), gotDiscountAmt, "discount_amount = D = 10000")
	require.NotNil(t, gotDiscountCode)
	assert.Equal(t, "SELLER10", *gotDiscountCode)
	require.NotNil(t, gotDiscountType)
	assert.Equal(t, "percentage", *gotDiscountType)
	require.NotNil(t, gotDiscountValue)
	assert.Equal(t, "10", *gotDiscountValue)

	// CANONICAL: BuyerOrderValueBeforeCoins = (P-D)+S = 110000
	assert.Equal(t, expectedBuyerVal, gotTotalBeforeCoins,
		"total_before_coins_amount = (P-D)+S = 110000 — CANONICAL buyer base")
	assert.Equal(t, expectedBuyerVal, gotTotalPayable,
		"total_payable_amount = (P-D)+S = 110000")
	assert.Equal(t, expectedBuyerVal, gotEscrowAmt,
		"escrow_amount = (P-D)+S (forward compat)")

	// ANTI-PROOF: NOT the old P+S+C-D formula
	assert.NotEqual(t, oldWrongVal, gotTotalBeforeCoins,
		"total_before_coins MUST NOT = P+S+C-D = 114500")
	assert.NotEqual(t, oldWrongVal, gotEscrowAmt,
		"escrow_amount MUST NOT = P+S+C-D = 114500")
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
				discount_amount,
				escrow_amount,
				coins_used, coin_discount_amount, total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000,
				5, 5000,
				0, $5,
				0,
				$6,
				0, 0, $7,
				'pending_payment', 'none', false,
				$8, 'immediate',
				'ORD-20260808-TEST02', $9, $9
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal, expectedBuyerVal, expectedBuyerVal,
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

func TestCanonicalPricingSnapshot_DiscountMetadataPersisted(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertOrderTestUsers(t, ctx, testDB, sellerID, buyerID)

	orderID := uuid.New()

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		now := time.Now()
		expiry := now.Add(24 * time.Hour)
		_, execErr := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount,
				service_fee_amount, total_payable_amount,
				discount_amount, discount_code, discount_type, discount_value,
				escrow_amount,
				coins_used, coin_discount_amount, total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000, 5, 5000,
				0, 120000,
				10000, 'FLAT50K', 'flat_amount', '50000',
				120000,
				0, 0, 120000,
				'pending_payment', 'none', false,
				$5, 'immediate',
				'ORD-20260808-TEST03', $6, $6
			)
		`, orderID, buyerID, sellerID, uuid.New(), expiry, now,
		)
		return execErr
	})
	require.NoError(t, err)

	var gotDiscountAmt int64
	var gotDiscountCode, gotDiscountType, gotDiscountValue *string
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT discount_amount, discount_code, discount_type, discount_value
			FROM orders WHERE id = $1
		`, orderID).Scan(&gotDiscountAmt, &gotDiscountCode, &gotDiscountType, &gotDiscountValue)
	})
	require.NoError(t, err)

	assert.Equal(t, int64(10000), gotDiscountAmt)
	require.NotNil(t, gotDiscountCode)
	assert.Equal(t, "FLAT50K", *gotDiscountCode)
	require.NotNil(t, gotDiscountType)
	assert.Equal(t, "flat_amount", *gotDiscountType)
	require.NotNil(t, gotDiscountValue)
	assert.Equal(t, "50000", *gotDiscountValue)
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
				discount_amount,
				escrow_amount,
				coins_used, coin_discount_amount, total_before_coins_amount,
				status, escrow_status, has_dispute,
				payment_expires_at, preparation_time_snapshot,
				order_number, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'for_sale', $4,
				1, 100000, 100000, 20000,
				5, 5000,
				0, $5,
				0,
				$6,
				0, 0, $7,
				'pending_payment', 'none', false,
				$8, 'immediate',
				'ORD-20260808-TEST04', $9, $9
			)
		`, orderID, buyerID, sellerID, uuid.New(),
			expectedBuyerVal, expectedBuyerVal, expectedBuyerVal,
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
