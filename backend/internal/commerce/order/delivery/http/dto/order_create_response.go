package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
)

// OrderCreateResponse represents the response for POST /api/v1/orders.
//
// This is a lightweight DTO that converts Money fields to int64 for JSON serialization.
// The raw Order entity contains money.Money fields which serialize as {} without a custom
// MarshalJSON. This DTO ensures all Money fields are serialized as JSON numbers.
//
// CANONICAL POST /orders CONTRACT:
// This response is the lightweight creation confirmation sent back to the client
// immediately after order creation. It is NOT the full order detail (GET /orders/:id).
//
// MOBILE CONSUMER (checkout_repository_impl.dart):
//   - id: required (UUID string)
//   - order_number: optional (string)
//   - status: required (string) - defaults to "pending_payment" if missing
//   - subtotal: required (int64) - money.Money converted to int64
//   - shipping_total: required (int64) - money.Money converted to int64
//   - commission_amount: required (int64) - money.Money converted to int64
//   - total_before_coins_amount: required (int64) - money.Money converted to int64
//   - created_at: required (RFC3339 string) - parsed via DateTime.tryParse()
type OrderCreateResponse struct {
	// Order identification
	ID          uuid.UUID `json:"id"`
	OrderNumber *string   `json:"order_number,omitempty"`
	Status      string    `json:"status"`

	// Pricing snapshot (Money fields converted to int64)
	Subtotal               int64 `json:"subtotal"`
	ShippingTotal          int64 `json:"shipping_total"`
	CommissionAmount       int64 `json:"commission_amount"`
	TotalBeforeCoinsAmount int64 `json:"total_before_coins_amount"`

	// Timestamps
	// RFC3339 string format matching mobile parser expectation:
	// final createdAtStr = responseData['created_at'] as String?;
	// final createdAt = createdAtStr != null ? DateTime.tryParse(createdAtStr) ...
	CreatedAt string `json:"created_at"`
}

// OrderToCreateResponse converts an Order entity to OrderCreateResponse.
// Money fields are converted to int64 using .Int64() method.
// Timestamps use RFC3339 string format matching mobile parser expectation.
func OrderToCreateResponse(order *entity.Order) *OrderCreateResponse {
	return &OrderCreateResponse{
		ID:                     order.ID,
		OrderNumber:            order.OrderNumber,
		Status:                 string(order.Status),
		Subtotal:               order.Subtotal.Int64(),
		ShippingTotal:          order.ShippingTotal.Int64(),
		CommissionAmount:       order.CommissionAmount.Int64(),
		TotalBeforeCoinsAmount: order.TotalBeforeCoinsAmount.Int64(),
		CreatedAt:              order.CreatedAt.UTC().Format(time.RFC3339),
	}
}
