//go:build integration

package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderinfra "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// CANONICAL ORDER TESTS — Integration/DB Tests
// ============================================================================
//
// This test suite protects the core business truths of the order system
// that require database interaction:
//
// 1. First-Come-First-Served (FCFS) inventory allocation
// 2. No overselling of limited stock
// 3. Idempotent order creation (no duplicate orders on retry)
//
// These are INTEGRATION tests that verify:
// - Database transaction isolation
// - Concurrent access patterns
// - Repository behavior with real data
//
// For pure domain logic tests (state machine, validations, calculations),
// see: ../../entity/order_domain_test.go
//
// REQUIREMENTS:
// - PostgreSQL database running
// - Test database schema applied
// - Environment configured for test DB access

// ============================================================================
// TEST: Concurrency & Stock Protection
// ============================================================================

// TestDoubleCheckoutProtection verifies that when two buyers try to purchase
// the same item simultaneously, only one succeeds (First-Come-First-Served).
//
// BUSINESS TRUTH PROTECTED:
// - Inventory is first-come-first-served
// - ForSale quantity cannot be oversold
// - Only one order is created for a single available item
//
// CRITICAL FOR: Preventing customer disputes from overselling
func TestDoubleCheckoutProtection(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	orderRepo := orderinfra.NewOrderRepository()
	forSaleRepo := forsaleRepo.NewForSaleRepository()

	// Setup: Create a forSale with exactly 1 item available
	sellerID := uuid.New()
	buyer1ID := uuid.New()
	buyer2ID := uuid.New()

	insertOrderTestUsers(t, ctx, testDB, sellerID, buyer1ID, buyer2ID)

	var forSaleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Kohaku Koi",
			"Beautiful Kohaku",
			[]byte(`["https://picsum.photos/seed/koi1/800/600"]`),
			"Kohaku",
			intPtr(30),
			intPtr(12),
			strPtr("female"),
			nil,                // breeder
			nil,                // bloodline
			[]string{"global"}, // certificates
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(500000), // 500000 IDR
			1,                 // CRITICAL: Only 1 item available
			false,
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil, // farmAddressID
			forsaleEntity.PreparationTimeImmediate,
			nil, // preparationNote
		)
		if err != nil {
			return err
		}
		// Publish the forSale (draft -> active)
		if err := forSale.Publish(); err != nil {
			return err
		}
		if err := forSaleRepo.Create(ctx, tx, forSale); err != nil {
			return err
		}
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err, "failed to setup forSale")

	// Execute: Two buyers attempt checkout simultaneously
	type result struct {
		orderID uuid.UUID
		err     error
	}
	results := make(chan result, 2)

	// Simulate concurrent checkout attempts
	var wg sync.WaitGroup
	for i, buyerID := range []uuid.UUID{buyer1ID, buyer2ID} {
		wg.Add(1)
		go func(idx int, bid uuid.UUID) {
			defer wg.Done()
			err := testDB.WithTx(ctx, func(tx db.Tx) error {
				// Lock forSale for update (prevents race condition)
				forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
				if err != nil {
					return err
				}

				// Try to reduce quantity (will fail if out of stock)
				if err := forSale.ReduceQuantity(1); err != nil {
					return err // Expected to fail for one buyer
				}

				// Update forSale in DB
				if err := forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
					return err
				}

				// Create order (only succeeds if quantity was reduced)
				order := orderentity.NewOrderFromSource(
					bid,
					sellerID,
					orderentity.OrderSourceForSale,
					forSaleID,
					nil, // negotiationID
					1,
					forSale.PricePerUnit,
					forSale.PricePerUnit, // Subtotal = unitPrice (quantity=1)
					money.New(20000),     // Shipping
					5,
					money.New((forSale.PricePerUnit.Int64()*5)/100), // Commission amount: 5%
					money.New(3000), // Buyer service fee
					forSale.PricePerUnit.Add(money.New(20000)).Add(money.New((forSale.PricePerUnit.Int64()*5)/100)).Add(money.New(3000)),
					nil, // shippingSetupID: nil for test
					"JNE",
					"truck",
					nil,         // auctionSettlementType
					"immediate", // preparationTimeSnapshot
					nil,         // preparationNoteSnapshot
					nil,         // shippingSource
					nil,         // shippingQuoteID
					nil,         // shippingQuotePrice
					nil,         // pricingTokenID
					"instant",
					time.Now(),
				)
				order.IdempotencyKey = strPtr(fmt.Sprintf("checkout-%d", idx))

				if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
					return err
				}

				results <- result{orderID: order.ID}
				return nil
			})

			if err != nil {
				results <- result{err: err}
			}
		}(i, buyerID)
	}

	wg.Wait()
	close(results)

	// Verify: Exactly ONE order succeeded, ONE failed
	var successfulOrders int
	var failedOrders int
	var succeededOrderID uuid.UUID

	for r := range results {
		if r.err == nil {
			successfulOrders++
			succeededOrderID = r.orderID
			t.Logf("Order succeeded: %s", r.orderID)
		} else {
			failedOrders++
			t.Logf("Order failed (expected): %v", r.err)
		}
	}

	assert.Equal(t, 1, successfulOrders, "exactly one order should succeed")
	assert.Equal(t, 1, failedOrders, "exactly one order should fail")
	assert.NotEqual(t, uuid.Nil, succeededOrderID, "succeeded order should have valid ID")

	// Verify: Final forSale state
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetByID(ctx, tx, forSaleID)
		if err != nil {
			return err
		}
		assert.Equal(t, 0, forSale.QuantityAvailable, "forSale should have 0 quantity")
		assert.Equal(t, forsaleEntity.ForSaleStatusSold, forSale.Status, "forSale should be marked as sold")
		return nil
	})
	require.NoError(t, err)
}

