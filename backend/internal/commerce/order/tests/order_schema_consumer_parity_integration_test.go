//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	orderhttp "github.com/labuda/backend/internal/commerce/order/delivery/http"
	"github.com/labuda/backend/internal/commerce/order/entity"
	orderinfra "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// ORDER-SCOPE-B — SCHEMA ↔ CONSUMER PARITY PROOF
// ============================================================================
//
// These tests run against a freshly migrated disposable database (testdb runs
// the migration chain, including 000096 which purges the dead orders columns).
// They prove the post-convergence contract:
//
//  1. GET /api/v1/admin/orders/:id reads ONLY canonical sources:
//       dispute status   → disputes row
//       shipping address → orders.address_snapshot
//       refunded amount  → refunds domain (gateway-succeeded refunds)
//  2. The order summary projection derives refunded_amount from the refunds
//     domain, not from a purged orders column.
//  3. Shipping-quote order lookups read the canonical address snapshot and
//     execute cleanly against the order_status_enum column.
//  4. The purged columns are really gone from the schema.

const parityAddressSnapshot = `{
  "recipient_name": "Budi Santoso",
  "phone": "081234567890",
  "province_id": "11",
  "province_name": "DKI Jakarta",
  "city_id": "1101",
  "city_name": "Jakarta Selatan",
  "district_id": "1101010",
  "district_name": "Tebet",
  "village_id": "1101010001",
  "village_name": "Tebet Barat",
  "street_address": "Jl. Mawar No. 1",
  "postal_code": "12810"
}`

// ---------- fixtures ----------

func parityUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB, label string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, id, "parity-"+label+"-"+id.String()[:8], "parity-"+label+"-"+id.String()[:8]+"@example.test")
	require.NoError(t, err)
	return id
}

func parityOrder(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	buyerID, sellerID uuid.UUID,
	status string,
	hasDispute bool,
	quoteID *uuid.UUID,
	orderNumber string,
	base int64,
) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id, source_type, source_id,
			quantity, unit_price, subtotal, shipping_total,
			commission_percent, commission_amount,
			status, escrow_status, has_dispute,
			shipping_option_name, shipping_transport_type, shipping_source, shipping_quote_id,
			address_snapshot, order_number,
			total_before_coins_amount, service_fee_amount, total_payable_amount,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, 'for_sale', $4,
			1, $5, $5, 10000,
			5, 5000,
			$6::order_status_enum, 'holding', $7,
			'JNE Reguler', 'truck', 'for_sale', $8,
			$9::jsonb, $10,
			$11, 0, $11,
			NOW(), NOW()
		)
	`, id, buyerID, sellerID, uuid.New(), base-10000, status, hasDispute, quoteID, parityAddressSnapshot, orderNumber, base)
	require.NoError(t, err)
	return id
}

// parityRefund inserts a refund row. product/shipping are the canonical
// gateway-succeeded split amounts; finalAmount is the decision amount.
func parityRefund(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	orderID, buyerID, sellerID uuid.UUID,
	gatewayStatus string,
	product, shipping, finalAmount int64,
) {
	t.Helper()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO refunds (
			id, order_id, buyer_id, seller_id, reason, status, requested_amount,
			refunded_product_amount, refunded_shipping_amount, final_refund_amount,
			gateway_status, gateway_attempts, opened_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, 'item_not_received', 'seller_approved', $5,
			$6, $7, $8,
			$9, 0, NOW(), NOW(), NOW()
		)
	`, uuid.New(), orderID, buyerID, sellerID, product+shipping, product, shipping, finalAmount, gatewayStatus)
	require.NoError(t, err)
}

