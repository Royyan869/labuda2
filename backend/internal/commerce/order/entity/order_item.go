package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// OrderItem represents a line item within an order.
// It captures the product relationship and price snapshot at order time.
type OrderItem struct {
	ID                uuid.UUID
	OrderID           uuid.UUID
	ProductID         uuid.UUID
	UnitPriceSnapshot money.Money // Price per unit at time of order (immutable snapshot, required)
	Quantity          int         // Quantity purchased
	Name              string      // Item name (copied from listing for historical accuracy)
	CreatedAt         time.Time
}

// NewOrderItem creates a new order item with product-based pricing.
func NewOrderItem(orderID, productID uuid.UUID, unitPriceSnapshot money.Money, quantity int, name string) *OrderItem {
	return &OrderItem{
		ID:                uuid.New(),
		OrderID:           orderID,
		ProductID:         productID,
		UnitPriceSnapshot: unitPriceSnapshot,
		Quantity:          quantity,
		Name:              name,
		CreatedAt:         time.Now(),
	}
}

// Subtotal returns the total price for this order item.
func (oi *OrderItem) Subtotal() money.Money {
	return oi.UnitPriceSnapshot.Mul(int64(oi.Quantity))
}


