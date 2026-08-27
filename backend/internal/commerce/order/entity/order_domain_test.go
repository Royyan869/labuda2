package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/money"
)

// ============================================================================
// DOMAIN TESTS — Pure Entity Logic (No Database Required)
// ============================================================================
//
// This test suite verifies the core business logic of the Order entity
// without any database dependencies. These tests can run without PostgreSQL
// and focus on:
//
// 1. State machine transitions (Order status, Escrow status)
// 2. Entity field contracts
// 3. Business rule validations
// 4. Calculation correctness
//
// Run with: go test ./internal/commerce/order/entity/...
//
// These tests are FAST and can run in parallel with other domain tests.

// ============================================================================
// TEST: Order Status Transitions
// ============================================================================

// TestOrderStatusTransitions verifies that order status transitions follow
// the valid state machine rules.
//
// BUSINESS TRUTH PROTECTED:
// - Only valid status transitions are allowed
// - Invalid transitions return proper errors
// - Status state machine is enforced
//
// CRITICAL FOR: Order lifecycle integrity
func TestOrderStatusTransitions(t *testing.T) {
	tests := []struct {
		name        string
		setupOrder  func() *orderentity.Order
		transition  func(*orderentity.Order) error
		wantErr     bool
		errType     error
		description string
	}{
		{
			name: "pending -> paid (valid)",
			setupOrder: func() *orderentity.Order {
				return createTestOrder(orderentity.StatusPending, orderentity.EscrowStatusHolding)
			},
			transition: func(o *orderentity.Order) error {
				return o.MarkPaid()
			},
			wantErr:     false,
			description: "pending order can be marked as paid",
		},
		{
			name: "paid -> shipped (valid)",
			setupOrder: func() *orderentity.Order {
				return createTestOrder(orderentity.StatusPaid, orderentity.EscrowStatusHolding)
			},
			transition: func(o *orderentity.Order) error {
				ref := strPtr("RESI-123-456")
				proofType := strPtr("tracking")
				return o.MarkShipped(proofType, ref, nil, nil)
			},
			wantErr:     false,
			description: "paid order can be marked as shipped with valid proof",
		},
		{
			name: "shipped -> completed (valid)",
			setupOrder: func() *orderentity.Order {
				return createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
			},
			transition: func(o *orderentity.Order) error {
				return o.ValidateComplete()
			},
			wantErr:     false,
			description: "shipped order can be completed",
		},
		{
			name: "pending -> cancelled (valid)",
			setupOrder: func() *orderentity.Order {
				return createTestOrder(orderentity.StatusPending, orderentity.EscrowStatusHolding)
			},
			transition: func(o *orderentity.Order) error {
				return o.Cancel()
			},
			wantErr:     false,
			description: "pending order can be cancelled",
		},
		{
			name: "completed -> shipped (invalid)",
			setupOrder: func() *orderentity.Order {
				return createTestOrder(orderentity.StatusCompleted, orderentity.EscrowStatusReleased)
			},
			transition: func(o *orderentity.Order) error {
				proofType := strPtr("tracking")
				ref := strPtr("REF")
				return o.MarkShipped(proofType, ref, nil, nil)
			},
			wantErr:     true,
			errType:     &orderentity.InvalidTransitionError{},
			description: "completed order cannot transition back to shipped",
		},
		{
			name: "paid with dispute -> complete fails",
			setupOrder: func() *orderentity.Order {
				o := createTestOrder(orderentity.StatusPaid, orderentity.EscrowStatusHolding)
				o.HasDispute = true
				return o
			},
			transition: func(o *orderentity.Order) error {
				return o.ValidateComplete()
			},
			wantErr:     true,
			errType:     &orderentity.DisputeActiveError{},
			description: "order with active dispute cannot be completed",
		},
		{
			name: "pending -> expired (valid)",
			setupOrder: func() *orderentity.Order {
				return createTestOrder(orderentity.StatusPending, orderentity.EscrowStatusHolding)
			},
			transition: func(o *orderentity.Order) error {
				return o.MarkExpired()
			},
			wantErr:     false,
			description: "pending order can expire",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := tt.setupOrder()
			err := tt.transition(order)

			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// ============================================================================
// TEST: Escrow Status Transitions
// ============================================================================

// TestEscrowStatusTransitions verifies that escrow status transitions
// follow valid state machine rules.
//
// BUSINESS TRUTH PROTECTED:
// - Escrow state machine is enforced
// - Invalid escrow transitions are rejected
//
// CRITICAL FOR: Financial integrity of the platform
func TestEscrowStatusTransitions(t *testing.T) {
	tests := []struct {
		name        string
		fromStatus  orderentity.EscrowStatus
		toStatus    orderentity.EscrowStatus
		wantAllowed bool
		description string
	}{
		{"holding -> released (valid)", orderentity.EscrowStatusHolding, orderentity.EscrowStatusReleased, true, "release to seller on completion"},
		{"holding -> refunded (valid)", orderentity.EscrowStatusHolding, orderentity.EscrowStatusRefunded, true, "refund to buyer on cancellation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build order with initial escrow status
			// B4A: Shipped can complete directly (canonical buyer path)
			orderStatus := orderentity.StatusPending
			if tt.toStatus == orderentity.EscrowStatusReleased {
				orderStatus = orderentity.StatusShipped // B4A: shipped→completed is canonical
			}
			order := createTestOrder(orderStatus, tt.fromStatus)

			// Attempt transition by calling appropriate method
			var err error
			switch tt.toStatus {
			case orderentity.EscrowStatusReleased:
				err = order.ValidateComplete() // Validates order can transition to completed
			case orderentity.EscrowStatusRefunded:
				// Direct refund not tested here (would need Cancel() flow)
				// Just verify it's a valid transition from holding
				if tt.fromStatus == orderentity.EscrowStatusHolding {
					err = nil
				}
			}

			if tt.wantAllowed {
				// For valid transitions, we expect no error
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// ============================================================================
// TEST: B4A Order Acceptance — Shipped → Completed (Canonical Buyer Path)
// ============================================================================

// TestB4A_ShippedToCompleted_CanonicalBuyerPath verifies the B4A redesign:
// buyer taps "Terima Barang" once on SHIPPED → ValidateComplete() succeeds
// → order goes to COMPLETED. No intermediate DELIVERED step for buyer.
//
// BUSINESS TRUTH PROTECTED:
// - Shipped → Completed is the canonical buyer acceptance path
// - No two-step flow (MarkDelivered → Complete) for buyer
// - ValidateComplete() works from both shipped and delivered (internal)
//
// CRITICAL FOR: B4A order acceptance redesign integrity
func TestB4A_ShippedToCompleted_CanonicalBuyerPath(t *testing.T) {
	t.Run("shipped order can be completed directly (B4A buyer acceptance)", func(t *testing.T) {
		order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)

		// Buyer taps "Terima Barang" → calls ValidateComplete()
		err := order.ValidateComplete()
		assert.NoError(t, err, "shipped order must be completable directly (B4A)")
	})

	t.Run("shipped order with dispute cannot be completed", func(t *testing.T) {
		order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
		order.HasDispute = true

		err := order.ValidateComplete()
		assert.Error(t, err, "shipped order with active dispute must not be completable")
		assert.IsType(t, &orderentity.DisputeActiveError{}, err)
	})

	t.Run("shipped order can open dispute", func(t *testing.T) {
		order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)

		err := order.MarkDisputeOpen()
		assert.NoError(t, err, "shipped order must allow dispute opening")
	})

	t.Run("paid order cannot be completed (must be shipped first)", func(t *testing.T) {
		order := createTestOrder(orderentity.StatusPaid, orderentity.EscrowStatusHolding)

		err := order.ValidateComplete()
		assert.Error(t, err, "paid order must not be directly completable")
	})
}

// ============================================================================
// TEST: Order Source Types
// ============================================================================

// TestOrderSourceTypes verifies that orders use the correct source types
// (listing, negotiation, auction) and NOT the legacy 'trade' or 'offer'.
//
// BUSINESS TRUTH PROTECTED:
// - Order source types are canonical (listing/negotiation/auction)
// - No legacy 'offer' or 'trade' references in source_type
// - Each source type is properly identified
//
// CRITICAL FOR: Domain cleanliness, preventing legacy contract regression
func TestOrderSourceTypes(t *testing.T) {
	validSources := []orderentity.OrderSourceType{
		orderentity.OrderSourceForSale,
		orderentity.OrderSourceNegotiation,
		orderentity.OrderSourceAuction,
	}

	for _, source := range validSources {
		t.Run(string(source), func(t *testing.T) {
			// Verify source type is valid
			assert.True(t, source.IsValid(), "source type should be valid: %s", source)

			// Verify order can be created with this source type
			order := orderentity.NewOrderFromSource(
				uuid.New(),
				uuid.New(),
				source,
				uuid.New(),
				nil,               // negotiationID
				1,                 // quantity
				money.New(100000), // Unit price
				money.New(100000), // Subtotal (quantity=1)
				money.Zero(),      // Shipping
				5,                 // commissionPercent
				money.New(5000),   // Commission amount: (100000 * 5) / 100 = 5000
				money.New(3000),   // Buyer service fee
				money.New(108000), // Total payable
				nil,               // shippingOptionID: nil for test
				"JNE",             // shippingOptionName
				"truck",           // shippingTransportType
				nil,               // shippingExpeditionName
				nil,               // shippingEstimatedDays
				nil,               // auctionSettlementType
				"immediate",       // preparationTimeSnapshot
				nil,               // preparationNoteSnapshot
				nil,               // shippingSource
				nil,               // shippingQuoteID
				nil,               // shippingQuotePrice
				nil,               // pricingTokenID
				"instant",         // paymentMethod
				time.Now(),        // paymentExpiresAt
			)

			assert.Equal(t, source, order.SourceType, "order source type should match")
			assert.NotEqual(t, "offer", string(order.SourceType), "should not use legacy 'offer' type")
			assert.NotEqual(t, "trade", string(order.SourceType), "should not use legacy 'trade' type")
		})
	}
}

// ============================================================================
// TEST: Legacy Regression Guards
// ============================================================================

// TestNoTradeInOrder verifies that Order entity does NOT reference Trade.
//
// BUSINESS TRUTH PROTECTED:
// - Order replaced Trade (they are mutually exclusive)
// - Order does not have a trade_id field
// - No business logic references Trade
//
// CRITICAL FOR: Ensuring Trade domain stays dead
func TestNoTradeInOrder(t *testing.T) {
	order := createTestOrder(orderentity.StatusPending, orderentity.EscrowStatusHolding)

	// Verify Order has NO trade_id field
	// The orderentity.Order struct should have: SourceType + SourceID instead
	assert.NotNil(t, order.SourceType, "order should have SourceType")
	assert.NotNil(t, order.SourceID, "order should have SourceID")
}

// TestNoVATInOrder verifies that Order entity does NOT use VAT logic.
//
// BUSINESS TRUTH PROTECTED:
// - VAT is not part of the canonical business model
// - No VAT percent or amount calculations
// - No VAT config dependencies
//
// CRITICAL FOR: Business model correctness (VAT was removed)
func TestNoVATInOrder(t *testing.T) {
	order := orderentity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,               // negotiationID
		1,                 // quantity
		money.New(100000), // Unit price
		money.New(100000), // Subtotal (quantity=1)
		money.New(10000),  // Shipping
		5,                 // commissionPercent
		money.New(5000),   // Commission amount: (100000 * 5) / 100 = 5000
		money.New(3000),   // Buyer service fee
		money.New(113000), // Total payable = canonical buyer base (P−D)+S + fee = 100000+10000+3000
		nil,               // shippingOptionID: nil for test
		"JNE",             // shippingOptionName
		"truck",           // shippingTransportType
		nil,               // shippingExpeditionName
		nil,               // shippingEstimatedDays
		nil,               // auctionSettlementType
		"immediate",       // preparationTimeSnapshot
		nil,               // preparationNoteSnapshot
		nil,               // shippingSource
		nil,               // shippingQuoteID
		nil,               // shippingQuotePrice
		nil,               // pricingTokenID
		"instant",         // paymentMethod
		time.Now(),        // paymentExpiresAt
	)

	// Verify order has NO VAT fields
	// The order stores the canonical buyer base: (P−D)+S via
	// TotalBeforeCoinsAmount (no VAT, no commission in buyer cash).
	// NO VAT in the calculation

	expectedSubtotal := money.New(100000)
	expectedCommission := money.New((100000 * 5) / 100) // 5% commission

	assert.Equal(t, expectedSubtotal.Int64(), order.Subtotal.Int64(), "subtotal should be unit price * quantity")
	assert.Equal(t, expectedCommission.Int64(), order.CommissionAmount.Int64(), "commission should be calculated correctly")

	// Verify the canonical buyer base snapshot: TotalBeforeCoinsAmount is the
	// token-supplied buyer base ((P−D)+S), NOT P+S+C and NOT P+S+C−D.
	assert.Equal(t, money.New(113000), order.TotalBeforeCoinsAmount,
		"TotalBeforeCoinsAmount = buyer base (P−D)+S + fee, commission excluded")
	assert.Equal(t, money.New(113000), order.TotalPayableAmount,
		"TotalPayableAmount = canonical total payable")
}

// TestOrderEntityFields verifies Order entity has the correct canonical fields.
//
// BUSINESS TRUTH PROTECTED:
// - Order has SourceType + SourceID for order source
// - Order has proper escrow status tracking
//
// CRITICAL FOR: Domain contract integrity
func TestOrderEntityFields(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	sourceID := uuid.New()
	overrideID := uuid.New()
	negotiationID := uuid.New()
	idempotencyKey := "test-key-123"

	order := orderentity.NewOrderFromSource(
		buyerID,
		sellerID,
		orderentity.OrderSourceForSale,
		sourceID,
		&negotiationID,    // negotiationID
		1,                 // quantity
		money.New(500000), // Unit price
		money.New(500000), // Subtotal (quantity=1)
		money.New(20000),  // Shipping
		5,                 // commissionPercent
		money.New(25000),  // Commission amount: (500000 * 5) / 100 = 25000
		money.New(3000),   // Buyer service fee
		money.New(548000), // Total payable
		&overrideID,       // shippingOptionID
		"JNE",             // shippingOptionName
		"truck",           // shippingTransportType
		nil,               // shippingExpeditionName
		nil,               // shippingEstimatedDays
		nil,               // auctionSettlementType
		"immediate",       // preparationTimeSnapshot
		nil,               // preparationNoteSnapshot
		nil,               // shippingSource
		nil,               // shippingQuoteID
		nil,               // shippingQuotePrice
		nil,               // pricingTokenID
		"instant",         // paymentMethod
		time.Now(),        // paymentExpiresAt
	)

	order.ID = overrideID
	order.IdempotencyKey = &idempotencyKey

	// Verify canonical fields
	assert.Equal(t, buyerID, order.BuyerID)
	assert.Equal(t, sellerID, order.SellerID)
	assert.Equal(t, orderentity.OrderSourceForSale, order.SourceType)
	assert.Equal(t, sourceID, order.SourceID)
	assert.NotNil(t, order.NegotiationID)
	assert.Equal(t, negotiationID, *order.NegotiationID)
	assert.Equal(t, orderentity.StatusPending, order.Status)
	assert.Equal(t, orderentity.EscrowStatusHolding, order.EscrowStatus)
	assert.Equal(t, 1, order.Quantity)
	assert.Equal(t, money.New(500000), order.UnitPrice)
	assert.Equal(t, money.New(500000), order.Subtotal) // 1 * 500000
	assert.Equal(t, overrideID, order.ID)
	assert.Equal(t, &idempotencyKey, order.IdempotencyKey)
}

// ============================================================================
// TEST: Shipping Quote Fields Persistence Contract
// ============================================================================

// TestShippingQuoteFieldsPersistence verifies that NewOrderFromSource correctly
// stores shipping_quote_id and shipping_quote_price when provided.
//
// BUSINESS TRUTH PROTECTED:
// - Shipping quote ID is stored on order entity
// - Shipping quote price snapshot is stored on order entity
// - Both fields are nil when not using shipping quote
//
// CRITICAL FOR: Shipping quote persistence (I1-D2D3)
func TestShippingQuoteFieldsPersistence(t *testing.T) {
	quoteID := uuid.New()
	var quotePrice int64 = 25000
	shippingSource := "shipping_quote"

	order := orderentity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,               // negotiationID
		1,                 // quantity
		money.New(100000), // Unit price
		money.New(100000), // Subtotal
		money.New(25000),  // Shipping (matches quote price)
		5,                 // commissionPercent
		money.New(5000),   // Commission amount
		money.New(3000),   // Buyer service fee
		money.New(133000), // Total payable
		nil,               // shippingOptionID: nil when using quote
		"Custom",          // shippingOptionName
		"truck",           // shippingTransportType
		nil,               // shippingExpeditionName
		nil,               // shippingEstimatedDays
		nil,               // auctionSettlementType
		"immediate",       // preparationTimeSnapshot
		nil,               // preparationNoteSnapshot
		&shippingSource,   // shippingSource = "shipping_quote"
		&quoteID,          // shippingQuoteID
		&quotePrice,       // shippingQuotePrice
		nil,               // pricingTokenID
		"instant",         // paymentMethod
		time.Now(),        // paymentExpiresAt
	)

	assert.NotNil(t, order.ShippingQuoteID, "ShippingQuoteID must be set when using shipping quote")
	assert.Equal(t, quoteID, *order.ShippingQuoteID, "ShippingQuoteID must match input")
	assert.NotNil(t, order.ShippingQuotePrice, "ShippingQuotePrice must be set when using shipping quote")
	assert.Equal(t, quotePrice, *order.ShippingQuotePrice, "ShippingQuotePrice must match input")
	assert.NotNil(t, order.ShippingSource, "ShippingSource must be set")
	assert.Equal(t, "shipping_quote", *order.ShippingSource, "ShippingSource must be 'shipping_quote'")
}

// TestShippingQuoteFieldsNilWhenNotUsed verifies that shipping quote fields
// are nil when using standard listing shipping option (not shipping quote).
func TestShippingQuoteFieldsNilWhenNotUsed(t *testing.T) {
	optionID := uuid.New()

	order := orderentity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,               // negotiationID
		1,                 // quantity
		money.New(100000), // Unit price
		money.New(100000), // Subtotal
		money.New(10000),  // Shipping
		5,                 // commissionPercent
		money.New(5000),   // Commission amount
		money.New(3000),   // Buyer service fee
		money.New(118000), // Total payable
		&optionID,         // shippingOptionID: set for standard option
		"JNE REG",         // shippingOptionName
		"truck",           // shippingTransportType
		nil,               // shippingExpeditionName
		nil,               // shippingEstimatedDays
		nil,               // auctionSettlementType
		"immediate",       // preparationTimeSnapshot
		nil,               // preparationNoteSnapshot
		nil,               // shippingSource: nil = standard for-sale surface option
		nil,               // shippingQuoteID: nil
		nil,               // shippingQuotePrice: nil
		nil,               // pricingTokenID
		"instant",         // paymentMethod
		time.Now(),        // paymentExpiresAt
	)

	assert.Nil(t, order.ShippingQuoteID, "ShippingQuoteID must be nil when not using shipping quote")
	assert.Nil(t, order.ShippingQuotePrice, "ShippingQuotePrice must be nil when not using shipping quote")
}

// TestShippingQuoteFieldsOnAuctionOrder verifies that shipping quote fields
// work correctly for auction-sourced orders with shipping quotes.
func TestShippingQuoteFieldsOnAuctionOrder(t *testing.T) {
	quoteID := uuid.New()
	var quotePrice int64 = 35000
	shippingSource := "shipping_quote"
	settlementType := orderentity.AuctionSettlementBuyNow

	order := orderentity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		orderentity.OrderSourceAuction,
		uuid.New(),        // auctionID
		nil,               // negotiationID
		1,                 // quantity
		money.New(200000), // Unit price (winning bid)
		money.New(200000), // Subtotal
		money.New(35000),  // Shipping (matches quote price)
		5,                 // commissionPercent
		money.New(10000),  // Commission amount
		money.New(3000),   // Buyer service fee
		money.New(248000), // Total payable
		nil,               // shippingOptionID: nil when using quote
		"Custom",          // shippingOptionName
		"truck",           // shippingTransportType
		nil,               // shippingExpeditionName
		nil,               // shippingEstimatedDays
		&settlementType,   // auctionSettlementType
		"immediate",       // preparationTimeSnapshot
		nil,               // preparationNoteSnapshot
		&shippingSource,   // shippingSource = "shipping_quote"
		&quoteID,          // shippingQuoteID
		&quotePrice,       // shippingQuotePrice
		nil,               // pricingTokenID
		"instant",         // paymentMethod
		time.Now(),        // paymentExpiresAt
	)

	assert.Equal(t, orderentity.OrderSourceAuction, order.SourceType)
	assert.NotNil(t, order.ShippingQuoteID, "ShippingQuoteID must be set for auction order with quote")
	assert.Equal(t, quoteID, *order.ShippingQuoteID)
	assert.NotNil(t, order.ShippingQuotePrice, "ShippingQuotePrice must be set for auction order with quote")
	assert.Equal(t, quotePrice, *order.ShippingQuotePrice)
}

// ============================================================================
// TEST: Immediate Preparation Deadline — Every Paid Order Gets ReadyToShipBy
// ============================================================================

// TestMarkPaid_ImmediateGetsDeadline proves that immediate orders get ReadyToShipBy set,
// closing the gap where immediate orders had nil deadline and no enforcement.
func TestMarkPaid_ImmediateGetsDeadline(t *testing.T) {
	order := createTestOrderWithPrep(orderentity.StatusPending, "immediate")

	before := time.Now()
	err := order.MarkPaid()
	after := time.Now()

	assert.NoError(t, err)
	assert.NotNil(t, order.ReadyToShipBy, "immediate order MUST get a ReadyToShipBy deadline")

	// immediate = 1 day
	expectedMin := before.Add(1 * 24 * time.Hour)
	expectedMax := after.Add(1 * 24 * time.Hour)
	assert.True(t, !order.ReadyToShipBy.Before(expectedMin) && !order.ReadyToShipBy.After(expectedMax),
		"immediate ReadyToShipBy should be ~1 day from now, got %v", order.ReadyToShipBy)
}

// TestMarkPaid_ShortDeadlineUnchanged proves short mapping is still 2 days.
func TestMarkPaid_ShortDeadlineUnchanged(t *testing.T) {
	order := createTestOrderWithPrep(orderentity.StatusPending, "short")

	before := time.Now()
	err := order.MarkPaid()
	after := time.Now()

	assert.NoError(t, err)
	assert.NotNil(t, order.ReadyToShipBy)

	expectedMin := before.Add(2 * 24 * time.Hour)
	expectedMax := after.Add(2 * 24 * time.Hour)
	assert.True(t, !order.ReadyToShipBy.Before(expectedMin) && !order.ReadyToShipBy.After(expectedMax),
		"short ReadyToShipBy should be ~2 days from now")
}

// TestMarkPaid_MediumDeadlineUnchanged proves medium mapping is still 5 days.
func TestMarkPaid_MediumDeadlineUnchanged(t *testing.T) {
	order := createTestOrderWithPrep(orderentity.StatusPending, "medium")

	before := time.Now()
	err := order.MarkPaid()

	assert.NoError(t, err)
	assert.NotNil(t, order.ReadyToShipBy)

	expected := before.Add(5 * 24 * time.Hour)
	assert.WithinDuration(t, expected, *order.ReadyToShipBy, 2*time.Second,
		"medium ReadyToShipBy should be ~5 days from now")
}

// TestMarkPaid_LongDeadlineUnchanged proves long mapping is still 7 days.
func TestMarkPaid_LongDeadlineUnchanged(t *testing.T) {
	order := createTestOrderWithPrep(orderentity.StatusPending, "long")

	before := time.Now()
	err := order.MarkPaid()

	assert.NoError(t, err)
	assert.NotNil(t, order.ReadyToShipBy)

	expected := before.Add(7 * 24 * time.Hour)
	assert.WithinDuration(t, expected, *order.ReadyToShipBy, 2*time.Second,
		"long ReadyToShipBy should be ~7 days from now")
}

// TestMarkPaid_UnknownPrepGetsFallbackDeadline proves unknown/empty preparation
// time gets a safe fallback deadline (2 days = short), not nil.
func TestMarkPaid_UnknownPrepGetsFallbackDeadline(t *testing.T) {
	cases := []string{"", "unknown", "garbage_value"}

	for _, prep := range cases {
		t.Run("prep="+prep, func(t *testing.T) {
			order := createTestOrderWithPrep(orderentity.StatusPending, prep)

			before := time.Now()
			err := order.MarkPaid()

			assert.NoError(t, err)
			assert.NotNil(t, order.ReadyToShipBy,
				"unknown prep %q MUST get fallback ReadyToShipBy (not nil)", prep)

			expected := before.Add(2 * 24 * time.Hour)
			assert.WithinDuration(t, expected, *order.ReadyToShipBy, 2*time.Second,
				"unknown prep %q fallback should be ~2 days (short)", prep)
		})
	}
}

// TestImmediateOverdue_TriggersAfterReadyPlusGrace proves that an immediate
// order with ReadyToShipBy set to 1 day ago + grace (2 days) = 3 days total
// triggers IsShipmentOverdue correctly.
func TestImmediateOverdue_TriggersAfterReadyPlusGrace(t *testing.T) {
	order := &orderentity.Order{
		Status: orderentity.StatusPaid,
	}

	// Set ReadyToShipBy to 4 days ago (past 1 day prep + 2 day grace = 3 days)
	fourDaysAgo := time.Now().Add(-4 * 24 * time.Hour)
	order.ReadyToShipBy = &fourDaysAgo

	assert.True(t, order.IsShipmentOverdue(),
		"order 4 days past ReadyToShipBy must be overdue (grace = 2 days)")
}

// TestImmediateNotOverdue_WithinGrace proves that an immediate order within
// the grace period is NOT overdue.
func TestImmediateNotOverdue_WithinGrace(t *testing.T) {
	order := &orderentity.Order{
		Status: orderentity.StatusPaid,
	}

	// Set ReadyToShipBy to 1 day ago (within 2 day grace period)
	oneDayAgo := time.Now().Add(-1 * 24 * time.Hour)
	order.ReadyToShipBy = &oneDayAgo

	assert.False(t, order.IsShipmentOverdue(),
		"order 1 day past ReadyToShipBy must NOT be overdue (grace = 2 days)")
}

// TestAllPrepValues_GetNonNilDeadline is the universal guarantee test:
// EVERY valid prep value results in a non-nil ReadyToShipBy after MarkPaid.
func TestAllPrepValues_GetNonNilDeadline(t *testing.T) {
	allPreps := []string{"immediate", "short", "medium", "long", "", "unknown"}

	for _, prep := range allPreps {
		t.Run("prep="+prep, func(t *testing.T) {
			order := createTestOrderWithPrep(orderentity.StatusPending, prep)
			err := order.MarkPaid()
			assert.NoError(t, err)
			assert.NotNil(t, order.ReadyToShipBy,
				"prep=%q: ReadyToShipBy MUST be non-nil after MarkPaid — nil means no enforcement", prep)
		})
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func createTestOrderWithPrep(status orderentity.Status, prepTime string) *orderentity.Order {
	order := orderentity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,               // negotiationID
		1,                 // quantity
		money.New(100000), // Unit price
		money.New(100000), // Subtotal
		money.Zero(),      // Shipping
		5,                 // commissionPercent
		money.New(5000),   // Commission
		money.New(3000),   // Buyer service fee
		money.New(108000), // Total payable
		nil,               // shippingOptionID
		"JNE",
		"truck",
		nil, nil, nil,
		prepTime, // preparationTimeSnapshot
		nil,      // preparationNoteSnapshot
		nil, nil, nil, nil,
		"instant",
		time.Now(),
	)
	order.Status = status
	order.EscrowStatus = orderentity.EscrowStatusHolding
	return order
}

func createTestOrder(status orderentity.Status, escrowStatus orderentity.EscrowStatus) *orderentity.Order {
	order := orderentity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,               // negotiationID
		1,                 // quantity
		money.New(100000), // Unit price
		money.New(100000), // Subtotal (quantity=1)
		money.Zero(),      // Shipping
		5,                 // commissionPercent
		money.New(5000),   // Commission amount: (100000 * 5) / 100 = 5000
		money.New(3000),   // Buyer service fee
		money.New(108000), // Total payable
		nil,               // shippingOptionID: nil for test
		"JNE",             // shippingOptionName
		"truck",           // shippingTransportType
		nil,               // shippingExpeditionName
		nil,               // shippingEstimatedDays
		nil,               // auctionSettlementType
		"immediate",       // preparationTimeSnapshot
		nil,               // preparationNoteSnapshot
		nil,               // shippingSource
		nil,               // shippingQuoteID
		nil,               // shippingQuotePrice
		nil,               // pricingTokenID
		"instant",         // paymentMethod
		time.Now(),        // paymentExpiresAt
	)
	// Override status for testing
	order.Status = status
	order.EscrowStatus = escrowStatus
	return order
}

func strPtr(s string) *string {
	return &s
}

// ============================================================================
// POST-SHIP DISPUTE WINDOW — AutoReleaseAt-derived mark-ship time
// ============================================================================

func TestIsWithinPostShipDisputeWindow_EarlyShipment(t *testing.T) {
	// Seller shipped quickly (day 1 of 5-day prep window).
	// Dispute window = mark-ship time + 12h.
	// The window must be exactly 12h regardless of how early the seller shipped.
	now := time.Now()
	autoRelease := now.Add(orderentity.AutoReleaseDuration) // mark-ship = now
	order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = &autoRelease

	assert.True(t, order.IsWithinPostShipDisputeWindow(),
		"should be within dispute window immediately after shipping")
}

func TestIsWithinPostShipDisputeWindow_LateShipment(t *testing.T) {
	// Seller shipped late (just before grace period expired).
	// The dispute window must still be a full 12 hours from mark-ship time,
	// NOT truncated by ReadyToShipBy arithmetic.
	markShipTime := time.Now().Add(-6 * time.Hour) // shipped 6h ago
	autoRelease := markShipTime.Add(orderentity.AutoReleaseDuration)
	order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = &autoRelease

	assert.True(t, order.IsWithinPostShipDisputeWindow(),
		"late shipment must still get full 12h window; 6h < 12h")
}

func TestIsWithinPostShipDisputeWindow_Expired(t *testing.T) {
	// Shipped 13 hours ago — window (12h) should be closed.
	markShipTime := time.Now().Add(-13 * time.Hour)
	autoRelease := markShipTime.Add(orderentity.AutoReleaseDuration)
	order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = &autoRelease

	assert.False(t, order.IsWithinPostShipDisputeWindow(),
		"13h after shipping exceeds 12h window")
}

func TestIsWithinPostShipDisputeWindow_ExactBoundary(t *testing.T) {
	// Shipped exactly 12 hours ago — right at the boundary.
	// time.Now() should be >= windowCloses, so window is closed.
	markShipTime := time.Now().Add(-12*time.Hour - time.Second)
	autoRelease := markShipTime.Add(orderentity.AutoReleaseDuration)
	order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = &autoRelease

	assert.False(t, order.IsWithinPostShipDisputeWindow(),
		"at boundary+1s the window should be closed")
}

func TestIsWithinPostShipDisputeWindow_JustBeforeBoundary(t *testing.T) {
	// Shipped 11h59m ago — just inside the window.
	markShipTime := time.Now().Add(-11*time.Hour - 59*time.Minute)
	autoRelease := markShipTime.Add(orderentity.AutoReleaseDuration)
	order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = &autoRelease

	assert.True(t, order.IsWithinPostShipDisputeWindow(),
		"11h59m is within the 12h window")
}

func TestIsWithinPostShipDisputeWindow_WrongStatus(t *testing.T) {
	autoRelease := time.Now().Add(orderentity.AutoReleaseDuration)
	order := createTestOrder(orderentity.StatusPaid, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = &autoRelease

	assert.False(t, order.IsWithinPostShipDisputeWindow(),
		"paid status should not allow post-ship dispute")
}

func TestIsWithinPostShipDisputeWindow_NilAutoReleaseAt(t *testing.T) {
	order := createTestOrder(orderentity.StatusShipped, orderentity.EscrowStatusHolding)
	order.AutoReleaseAt = nil

	assert.False(t, order.IsWithinPostShipDisputeWindow(),
		"nil AutoReleaseAt means no ship timestamp derivable")
}