// parityPricingToken inserts the minimal pricing-token row an order links back
// to. orders.pricing_token_id is a FK to pricing_tokens(token) — the token's
// canonical identifier — not to its surrogate id. escrow_amount is the token's
// canonical buyer-funded base (PD + S); order_value_for_coins is PD.
func parityPricingToken(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	buyerID uuid.UUID,
	base int64,
	discountedProduct int64,
) uuid.UUID {
	t.Helper()
	token := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO pricing_tokens (
			id, token, user_id, quantity, unit_price, subtotal, shipping_total,
			commission_percent, commission_amount, escrow_amount,
			shipping_option_id, shipping_option_name, shipping_transport_type,
			address_id, address_snapshot, max_coins_allowed, order_value_for_coins, expires_at
		) VALUES (
			$1, $1, $2, 1, $3, $3, 10000,
			5, 5000, $4,
			$5, 'JNE Reguler', 'truck',
			$6, $7::jsonb, 0, $8, NOW() + INTERVAL '1 hour'
		)
	`, token, buyerID, base-10000, base, uuid.New(), uuid.New(), parityAddressSnapshot, discountedProduct)
	require.NoError(t, err)
	return token
}

// ---------- 1. admin order detail ----------

// TestAdminOrderDetailReadsCanonicalSourcesOnly proves the admin detail endpoint
// resolves dispute status, shipping address and refunded amount from canonical
// sources (the legacy orders columns dispute_status / shipping_address_id /
// refunded_amount do not exist or were purged).
func TestAdminOrderDetailReadsCanonicalSourcesOnly(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	buyerID := parityUser(t, ctx, tdb, "buyer")
	sellerID := parityUser(t, ctx, tdb, "seller")
	adminID := parityUser(t, ctx, tdb, "admin")

	// Order WITH a dispute and one settled + one non-settled refund.
	disputedOrderID := parityOrder(t, ctx, tdb, buyerID, sellerID, "dispute_open", true, nil, "PARITY-DISPUTED", 95000)
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO disputes (id, order_id, buyer_id, seller_id, reason, status, opened_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_damaged', 'under_review', NOW(), NOW(), NOW())
	`, uuid.New(), disputedOrderID, buyerID, sellerID)
	require.NoError(t, err)
	// Canonical: 85000 product + 10000 shipping, gateway-succeeded.
	parityRefund(t, ctx, tdb, disputedOrderID, buyerID, sellerID, "succeeded", 85000, 10000, 95000)
	// NOT canonical: not settled at the gateway, must be excluded from the total.
	parityRefund(t, ctx, tdb, disputedOrderID, buyerID, sellerID, "unsubmitted", 12345, 0, 12345)

	// Order WITHOUT a dispute and WITHOUT any refund.
	cleanOrderID := parityOrder(t, ctx, tdb, buyerID, sellerID, "paid", false, nil, "PARITY-CLEAN", 95000)

	devLog, _ := zap.NewDevelopment()
	handler := orderhttp.NewAdminOrderHandler(nil, db.NewFromPool(tdb.Pool()), devLog)

	type adminAddress struct {
		ID            *string `json:"id"`
		RecipientName string  `json:"recipient_name"`
		Phone         string  `json:"phone"`
		Province      string  `json:"province"`
		City          string  `json:"city"`
		Address       string  `json:"address"`
		PostalCode    string  `json:"postal_code"`
	}
	type adminDetail struct {
		DisputeStatus   *string       `json:"dispute_status"`
		RefundedAmount  int64         `json:"refunded_amount"`
		HasDispute      bool          `json:"has_dispute"`
		ShippingAddress *adminAddress `json:"shipping_address"`
	}
	callDetail := func(orderID uuid.UUID) adminDetail {
		t.Helper()
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/"+orderID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: orderID.String()}}
		c.Set("user_id", adminID)

		handler.GetOrderDetail(c)

		require.Equal(t, http.StatusOK, rec.Code, "admin detail must succeed, body=%s", rec.Body.String())
		var env struct {
			Success bool        `json:"success"`
			Data    adminDetail `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		require.True(t, env.Success, "body=%s", rec.Body.String())
		return env.Data
	}

	// --- order with dispute ---
	disputed := callDetail(disputedOrderID)
	require.NotNil(t, disputed.DisputeStatus, "dispute_status must come from the canonical disputes row")
	require.Equal(t, "under_review", *disputed.DisputeStatus)
	require.True(t, disputed.HasDispute)
	require.Equal(t, int64(95000), disputed.RefundedAmount,
		"refunded_amount must be the sum of gateway-succeeded refunds only")
	require.NotNil(t, disputed.ShippingAddress, "shipping_address must come from orders.address_snapshot")
	require.Equal(t, "Budi Santoso", disputed.ShippingAddress.RecipientName)
	require.Equal(t, "081234567890", disputed.ShippingAddress.Phone)
	require.Equal(t, "DKI Jakarta", disputed.ShippingAddress.Province)
	require.Equal(t, "Jakarta Selatan", disputed.ShippingAddress.City)
	require.Equal(t, "Jl. Mawar No. 1", disputed.ShippingAddress.Address)
	require.Equal(t, "12810", disputed.ShippingAddress.PostalCode)
	require.Nil(t, disputed.ShippingAddress.ID,
		"the immutable snapshot carries no address identity; no fake address id may be exposed")

	// --- order without dispute ---
	clean := callDetail(cleanOrderID)
	require.Nil(t, clean.DisputeStatus, "no dispute ⇒ no dispute_status")
	require.False(t, clean.HasDispute)
	require.Equal(t, int64(0), clean.RefundedAmount, "no succeeded refund ⇒ zero refunded amount")
	require.NotNil(t, clean.ShippingAddress, "address snapshot must be present without any dispute")
	require.Equal(t, "Budi Santoso", clean.ShippingAddress.RecipientName)
}

// ---------- 2. projection ----------

// TestOrderSummaryProjectionRefundedAmountIsCanonical proves the read-model
// projection derives refunded_amount from the refunds domain.
func TestOrderSummaryProjectionRefundedAmountIsCanonical(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	buyerID := parityUser(t, ctx, tdb, "proj-buyer")
	sellerID := parityUser(t, ctx, tdb, "proj-seller")

	orderID := parityOrder(t, ctx, tdb, buyerID, sellerID, "partially_refunded", false, nil, "PARITY-PROJ", 95000)
	parityRefund(t, ctx, tdb, orderID, buyerID, sellerID, "succeeded", 85000, 10000, 95000)
	parityRefund(t, ctx, tdb, orderID, buyerID, sellerID, "unsubmitted", 5000, 0, 5000)

	w := worker.NewProjectionWorker(db.NewFromPool(tdb.Pool()), zap.NewNop(), worker.ProjectionWorkerConfig{})
	require.NoError(t, w.RebuildAll(ctx))

	var refunded int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT refunded_amount FROM order_summaries WHERE id = $1`, orderID).Scan(&refunded))
	require.Equal(t, int64(95000), refunded,
		"order_summaries.refunded_amount must equal the gateway-succeeded refunds total")

	var base int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT total_before_coins_amount FROM order_summaries WHERE id = $1`, orderID).Scan(&base))
	require.Equal(t, int64(95000), base, "000095 column rename must still project the canonical base")
}

// ---------- 3. shipping-quote lookups ----------

// TestShippingQuoteOrderLookupsUseCanonicalAddressSnapshot proves the quote
// lookups read orders.address_snapshot (orders.shipping_destination was never
// written) and execute against the order_status_enum column.
func TestShippingQuoteOrderLookupsUseCanonicalAddressSnapshot(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	buyerID := parityUser(t, ctx, tdb, "quote-buyer")
	sellerID := parityUser(t, ctx, tdb, "quote-seller")

	// chat_rooms enforces the canonical participant ordering (participant_a < participant_b).
	roomA, roomB := buyerID, sellerID
	if roomB.String() < roomA.String() {
		roomA, roomB = roomB, roomA
	}

	chatID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at)
		VALUES ($1, 'direct', $2, $3, NOW(), NOW(), NOW())
	`, chatID, roomA, roomB)
	require.NoError(t, err)

	quoteID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO shipping_quotes (
			id, chat_id, product_id, source_type, source_id, seller_id, buyer_id,
			cost, status, created_at, expires_at, reactivation_count, max_reuse
		) VALUES (
			$1, $2, $3, 'for_sale', $4, $5, $6,
			25000, 'ACTIVE', NOW(), $7, 0, 2
		)
	`, quoteID, chatID, uuid.New(), uuid.New(), sellerID, buyerID, time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	orderID := parityOrder(t, ctx, tdb, buyerID, sellerID, "paid", false, &quoteID, "PARITY-QUOTE", 95000)

	repo := orderinfra.NewOrderRepository()

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := repo.GetByShippingQuoteID(ctx, tx, quoteID)
		require.NoError(t, err, "quote lookup must execute against the canonical schema")
		require.NotNil(t, order)
		require.Equal(t, orderID, order.ID)
		require.NotNil(t, order.AddressSnapshot,
			"the buyer's address must come from orders.address_snapshot, not the never-written shipping_destination")
		require.Equal(t, "Budi Santoso", order.AddressSnapshot.RecipientName)
		require.Equal(t, "Jl. Mawar No. 1", order.AddressSnapshot.StreetAddress)
		require.Equal(t, "12810", order.AddressSnapshot.PostalCode)

		// enum vs text[] parity: these must not raise
		// "operator does not exist: order_status_enum = text".
		count, err := repo.CountValidOrdersByShippingQuoteID(ctx, tx, quoteID)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		blocking, err := repo.GetBlockingOrderByShippingQuoteID(ctx, tx, quoteID)
		require.NoError(t, err)
		require.NotNil(t, blocking)
		require.Equal(t, orderID, blocking.ID)

		// An unknown quote id must return an empty result, not a type error.
		unknown := uuid.New()
		count, err = repo.CountValidOrdersByShippingQuoteID(ctx, tx, unknown)
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
		blocking, err = repo.GetBlockingOrderByShippingQuoteID(ctx, tx, unknown)
		require.NoError(t, err)
		require.Nil(t, blocking)
		return nil
	})
	require.NoError(t, err)
}

// ---------- 4. purged schema ----------

// TestPurgedOrderColumnsAreAbsentFromSchema proves 000096 removed the dead
// columns (and that no query above can silently depend on them again).
func TestPurgedOrderColumnsAreAbsentFromSchema(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	rows, err := tdb.Pool().Query(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_name = 'orders'
	`)
	require.NoError(t, err)
	defer rows.Close()

	present := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		present[name] = true
	}
	require.NoError(t, rows.Err())

	for _, purged := range []string{
		"shipping_destination", "refunded_amount", "destination_address",
		"discount_code", "discount_type", "discount_value",
		// purged earlier by 000093 — must not come back either
		"escrow_amount", "discount_amount", "coins_used", "coin_discount_amount",
		// never existed in any migration
		"dispute_status", "shipping_address_id", "order_type",
	} {
		require.False(t, present[purged], "orders.%s must not exist in the canonical schema", purged)
	}
	require.True(t, present["address_snapshot"], "orders.address_snapshot is the canonical address source")
	require.True(t, present["total_before_coins_amount"], "orders.total_before_coins_amount is the canonical money base")
}

