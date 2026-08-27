package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// OrderQuery provides minimal order data needed for rating validation.
// This avoids coupling the rating domain to the full order entity.
type OrderQuery struct {
	ID        uuid.UUID
	BuyerID   uuid.UUID
	SellerID  uuid.UUID
	Status    string
	OrderType string
	HasDispute bool // 🔥 TASK 4: Dispute exclusion - prevent rating during active disputes
}

// OrderQueryRepository provides read-only access to order data for rating validation.
// It only queries the fields needed for rating operations.
type OrderQueryRepository struct{}

// NewOrderQueryRepository creates a new OrderQueryRepository.
func NewOrderQueryRepository() *OrderQueryRepository {
	return &OrderQueryRepository{}
}

// GetForUpdate retrieves order data with FOR UPDATE lock for rating validation.
// Returns nil if order not found.
func (r *OrderQueryRepository) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*OrderQuery, error) {
	var id, buyerID, sellerID uuid.UUID
	var status, orderType string
	var hasDispute bool

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id, status, order_type, has_dispute
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(
		&id, &buyerID, &sellerID, &status, &orderType, &hasDispute,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // Order not found
		}
		return nil, fmt.Errorf("get order for update failed: %w", err)
	}

	return &OrderQuery{
		ID:        id,
		BuyerID:   buyerID,
		SellerID:  sellerID,
		Status:    status,
		OrderType: orderType,
		HasDispute: hasDispute, // 🔥 TASK 4: Include dispute status
	}, nil
}

// Get retrieves order data without locking for read operations.
// Returns nil if order not found.
func (r *OrderQueryRepository) Get(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*OrderQuery, error) {
	var id, buyerID, sellerID uuid.UUID
	var status, orderType string
	var hasDispute bool

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id, status, order_type, has_dispute
		FROM orders
		WHERE id = $1
	`, orderID).Scan(
		&id, &buyerID, &sellerID, &status, &orderType, &hasDispute,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // Order not found
		}
		return nil, fmt.Errorf("get order failed: %w", err)
	}

	return &OrderQuery{
		ID:        id,
		BuyerID:   buyerID,
		SellerID:  sellerID,
		Status:    status,
		OrderType: orderType,
		HasDispute: hasDispute, // 🔥 TASK 4: Include dispute status
	}, nil
}