// TestStockRaceCondition verifies that when multiple buyers compete for limited stock,
// the system never oversells beyond the available quantity.
//
// BUSINESS TRUTH PROTECTED:
// - Stock cannot go negative
// - Number of successful orders ≤ available quantity
// - Concurrent requests are properly serialized via FOR UPDATE lock
//
// CRITICAL FOR: Flash sales, popular items, high-traffic scenarios
func TestStockRaceCondition(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	orderRepo := orderinfra.NewOrderRepository()
	forSaleRepo := forsaleRepo.NewForSaleRepository()

	sellerID := uuid.New()
	initialStock := 5 // Limited stock

	// Setup: seller + buyer user fixtures (FK: for_sales.seller_id
	// and orders.buyer_id both reference users(id) — every buyerID used by
	// the concurrent goroutines below must exist before CreateOrderTx runs).
	numBuyers := 10 // 10 buyers competing for 5 items
	buyerIDs := make([]uuid.UUID, numBuyers)
	for i := range buyerIDs {
		buyerIDs[i] = uuid.New()
	}
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local")
		if err != nil {
			return err
		}
		for _, buyerID := range buyerIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
				buyerID, "fb-"+buyerID.String()[:8], buyerID.String()+"@test.local"); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err, "seller/buyer user fixtures failed")

	// Setup: Create a forSale with limited stock
	var forSaleID uuid.UUID
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Popular Koi",
			"High demand item",
			[]byte(`[]`),
			"Showa",
			intPtr(25),
			nil,                // ageMonths
			nil,                // gender
			nil,                // breeder
			nil,                // bloodline
			[]string{"global"}, // certificates
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(300000),
			initialStock, // Limited stock
			false,
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil, // farmAddressID
			forsaleEntity.PreparationTimeImmediate,
			nil, // preparationNote
		)
		if err != nil {
			return err
		}
		if err := forSale.Publish(); err != nil {
			return err
		}
		if err := forSaleRepo.Create(ctx, tx, forSale); err != nil {
			return err
		}
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err)

	// Execute: MORE buyers than available stock try to purchase
	var wg sync.WaitGroup
	successCount := make(chan int, numBuyers)

	for i := 0; i < numBuyers; i++ {
		wg.Add(1)
		go func(buyerIndex int) {
			defer wg.Done()
			err := testDB.WithTx(ctx, func(tx db.Tx) error {
				// Lock forSale with FOR UPDATE
				forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
				if err != nil {
					return err
				}

				// Check and reduce quantity atomically
				if forSale.QuantityAvailable < 1 {
					return &forsaleEntity.InsufficientQuantityError{
						Available: forSale.QuantityAvailable,
						Requested: 1,
					}
				}

				if err := forSale.ReduceQuantity(1); err != nil {
					return err
				}

				if err := forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
					return err
				}

				// Create order
				buyerID := buyerIDs[buyerIndex]
				order := orderentity.NewOrderFromSource(
					buyerID,
					sellerID,
					orderentity.OrderSourceForSale,
					forSaleID,
					nil, // negotiationID
					1,
					forSale.PricePerUnit,
					forSale.PricePerUnit, // Subtotal = unitPrice (quantity=1)
					money.New(15000),     // Shipping
					5,
					money.New((forSale.PricePerUnit.Int64()*5)/100), // Commission amount: 5%
					money.New(3000), // Buyer service fee
					forSale.PricePerUnit.Add(money.New(15000)).Add(money.New((forSale.PricePerUnit.Int64()*5)/100)).Add(money.New(3000)),
					nil, // shippingSetupID: nil for test
					"JNE",
					"truck",
					nil,         // auctionSettlementType
					"immediate", // preparationTimeSnapshot
					nil,         // preparationNoteSnapshot
					nil,         // shippingSource
					nil,         // shippingQuoteID
					nil,         // shippingQuotePrice
					nil,         // pricingTokenID
					"instant",
					time.Now(),
				)
				order.IdempotencyKey = strPtr(fmt.Sprintf("race-%d", buyerIndex))
				return orderRepo.CreateOrderTx(ctx, tx, order)
			})

			if err == nil {
				successCount <- 1
			}
		}(i)
	}

	wg.Wait()
	close(successCount)

	// Count successful orders
	successes := 0
	for range successCount {
		successes++
	}

	// Verify: No overselling occurred
	assert.LessOrEqual(t, successes, initialStock, "successful orders cannot exceed available stock")
	assert.Equal(t, 5, successes, "exactly %d orders should succeed with stock of %d", initialStock, initialStock)

	// Verify: Final forSale state
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetByID(ctx, tx, forSaleID)
		if err != nil {
			return err
		}
		assert.Equal(t, 0, forSale.QuantityAvailable, "all stock should be exhausted")
		assert.Equal(t, forsaleEntity.ForSaleStatusSold, forSale.Status, "forSale should be sold")
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// TEST: Idempotency & Retry Safety
// ============================================================================

