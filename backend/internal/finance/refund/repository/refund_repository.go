// Package repository defines the refund repository interface.
package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/pkg/db"
)

// RefundRepository defines the interface for refund persistence.
type RefundRepository interface {
	// Create creates a new refund within a transaction.
	Create(ctx context.Context, tx db.Tx, refund *entity.Refund) error

	// GetByID retrieves a refund by ID without locking.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Refund, error)

	// GetByOrderID retrieves a refund by order ID without locking.
	GetByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Refund, error)

	// GetForUpdate retrieves a refund with FOR UPDATE lock.
	// This must be used within a transaction for state changes.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Refund, error)

	// Update updates an existing refund within a transaction.
	Update(ctx context.Context, tx db.Tx, refund *entity.Refund) error

	// ListByBuyer retrieves refunds for a buyer with pagination.
	ListByBuyer(ctx context.Context, tx db.Tx, buyerID uuid.UUID, limit int, offset int64) ([]*entity.Refund, error)

	// ListBySeller retrieves refunds for a seller with pagination.
	ListBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, limit int, offset int64) ([]*entity.Refund, error)

	// GetByGatewayIdempotencyKey looks up a refund by the idempotency key
	// we sent to the payment gateway. Used for duplicate-request detection
	// in the gateway refund orchestration. Returns nil if not found.
	GetByGatewayIdempotencyKey(ctx context.Context, tx db.Tx, key string) (*entity.Refund, error)

	// GetByGatewayRefundID looks up a refund by the gateway-side refund id
	// (Midtrans transaction id returned by the refund webhook). Returns nil
	// if not found. Used by the refund webhook handler.
	GetByGatewayRefundID(ctx context.Context, tx db.Tx, gatewayRefundID string) (*entity.Refund, error)

	// GetSuccessfulRefundTotalByOrder returns the cumulative amount of
	// successful gateway refunds already recorded for an order. excludeRefundID
	// allows the caller to omit the in-flight refund row from the sum.
	GetSuccessfulRefundTotalByOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID, excludeRefundID *uuid.UUID) (int64, error)

	// HasActiveRefundByOrderID returns true if the order has a refund in a
	// non-terminal status (anything other than 'refunded' or 'admin_released').
	// H2-F2a: Used by OrderCompletionService to block auto-complete while
	// refund is being negotiated or settled.
	HasActiveRefundByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (bool, error)

	// CreateEvidence creates an evidence attachment for a refund.
	CreateEvidence(ctx context.Context, tx db.Tx, refundID uuid.UUID, mediaURL string) error

	// ListEvidence retrieves all evidence URLs for a refund.
	ListEvidence(ctx context.Context, tx db.Tx, refundID uuid.UUID) ([]string, error)

	// ListByOrderID retrieves refunds for a specific order in deterministic
	// newest-first order using an opaque keyset cursor.
	ListByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID, limit int, cursor *OrderRefundCursor) ([]*entity.Refund, error)

	// S2C2: Cumulative product/shipping/coins/commission queries.
	// Each SUMs across all succeeded gateway refunds for the order,
	// excluding the given refundID (the in-flight row being processed).

	GetCumulativeProductRefundByOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID, excludeRefundID *uuid.UUID) (int64, error)
	GetCumulativeShippingRefundByOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID, excludeRefundID *uuid.UUID) (int64, error)
	GetCumulativeCoinsRefundedByOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID, excludeRefundID *uuid.UUID) (int64, error)
	// NOTE: commission reversal is NOT stored per refund row; it is derived
	// from GetCumulativeProductRefundByOrder + proportionalFloor(C, cumProduct, PD)
	// at the application layer (refund_math.go / refund_gateway.go).
}

// OrderRefundCursor is the canonical cursor for order-scoped refund history.
type OrderRefundCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c OrderRefundCursor) Encode() string {
	return fmt.Sprintf("%s|%s", c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID.String())
}

func DecodeOrderRefundCursor(raw string) (OrderRefundCursor, error) {
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 { return OrderRefundCursor{}, fmt.Errorf("malformed cursor") }
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil { return OrderRefundCursor{}, fmt.Errorf("malformed cursor timestamp: %w", err) }
	id, err := uuid.Parse(parts[1])
	if err != nil { return OrderRefundCursor{}, fmt.Errorf("malformed cursor id: %w", err) }
	return OrderRefundCursor{CreatedAt: ts, ID: id}, nil
}