// ---------- 5. every orders write-model reader executes ----------

// TestOrdersWriteModelReadersExecuteAgainstCanonicalSchema locks the reader half
// of the parity contract. Every reader that scans an orders row reads
// completed_at/created_at/updated_at (all timestamptz), so a SELECT/Scan arity
// mismatch or a non-canonical timestamp representation (epoch int64, which pgx
// rejects with "cannot scan timestamptz in binary format into *int64") fails
// here instead of in production.
//
// It also locks the canonical money/timeline hydration for every reader whose
// result reaches an API surface or a money decision. A reader that silently
// omits total_before_coins_amount would make Order.HasCanonicalMoneyBase()
// false and PostOrderCreateResponse.total_before_coins_amount zero; a reader
// that omits payment_expires_at would make Order.IsExpired() true. Both are
// wire/behaviour defects, not stylistic ones.
func TestOrdersWriteModelReadersExecuteAgainstCanonicalSchema(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	buyerID := parityUser(t, ctx, tdb, "reader-buyer")
	sellerID := parityUser(t, ctx, tdb, "reader-seller")

	const orderNumber = "PARITY-READERS-1"
	// Seeded money: base = PD + S = 95000, shipping_total = 10000 ⇒ PD = 85000.
	const wantBase = int64(95000)
	const wantDiscountedProduct = int64(85000)

	orderID := parityOrder(t, ctx, tdb, buyerID, sellerID, "paid", false, nil, orderNumber, wantBase)

	// Give the order the identities the remaining readers resolve by.
	pricingTokenID := parityPricingToken(t, ctx, tdb, buyerID, wantBase, wantDiscountedProduct)
	const idempotencyKey = "parity-readers-key"
	_, err := tdb.Pool().Exec(ctx, `
		UPDATE orders
		SET pricing_token_id = $2, idempotency_key = $3, payment_expires_at = NOW() + INTERVAL '1 hour'
		WHERE id = $1
	`, orderID, pricingTokenID, idempotencyKey)
	require.NoError(t, err)

	repo := orderinfra.NewOrderRepository()

	assertReader := func(label string, order *entity.Order, err error) {
		t.Helper()
		require.NoError(t, err, "%s must execute against the canonical schema", label)
		require.NotNil(t, order, "%s must return the seeded order", label)
		require.Equal(t, orderID, order.ID, label)
		// Proof the timestamptz columns were scanned as time.Time, not coerced.
		require.False(t, order.CreatedAt.IsZero(), "%s must scan created_at", label)
		require.False(t, order.UpdatedAt.IsZero(), "%s must scan updated_at", label)
		require.False(t, order.CreatedAt.After(time.Now().Add(time.Minute)),
			"%s created_at must be a real timestamp, not an epoch integer", label)
	}

	assertCanonicalMoneyBase := func(label string, order *entity.Order, err error) {
		t.Helper()
		assertReader(label, order, err)
		require.Equal(t, wantBase, order.TotalBeforeCoinsAmount.Int64(),
			"%s must hydrate total_before_coins_amount (PD + S)", label)
		require.Equal(t, wantDiscountedProduct, order.DiscountedProductAmount().Int64(),
			"%s must derive PD = total_before_coins_amount - shipping_total", label)
		require.True(t, order.HasCanonicalMoneyBase(),
			"%s must satisfy the canonical money-base guard", label)
		require.False(t, order.PaymentExpiresAt.IsZero(),
			"%s must hydrate payment_expires_at (single source of truth for expiry)", label)
	}

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		byID, err := repo.GetByID(ctx, tx, orderID)
		assertCanonicalMoneyBase("GetByID", byID, err)

		locked, err := repo.GetForUpdate(ctx, tx, orderID)
		assertCanonicalMoneyBase("GetForUpdate", locked, err)

		// POST /orders duplicate recovery returns this order as the create
		// response (OrderToCreateResponse), and mobile reads
		// total_before_coins_amount from it — it MUST be the canonical base.
		byToken, err := repo.GetByPricingTokenID(ctx, tx, pricingTokenID)
		assertCanonicalMoneyBase("GetByPricingTokenID", byToken, err)

		byKey, err := repo.GetByIdempotencyKey(ctx, tx, buyerID, idempotencyKey)
		assertCanonicalMoneyBase("GetByIdempotencyKey", byKey, err)

		// A miss must be an explicit not-found, never a silently zero-valued order.
		missing, err := repo.GetByID(ctx, tx, uuid.New())
		require.Error(t, err, "an unknown order id must be reported, not silently zero-valued")
		require.Contains(t, err.Error(), "order not found")
		require.Nil(t, missing)

		noToken, err := repo.GetByPricingTokenID(ctx, tx, uuid.New())
		require.NoError(t, err, "an unknown pricing token must be an idempotent nil")
		require.Nil(t, noToken)
		return nil
	}))
}