// TestOrderCreationIdempotency verifies that retrying the same order request
// with an idempotency key does not create duplicate orders.
//
// BUSINESS TRUTH PROTECTED:
// - Network retries don't create duplicate orders
// - Same idempotency key + buyer = same order
// - Idempotency is scoped per-buyer
//
// CRITICAL FOR: Mobile apps with unreliable networks, payment retries
func TestOrderCreationIdempotency(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	orderRepo := orderinfra.NewOrderRepository()
	forSaleRepo := forsaleRepo.NewForSaleRepository()

	sellerID := uuid.New()
	buyerID := uuid.New()
	idempotencyKey := "retry-key-12345"

	// Product creation now enforces products.seller_id -> users(id), so the
	// canonical seller and buyer identities must exist before we create the
	// fixed-price sale fixture.
	insertOrderTestUsers(t, ctx, testDB, sellerID, buyerID)

	// Setup: Create a forSale
	var forSaleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Test Item",
			"Description",
			[]byte(`[]`),
			"Kohaku",
			nil,                // sizeCM
			nil,                // ageMonths
			nil,                // gender
			nil,                // breeder
			nil,                // bloodline
			[]string{"global"}, // certificates
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(100000),
			3,
			false,
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil, // farmAddressID
			forsaleEntity.PreparationTimeImmediate,
			nil, // preparationNote
		)
		if err != nil {
			return err
		}
		if err := forSale.Publish(); err != nil {
			return err
		}
		if err := forSaleRepo.Create(ctx, tx, forSale); err != nil {
			return err
		}
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err)

	var firstOrderID uuid.UUID

	// First attempt: Create order with idempotency key
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		if err != nil {
			return err
		}
		if err := forSale.ReduceQuantity(1); err != nil {
			return err
		}
		if err := forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
			return err
		}

		order := orderentity.NewOrderFromSource(
			buyerID,
			sellerID,
			orderentity.OrderSourceForSale,
			forSaleID,
			nil, // negotiationID
			1,
			forSale.PricePerUnit,
			forSale.PricePerUnit, // Subtotal = unitPrice (quantity=1)
			money.New(10000),     // Shipping
			5,
			money.New((forSale.PricePerUnit.Int64()*5)/100), // Commission amount: 5%
			money.New(3000), // Buyer service fee
			forSale.PricePerUnit.Add(money.New(10000)).Add(money.New((forSale.PricePerUnit.Int64()*5)/100)).Add(money.New(3000)),
			nil, // shippingSetupID: nil for test
			"JNE",
			"truck",
			nil,         // auctionSettlementType
			"immediate", // preparationTimeSnapshot
			nil,         // preparationNoteSnapshot
			nil,         // shippingSource
			nil,         // shippingQuoteID
			nil,         // shippingQuotePrice
			nil,         // pricingTokenID
			"instant",
			time.Now(),
		)
		order.IdempotencyKey = &idempotencyKey
		firstOrderID = order.ID
		return orderRepo.CreateOrderTx(ctx, tx, order)
	})
	require.NoError(t, err, "first order creation should succeed")

	// Second attempt: Retry with SAME idempotency key (simulates network retry)
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		// Check for existing order with same idempotency key
		existingOrder, err := orderRepo.GetByIdempotencyKey(ctx, tx, buyerID, idempotencyKey)
		if err != nil {
			return err
		}

		// Idempotency check should find the first order
		require.NotNil(t, existingOrder, "existing order should be found")
		assert.Equal(t, firstOrderID, existingOrder.ID, "should return same order ID")
		assert.Equal(t, buyerID, existingOrder.BuyerID, "buyer ID should match")
		assert.Equal(t, idempotencyKey, *existingOrder.IdempotencyKey, "idempotency key should match")

		return nil
	})
	require.NoError(t, err, "idempotency check should succeed")

	// Verify: Only ONE order exists
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetByID(ctx, tx, forSaleID)
		if err != nil {
			return err
		}
		assert.Equal(t, 2, forSale.QuantityAvailable, "only 1 item should be deducted (not 2)")
		return nil
	})
	require.NoError(t, err)
}

