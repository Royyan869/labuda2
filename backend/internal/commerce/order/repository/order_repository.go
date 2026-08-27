package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/db"
)

// ErrDuplicatePricingToken is returned when an order with the same pricing_token_id already exists.
// This is the canonical duplicate signal for retrying the same checkout attempt.
var ErrDuplicatePricingToken = errors.New("order with this pricing_token_id already exists")

// ErrDuplicateSource is returned when an order with the same (source_type, source_id) already exists.
// This is expected behavior for idempotent auction settlement.
var ErrDuplicateSource = errors.New("order with this source_type and source_id already exists")

// ErrDuplicateIdempotencyKey is returned when an order with the same (buyer_id, idempotency_key)
// already exists. The buyer-scoped idempotency key has been reused for a different request payload
// (e.g. different pricing token). Callers should surface a 409 to the client.
var ErrDuplicateIdempotencyKey = errors.New("order with this buyer_id and idempotency_key already exists")

// OrderRepository handles order persistence.
type OrderRepository interface {
	// CreateOrderTx persists a new order within a transaction.
	CreateOrderTx(ctx context.Context, tx db.Tx, order *entity.Order) error

	// CreateOrderItemTx persists an order item within a transaction.
	CreateOrderItemTx(ctx context.Context, tx db.Tx, orderItem *entity.OrderItem) error

	// GetByID retrieves an order without locking (for read-only operations).
	GetByID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Order, error)

	// GetForUpdate retrieves an order with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetForUpdate(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Order, error)

	// UpdateStatusTx persists order status and escrow status changes.
	UpdateStatusTx(ctx context.Context, tx db.Tx, order *entity.Order) error

	// GetByPricingTokenID retrieves an order by its pricing token ID.
	// Returns nil if no order found with the given pricing token.
	GetByPricingTokenID(ctx context.Context, tx db.Tx, pricingTokenID uuid.UUID) (*entity.Order, error)

	// GetByIdempotencyKey retrieves an order by buyer_id and idempotency key.
	// Idempotency is scoped per-buyer, so both buyer_id and key are required.
	// Returns nil if no order found with the given buyer and key.
	GetByIdempotencyKey(ctx context.Context, tx db.Tx, buyerID uuid.UUID, idempotencyKey string) (*entity.Order, error)

	// GetByShippingQuoteID retrieves an order by its shipping quote ID.
	// SCENARIO 6 FIX: Prevents double-order attacks from same shipping quote.
	// Returns nil if no order found with the given shipping quote.
	GetByShippingQuoteID(ctx context.Context, tx db.Tx, shippingQuoteID uuid.UUID) (*entity.Order, error)

	// GetBlockingOrderByShippingQuoteID retrieves an order by shipping quote ID
	// only when the order is in a quote-blocking status.
	GetBlockingOrderByShippingQuoteID(ctx context.Context, tx db.Tx, shippingQuoteID uuid.UUID) (*entity.Order, error)

	// CountValidOrdersByShippingQuoteID counts orders with valid statuses for a shipping quote.
	// Valid statuses are quote-blocking statuses.
	// Used for shipping quote reactivation validation to prevent duplicate orders.
	CountValidOrdersByShippingQuoteID(ctx context.Context, tx db.Tx, shippingQuoteID uuid.UUID) (int64, error)

	// GetBySource retrieves an order by its source type and source ID.
	GetBySource(ctx context.Context, tx db.Tx, sourceType string, sourceID uuid.UUID) (*entity.Order, error)

	// GetOrderItems retrieves all order items for a given order.
	GetOrderItems(ctx context.Context, tx db.Tx, orderID uuid.UUID) ([]*entity.OrderItem, error)

	// FindOrdersForAutoComplete returns IDs of orders that are due for auto-completion.
	// Uses FOR UPDATE SKIP LOCKED to support concurrent workers.
	// Query conditions: status IN ('shipped', 'delivered'), escrow_status = 'holding', auto_release_at <= NOW()
	FindOrdersForAutoComplete(ctx context.Context, tx db.Tx, limit int) ([]uuid.UUID, error)

	// FindOverdueOrdersForCancel returns IDs of orders that are overdue for shipment.
	// Uses FOR UPDATE SKIP LOCKED to support concurrent workers.
	// Query conditions: status = 'paid', escrow_status = 'holding', ready_to_ship_by + grace_period < NOW()
	FindOverdueOrdersForCancel(ctx context.Context, tx db.Tx, limit int) ([]uuid.UUID, error)

	// GetByOrderNumber retrieves an order by its human-readable order number.
	GetByOrderNumber(ctx context.Context, tx db.Tx, orderNumber string) (*entity.Order, error)

	// CreateShippingProofTx creates a shipping proof within a transaction.
	CreateShippingProofTx(ctx context.Context, tx db.Tx, proof *entity.ShippingProof) error

	// GetShippingProofsByOrderID retrieves all shipping proofs for an order.
	GetShippingProofsByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) ([]*entity.ShippingProof, error)

	// GetOrderStats retrieves order statistics for a given user and role.
	GetOrderStats(ctx context.Context, tx db.Tx, userID uuid.UUID, isSeller bool) (*OrderStats, error)

	// CountActiveOrdersByProduct counts active orders for a product.
	// Active orders are those with status: pending, paid, shipped, delivered.
	CountActiveOrdersByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error)

	// CountAnyOrdersByProduct counts any orders for a product.
	CountAnyOrdersByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error)
}

// OrderStats represents order statistics grouped by status.
type OrderStats struct {
	TotalOrders    int64
	PendingPayment int64
	Paid           int64
	Shipped        int64
	Completed      int64
	Cancelled      int64
}

// OrderListSummary represents a simplified order for listing.
type OrderListSummary struct {
	OrderID     uuid.UUID
	OrderNumber string
	Status      string
	BuyerID     uuid.UUID
	Subtotal    int64
	Total       int64
	CreatedAt   time.Time
}


