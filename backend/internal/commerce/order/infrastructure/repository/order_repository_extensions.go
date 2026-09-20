package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	orderrepo "github.com/labuda/backend/internal/commerce/order/repository"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

func quoteBlockingOrderStatuses() []string {
	return []string{
		string(entity.StatusPending),
		string(entity.StatusPaid),
		string(entity.StatusShipped),
		string(entity.StatusDelivered),
		string(entity.StatusDisputeOpen),
		string(entity.StatusCompleted),
		string(entity.StatusPartiallyRefunded),
	}
}

// GetBlockingOrderByShippingQuoteID retrieves an order by shipping quote ID only
// when the order is in a quote-blocking status.
//
// NARROW PROJECTION: this reader hydrates the order's identity, status, escrow
// status, quote metadata and address snapshot only. Its sole consumer
// (shipping-quote reactivation) reads the status; money fields and lifecycle
// deadlines are NOT hydrated and must never be read from this result — use
// GetByID/GetForUpdate when the caller needs the canonical money base or
// payment/completion deadlines.
func (r *OrderRepository) GetBlockingOrderByShippingQuoteID(
	ctx context.Context,
	tx db.Tx,
	shippingQuoteID uuid.UUID,
) (*entity.Order, error) {
	var id, buyerID, sellerID, sourceID uuid.UUID
	var shippingSetupID *uuid.UUID // NULLABLE: nil when using shipping quote
	var negotiationID *uuid.UUID
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var status, escrowStatus, sourceType string
	var shippingSetupName, shippingTransportType sql.NullString // NULLABLE in DB
	var trackingNumber, shippingNote *string
	var orderNum *string
	var autoReleaseAt *time.Time
	var completedAt sql.NullTime
	var hasDispute bool
	var confirmationExtensionUsed bool
	var idempotencyKeyPtr *string
	var preparationTimeSnapshot sql.NullString // NULLABLE in DB
	var preparationNoteSnapshot *string
	var readyToShipBy *time.Time
	var addressSnapshotJSON []byte
	var originSnapshotJSON []byte
	var shippingSourcePtr *string
	var shippingQuoteIDPtr *uuid.UUID
	var shippingQuotePricePtr *int64
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id,
		       source_type, source_id, negotiation_id,
		       quantity, unit_price, subtotal, shipping_total, commission_percent,
		       commission_amount, status, escrow_status,
		       auto_release_at, has_dispute, confirmation_extension_used, idempotency_key,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
		       tracking_number, shipping_note, order_number,
		       preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by,
		       address_snapshot,
		       shipping_source, shipping_origin_snapshot,
		       shipping_quote_id, shipping_quote_price,
		       completed_at, created_at, updated_at
		FROM orders
		WHERE shipping_quote_id = $1
		  AND status::text = ANY($2::text[])
		ORDER BY created_at DESC
		LIMIT 1
	`, shippingQuoteID, quoteBlockingOrderStatuses()).Scan(
		&id, &buyerID, &sellerID,
		&sourceType, &sourceID, &negotiationID,
		&quantity, &unitPrice, &subtotal, &shippingTotal, &commissionPercent,
		&commissionAmount, &status, &escrowStatus,
		&autoReleaseAt, &hasDispute, &confirmationExtensionUsed, &idempotencyKeyPtr,
		&shippingSetupID, &shippingSetupName, &shippingTransportType,
		&trackingNumber, &shippingNote,
		&orderNum,
		&preparationTimeSnapshot, &preparationNoteSnapshot, &readyToShipBy, &addressSnapshotJSON,
		&shippingSourcePtr, &originSnapshotJSON,
		&shippingQuoteIDPtr, &shippingQuotePricePtr,
		&completedAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get blocking order by shipping quote id failed: %w", err)
	}

	var shippingDestination *addressentity.AddressSnapshot
	if addressSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(addressSnapshotJSON, &snapshot); err == nil {
			shippingDestination = &snapshot
		}
	}

	var shippingOrigin *addressentity.AddressSnapshot
	if originSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(originSnapshotJSON, &snapshot); err == nil {
			shippingOrigin = &snapshot
		}
	}

	order := &entity.Order{
		ID:                        id,
		BuyerID:                   buyerID,
		SellerID:                  sellerID,
		SourceType:                entity.OrderSourceType(sourceType),
		SourceID:                  sourceID,
		NegotiationID:             negotiationID,
		Quantity:                  quantity,
		UnitPrice:                 money.New(unitPrice),
		Subtotal:                  money.New(subtotal),
		ShippingTotal:             money.New(shippingTotal),
		CommissionPercent:         commissionPercent,
		CommissionAmount:          money.New(commissionAmount),
		ShippingSetupID:          shippingSetupID,
		ShippingSetupName:        shippingSetupName.String,
		ShippingTransportType:     shippingTransportType.String,
		TrackingNumber:            trackingNumber,
		ShippingNote:              shippingNote,
		OrderNumber:               orderNum,
		ShippingSource:            shippingSourcePtr,
		ShippingOrigin:            shippingOrigin,
		ShippingQuoteID:           shippingQuoteIDPtr,
		ShippingQuotePrice:        shippingQuotePricePtr,
		PreparationTimeSnapshot:   preparationTimeSnapshot.String,
		PreparationNoteSnapshot:   preparationNoteSnapshot,
		ReadyToShipBy:             readyToShipBy,
		AddressSnapshot:           shippingDestination,
		Status:                    entity.Status(status),
		EscrowStatus:              entity.EscrowStatus(escrowStatus),
		HasDispute:                hasDispute,
		ConfirmationExtensionUsed: confirmationExtensionUsed,
		IdempotencyKey:            idempotencyKeyPtr,
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}

	if autoReleaseAt != nil {
		ts := *autoReleaseAt
		order.AutoReleaseAt = &ts
	}

	if completedAt.Valid {
		ts := completedAt.Time
		order.CompletedAt = &ts
	}

	return order, nil
}

// GetOrderItems retrieves all order items for a given order.
func (r *OrderRepository) GetOrderItems(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) ([]*entity.OrderItem, error) {
	query := `
		SELECT id, order_id, product_id, name, unit_price_snapshot, quantity, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at ASC
	`

	rows, err := tx.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order items failed: %w", err)
	}
	defer rows.Close()

	var items []*entity.OrderItem
	for rows.Next() {
		var item entity.OrderItem
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductID,
			&item.Name,
			&item.UnitPriceSnapshot,
			&item.Quantity,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan order item failed: %w", err)
		}
		items = append(items, &item)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate order items failed: %w", rows.Err())
	}

	return items, nil
}

// GetOrderStats retrieves order statistics for a given user and role.
func (r *OrderRepository) GetOrderStats(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	isSeller bool,
) (*orderrepo.OrderStats, error) {
	var stats orderrepo.OrderStats

	// Build query based on role (buyer vs seller)
	var whereClause string
	if isSeller {
		whereClause = "WHERE seller_id = $1"
	} else {
		whereClause = "WHERE buyer_id = $1"
	}

	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending_payment') as pending_payment,
			COUNT(*) FILTER (WHERE status = 'paid') as paid,
			COUNT(*) FILTER (WHERE status = 'shipped') as shipped,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled
		FROM orders
		` + whereClause

	var total, pendingPayment, paid, shipped, completed, cancelled int64
	err := tx.QueryRow(ctx, query, userID).Scan(
		&total, &pendingPayment, &paid, &shipped, &completed, &cancelled,
	)

	if err != nil {
		return nil, fmt.Errorf("get order stats failed: %w", err)
	}

	stats.TotalOrders = total
	stats.PendingPayment = pendingPayment
	stats.Paid = paid
	stats.Shipped = shipped
	stats.Completed = completed
	stats.Cancelled = cancelled

	return &stats, nil
}

// CountActiveOrdersByProduct counts active orders for a product.
// Active orders are those with status: pending, paid, shipped, delivered.
// This is used to prevent shipping option changes when orders are in flight.
func (r *OrderRepository) CountActiveOrdersByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) (int64, error) {
	var count int64
	query := `
		SELECT COUNT(*)
		FROM orders o
		INNER JOIN order_items oi ON o.id = oi.order_id
		WHERE oi.product_id = $1
		  AND o.status IN ('pending_payment', 'paid', 'shipped', 'delivered')
	`
	err := tx.QueryRow(ctx, query, productID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active orders for product failed: %w", err)
	}
	return count, nil
}

// CountAnyOrdersByProduct counts any orders for a product.
// This is used to prevent critical field edits when any orders exist.
func (r *OrderRepository) CountAnyOrdersByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) (int64, error) {
	var count int64
	query := `
		SELECT COUNT(*)
		FROM order_items
		WHERE product_id = $1
	`
	err := tx.QueryRow(ctx, query, productID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count any orders for product failed: %w", err)
	}
	return count, nil
}