// TestDifferentBuyersSameIdempotencyKey verifies that idempotency is scoped
// per-buyer — different buyers can use the same key without collision.
//
// BUSINESS TRUTH PROTECTED:
// - Idempotency key is scoped to buyer
// - Different buyers don't interfere with each other's retries
//
// CRITICAL FOR: Preventing cross-buyer idempotency conflicts
func TestDifferentBuyersSameIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	orderRepo := orderinfra.NewOrderRepository()
	forSaleRepo := forsaleRepo.NewForSaleRepository()

	sellerID := uuid.New()
	buyer1ID := uuid.New()
	buyer2ID := uuid.New()
	sameKey := "shared-key-999"

	insertOrderTestUsers(t, ctx, testDB, sellerID, buyer1ID, buyer2ID)

	// Setup: Create forSale with 2 items
	var forSaleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forsaleEntity.NewForSale(
			sellerID,
			"Item",
			"Desc",
			[]byte(`[]`),
			"Kohaku",
			nil,                // sizeCM
			nil,                // ageMonths
			nil,                // gender
			nil,                // breeder
			nil,                // bloodline
			[]string{"global"}, // certificates
			forsaleEntity.ForSaleTypeFixedPrice,
			money.New(100000),
			2,
			false,
			forsaleEntity.ForSaleVisibilityPublic,
			forsaleEntity.ForSaleOriginDirectCreate,
			nil, // farmAddressID
			forsaleEntity.PreparationTimeImmediate,
			nil, // preparationNote
		)
		if err != nil {
			return err
		}
		if err := forSale.Publish(); err != nil {
			return err
		}
		if err := forSaleRepo.Create(ctx, tx, forSale); err != nil {
			return err
		}
		forSaleID = forSale.ID
		return nil
	})
	require.NoError(t, err)

	// Buyer 1 creates order with the key
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		if err != nil {
			return err
		}
		if err := forSale.ReduceQuantity(1); err != nil {
			return err
		}
		if err := forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
			return err
		}

		order := orderentity.NewOrderFromSource(
			buyer1ID,
			sellerID,
			orderentity.OrderSourceForSale,
			forSaleID,
			nil,
			1,
			forSale.PricePerUnit,
			forSale.PricePerUnit, // Subtotal = unitPrice (quantity=1)
			money.New(10000),
			5,
			money.New((forSale.PricePerUnit.Int64()*5)/100), // Commission amount: 5%
			money.New(3000), // Buyer service fee
			forSale.PricePerUnit.Add(money.New(10000)).Add(money.New((forSale.PricePerUnit.Int64()*5)/100)).Add(money.New(3000)),
			nil, // shippingSetupID: nil for test
			"JNE",
			"truck",
			nil,         // auctionSettlementType
			"immediate", // preparationTimeSnapshot
			nil,         // preparationNoteSnapshot
			nil,         // shippingSource
			nil,         // shippingQuoteID
			nil,         // shippingQuotePrice
			nil,         // pricingTokenID
			"instant",
			time.Now(),
		)
		order.IdempotencyKey = &sameKey
		return orderRepo.CreateOrderTx(ctx, tx, order)
	})
	require.NoError(t, err)

	// Buyer 2 should be able to use the SAME key (different scope)
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		// Check buyer 2's idempotency — should return nil (no collision)
		existingOrder, err := orderRepo.GetByIdempotencyKey(ctx, tx, buyer2ID, sameKey)
		if err != nil {
			return err
		}
		assert.Nil(t, existingOrder, "buyer 2 should not find buyer 1's order with same key")

		// Create buyer 2's order with same key
		forSale, err := forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
		if err != nil {
			return err
		}
		if err := forSale.ReduceQuantity(1); err != nil {
			return err
		}
		if err := forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
			return err
		}

		order := orderentity.NewOrderFromSource(
			buyer2ID,
			sellerID,
			orderentity.OrderSourceForSale,
			forSaleID,
			nil,
			1,
			forSale.PricePerUnit,
			forSale.PricePerUnit, // Subtotal = unitPrice (quantity=1)
			money.New(10000),
			5,
			money.New((forSale.PricePerUnit.Int64()*5)/100), // Commission amount: 5%
			money.New(3000), // Buyer service fee
			forSale.PricePerUnit.Add(money.New(10000)).Add(money.New((forSale.PricePerUnit.Int64()*5)/100)).Add(money.New(3000)),
			nil, // shippingSetupID: nil for test
			"JNE",
			"truck",
			nil,         // auctionSettlementType
			"immediate", // preparationTimeSnapshot
			nil,         // preparationNoteSnapshot
			nil,         // shippingSource
			nil,         // shippingQuoteID
			nil,         // shippingQuotePrice
			nil,         // pricingTokenID
			"instant",
			time.Now(),
		)
		order.IdempotencyKey = &sameKey
		return orderRepo.CreateOrderTx(ctx, tx, order)
	})
	require.NoError(t, err, "buyer 2 should be able to use same idempotency key")

	// Verify both orders exist independently
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		order1, _ := orderRepo.GetByIdempotencyKey(ctx, tx, buyer1ID, sameKey)
		order2, _ := orderRepo.GetByIdempotencyKey(ctx, tx, buyer2ID, sameKey)

		assert.NotNil(t, order1, "buyer 1 order should exist")
		assert.NotNil(t, order2, "buyer 2 order should exist")
		assert.NotEqual(t, order1.ID, order2.ID, "orders should have different IDs")
		assert.Equal(t, buyer1ID, order1.BuyerID, "order 1 should belong to buyer 1")
		assert.Equal(t, buyer2ID, order2.BuyerID, "order 2 should belong to buyer 2")

		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func insertOrderTestUsers(t *testing.T, ctx context.Context, testDB *testdb.TestDB, sellerID uuid.UUID, buyerIDs ...uuid.UUID) {
	t.Helper()

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local"); err != nil {
			return err
		}
		for _, buyerID := range buyerIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
				buyerID, "fb-"+buyerID.String()[:8], buyerID.String()+"@test.local"); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err, "seller/buyer user fixtures failed")
}
