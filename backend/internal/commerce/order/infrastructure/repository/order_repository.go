package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// OrderRepository handles order persistence using pgx-based DB layer.
// It enforces row-level locking for concurrent state transitions.
type OrderRepository struct{}

// NewOrderRepository creates a new OrderRepository.
func NewOrderRepository() *OrderRepository {
	return &OrderRepository{}
}

// CreateOrderTx persists a new order within a transaction.
func (r *OrderRepository) CreateOrderTx(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	// Marshal address snapshot to JSONB
	var addressSnapshotJSON []byte
	if order.ShippingDestination != nil {
		addrBytes, err := json.Marshal(order.ShippingDestination)
		if err != nil {
			return fmt.Errorf("marshal address snapshot failed: %w", err)
		}
		addressSnapshotJSON = addrBytes
	} else {
		addressSnapshotJSON = nil
	}

	// Marshal shipping origin snapshot to JSONB
	var originSnapshotJSON []byte
	if order.ShippingOrigin != nil {
		originBytes, err := json.Marshal(order.ShippingOrigin)
		if err != nil {
			return fmt.Errorf("marshal shipping origin snapshot failed: %w", err)
		}
		originSnapshotJSON = originBytes
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id,
			source_type, source_id, negotiation_id,
			quantity, unit_price, subtotal, shipping_total, commission_percent,
			commission_amount, service_fee_amount, total_payable_amount,
			total_before_coins_amount,
			status, escrow_status,
			auto_release_at, has_dispute, idempotency_key,
			shipping_option_id, shipping_option_name, shipping_transport_type,
			preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by, address_snapshot,
			pricing_token_id,
			payment_expires_at,
			shipping_source, shipping_origin_snapshot,
			shipping_quote_id, shipping_quote_price,
			order_number,
			completed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
		        $15,
		        $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29,
		        $30, $31, $32, $33, $34, $35, $36, $37)
	`,
		order.ID,
		order.BuyerID,
		order.SellerID,
		string(order.SourceType),
		order.SourceID,
		order.NegotiationID,
		order.Quantity,
		order.UnitPrice.Int64(),
		order.Subtotal.Int64(),
		order.ShippingTotal.Int64(),
		order.CommissionPercent,
		order.CommissionAmount.Int64(),
		order.ServiceFeeAmount.Int64(),
		order.TotalPayableAmount.Int64(),
		order.TotalPayableAmount.Int64(), // total_before_coins_amount = total_payable when no coins
		string(order.Status),
		string(order.EscrowStatus),
		order.AutoReleaseAt,
		order.HasDispute,
		order.IdempotencyKey,
		order.ShippingSetupID,
		order.ShippingSetupName,
		order.ShippingTransportType,
		order.PreparationTimeSnapshot,
		order.PreparationNoteSnapshot,
		order.ReadyToShipBy,
		addressSnapshotJSON,
		order.PricingTokenID,
		order.PaymentExpiresAt,
		order.ShippingSource,
		originSnapshotJSON,
		order.ShippingQuoteID,
		order.ShippingQuotePrice,
		order.OrderNumber,
		order.CompletedAt,
		order.CreatedAt,
		order.UpdatedAt,
	)

	if err != nil {
		errStr := err.Error()
		isUnique := db.IsUniqueViolation(err) || strings.Contains(errStr, "23505") || strings.Contains(strings.ToLower(errStr), "duplicate key")
		if isUnique {
			if order.PricingTokenID != nil &&
				(strings.Contains(errStr, "idx_orders_pricing_token_id") || strings.Contains(errStr, "pricing_token_id")) {
				return orderrepository.ErrDuplicatePricingToken
			}
			if order.IdempotencyKey != nil && strings.Contains(errStr, "idx_orders_buyer_idempotency") {
				return orderrepository.ErrDuplicateIdempotencyKey
			}
		}
		return fmt.Errorf("create order failed: %w", err)
	}

	return nil
}

// GetByID retrieves an order without locking (for read-only operations).
func (r *OrderRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Order, error) {
	var id, buyerID, sellerID, sourceID uuid.UUID
	var shippingSetupID sql.NullString // NULLABLE: nil when using shipping quote
	var negotiationID sql.NullString
	var pricingTokenID sql.NullString // Pricing token ID (prevents double-ordering)
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var serviceFeeAmount, totalPayableAmount, totalBeforeCoinsAmount int64
	var status, escrowStatus, sourceType string
	var shippingSetupName, shippingTransportType sql.NullString // NULLABLE in DB
	var trackingNumber sql.NullString
	var proofType sql.NullString
	var shippingProofMedia sql.NullString
	var shippingNote sql.NullString
	var orderNum sql.NullString
	var autoReleaseAt sql.NullTime
	var confirmationExtendedAt sql.NullTime
	var completedAt sql.NullTime
	var hasDispute bool
	var confirmationExtensionUsed bool
	var idempotencyKey sql.NullString
	var preparationTimeSnapshot sql.NullString // NULLABLE in DB
	var preparationNoteSnapshot sql.NullString
	var readyToShipBy sql.NullTime
	var addressSnapshotJSON []byte
	var originSnapshotJSON []byte
	var shippingSourceDB sql.NullString
	var shippingQuoteIDDB sql.NullString
	var shippingQuotePriceDB sql.NullInt64
	var paymentExpiresAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id,
		       source_type, source_id, negotiation_id,
		       quantity, unit_price, subtotal, shipping_total,
		       commission_percent, commission_amount, service_fee_amount, total_payable_amount, total_before_coins_amount,
		       status, escrow_status, auto_release_at, has_dispute,
		       confirmation_extension_used, confirmation_extended_at, idempotency_key,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
	       tracking_number, proof_type, shipping_proof_media, shipping_note,
		       order_number,
		       preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by, address_snapshot,
		       pricing_token_id,
		       payment_expires_at,
		       shipping_source, shipping_origin_snapshot,
		       shipping_quote_id, shipping_quote_price,
		       completed_at, created_at, updated_at
		FROM orders
		WHERE id = $1
	`, orderID).Scan(
		&id, &buyerID, &sellerID,
		&sourceType, &sourceID, &negotiationID,
		&quantity, &unitPrice, &subtotal, &shippingTotal,
		&commissionPercent, &commissionAmount, &serviceFeeAmount, &totalPayableAmount, &totalBeforeCoinsAmount,
		&status, &escrowStatus, &autoReleaseAt, &hasDispute, &confirmationExtensionUsed, &confirmationExtendedAt, &idempotencyKey,
		&shippingSetupID, &shippingSetupName, &shippingTransportType,
		&trackingNumber, &proofType, &shippingProofMedia, &shippingNote,
		&orderNum,
		&preparationTimeSnapshot, &preparationNoteSnapshot, &readyToShipBy, &addressSnapshotJSON,
		&pricingTokenID,
		&paymentExpiresAt,
		&shippingSourceDB, &originSnapshotJSON,
		&shippingQuoteIDDB, &shippingQuotePriceDB,
		&completedAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, fmt.Errorf("get order failed: %w", err)
	}

	// Unmarshal address snapshot from JSONB
	var shippingDestination *addressentity.AddressSnapshot
	if addressSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(addressSnapshotJSON, &snapshot); err == nil {
			shippingDestination = &snapshot
		}
	}

	// Unmarshal shipping origin snapshot from JSONB
	var shippingOrigin *addressentity.AddressSnapshot
	if originSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(originSnapshotJSON, &snapshot); err == nil {
			shippingOrigin = &snapshot
		}
	}

	order := &entity.Order{
		ID:                     id,
		BuyerID:                buyerID,
		SellerID:               sellerID,
		SourceType:             entity.OrderSourceType(sourceType),
		SourceID:               sourceID,
		NegotiationID:          db.ToUUIDPtr(negotiationID),
		PricingTokenID:         db.ToUUIDPtr(pricingTokenID),
		Quantity:               quantity,
		UnitPrice:              money.New(unitPrice),
		Subtotal:               money.New(subtotal),
		ShippingTotal:          money.New(shippingTotal),
		CommissionPercent:      commissionPercent,
		CommissionAmount:       money.New(commissionAmount),
		ServiceFeeAmount:       money.New(serviceFeeAmount),
		TotalPayableAmount:     money.New(totalPayableAmount),
		TotalBeforeCoinsAmount: money.New(totalBeforeCoinsAmount),
		ShippingSetupID:       db.ToUUIDPtr(shippingSetupID),
		ShippingSetupName:     shippingSetupName.String,
		ShippingTransportType:  shippingTransportType.String,
		// Shipping Proof fields
		ProofType:          db.ToStringPtr(proofType),
		TrackingNumber:     db.ToStringPtr(trackingNumber),
		ShippingProofMedia: db.ToStringPtr(shippingProofMedia),
		ShippingNote:       db.ToStringPtr(shippingNote),
		OrderNumber:        db.ToStringPtr(orderNum),
		// Shipping Source + Origin + Quote
		ShippingSource:     db.ToStringPtr(shippingSourceDB),
		ShippingOrigin:     shippingOrigin,
		ShippingQuoteID:    db.ToUUIDPtr(shippingQuoteIDDB),
		ShippingQuotePrice: db.ToInt64Ptr(shippingQuotePriceDB),
		// Shipping Readiness Snapshot
		PreparationTimeSnapshot:   preparationTimeSnapshot.String,
		PreparationNoteSnapshot:   db.ToStringPtr(preparationNoteSnapshot),
		ReadyToShipBy:             db.ToTimePtr(readyToShipBy),
		ShippingDestination:       shippingDestination,
		Status:                    entity.Status(status),
		EscrowStatus:              entity.EscrowStatus(escrowStatus),
		HasDispute:                hasDispute,
		ConfirmationExtensionUsed: confirmationExtensionUsed,
		IdempotencyKey:            db.ToStringPtr(idempotencyKey),
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}

	if autoReleaseAt.Valid {
		ts := autoReleaseAt.Time
		order.AutoReleaseAt = &ts
	}

	if confirmationExtendedAt.Valid {
		ts := confirmationExtendedAt.Time
		order.ConfirmationExtendedAt = &ts
	}

	if completedAt.Valid {
		ts := completedAt.Time
		order.CompletedAt = &ts
	}

	if paymentExpiresAt.Valid {
		order.PaymentExpiresAt = paymentExpiresAt.Time
	}

	return order, nil
}

// GetForUpdate retrieves an order with FOR UPDATE lock.
// This prevents concurrent modifications and must be used within a transaction.
func (r *OrderRepository) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Order, error) {
	var id, buyerID, sellerID, sourceID uuid.UUID
	var shippingSetupID sql.NullString // NULLABLE: nil when using shipping quote
	var negotiationID sql.NullString
	var pricingTokenID sql.NullString // Pricing token ID (prevents double-ordering)
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var serviceFeeAmount, totalPayableAmount, totalBeforeCoinsAmount int64
	var status, escrowStatus, sourceType string
	var shippingSetupName, shippingTransportType sql.NullString // NULLABLE in DB
	var trackingNumber sql.NullString
	var proofType sql.NullString
	var shippingProofMedia sql.NullString
	var shippingNote sql.NullString
	var orderNum sql.NullString
	var autoReleaseAt sql.NullTime
	var confirmationExtendedAt sql.NullTime
	var completedAt sql.NullTime
	var hasDispute bool
	var confirmationExtensionUsed bool
	var idempotencyKey sql.NullString
	var preparationTimeSnapshot sql.NullString // NULLABLE in DB
	var preparationNoteSnapshot sql.NullString
	var readyToShipBy sql.NullTime
	var addressSnapshotJSON []byte
	var originSnapshotJSON []byte
	var shippingSourceDB sql.NullString
	var shippingQuoteIDDB sql.NullString
	var shippingQuotePriceDB sql.NullInt64
	var paymentExpiresAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id,
		       source_type, source_id, negotiation_id,
		       quantity, unit_price, subtotal, shipping_total,
		       commission_percent, commission_amount, service_fee_amount, total_payable_amount, total_before_coins_amount,
		       status, escrow_status, auto_release_at, has_dispute,
		       confirmation_extension_used, confirmation_extended_at, idempotency_key,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
	       tracking_number, proof_type, shipping_proof_media, shipping_note,
		       order_number,
		       preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by, address_snapshot,
		       pricing_token_id,
		       payment_expires_at,
		       shipping_source, shipping_origin_snapshot,
		       shipping_quote_id, shipping_quote_price,
		       completed_at, created_at, updated_at
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(
		&id, &buyerID, &sellerID,
		&sourceType, &sourceID, &negotiationID,
		&quantity, &unitPrice, &subtotal, &shippingTotal,
		&commissionPercent, &commissionAmount, &serviceFeeAmount, &totalPayableAmount, &totalBeforeCoinsAmount,
		&status, &escrowStatus, &autoReleaseAt, &hasDispute, &confirmationExtensionUsed, &confirmationExtendedAt, &idempotencyKey,
		&shippingSetupID, &shippingSetupName, &shippingTransportType,
		&trackingNumber, &proofType, &shippingProofMedia, &shippingNote,
		&orderNum,
		&preparationTimeSnapshot, &preparationNoteSnapshot, &readyToShipBy, &addressSnapshotJSON,
		&pricingTokenID,
		&paymentExpiresAt,
		&shippingSourceDB, &originSnapshotJSON,
		&shippingQuoteIDDB, &shippingQuotePriceDB,
		&completedAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, fmt.Errorf("get order for update failed: %w", err)
	}

	// Unmarshal address snapshot from JSONB
	var shippingDestination *addressentity.AddressSnapshot
	if addressSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(addressSnapshotJSON, &snapshot); err == nil {
			shippingDestination = &snapshot
		}
	}

	// Unmarshal shipping origin snapshot from JSONB
	var shippingOrigin *addressentity.AddressSnapshot
	if originSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(originSnapshotJSON, &snapshot); err == nil {
			shippingOrigin = &snapshot
		}
	}

	order := &entity.Order{
		ID:                     id,
		BuyerID:                buyerID,
		SellerID:               sellerID,
		SourceType:             entity.OrderSourceType(sourceType),
		SourceID:               sourceID,
		NegotiationID:          db.ToUUIDPtr(negotiationID),
		PricingTokenID:         db.ToUUIDPtr(pricingTokenID),
		Quantity:               quantity,
		UnitPrice:              money.New(unitPrice),
		Subtotal:               money.New(subtotal),
		ShippingTotal:          money.New(shippingTotal),
		CommissionPercent:      commissionPercent,
		CommissionAmount:       money.New(commissionAmount),
		ServiceFeeAmount:       money.New(serviceFeeAmount),
		TotalPayableAmount:     money.New(totalPayableAmount),
		TotalBeforeCoinsAmount: money.New(totalBeforeCoinsAmount),
		ShippingSetupID:       db.ToUUIDPtr(shippingSetupID),
		ShippingSetupName:     shippingSetupName.String,
		ShippingTransportType:  shippingTransportType.String,
		// Shipping Proof fields
		ProofType:          db.ToStringPtr(proofType),
		TrackingNumber:     db.ToStringPtr(trackingNumber),
		ShippingProofMedia: db.ToStringPtr(shippingProofMedia),
		ShippingNote:       db.ToStringPtr(shippingNote),
		OrderNumber:        db.ToStringPtr(orderNum),
		// Shipping Source + Origin + Quote
		ShippingSource:     db.ToStringPtr(shippingSourceDB),
		ShippingOrigin:     shippingOrigin,
		ShippingQuoteID:    db.ToUUIDPtr(shippingQuoteIDDB),
		ShippingQuotePrice: db.ToInt64Ptr(shippingQuotePriceDB),
		// Shipping Readiness Snapshot
		PreparationTimeSnapshot:   preparationTimeSnapshot.String,
		PreparationNoteSnapshot:   db.ToStringPtr(preparationNoteSnapshot),
		ReadyToShipBy:             db.ToTimePtr(readyToShipBy),
		ShippingDestination:       shippingDestination,
		Status:                    entity.Status(status),
		EscrowStatus:              entity.EscrowStatus(escrowStatus),
		HasDispute:                hasDispute,
		ConfirmationExtensionUsed: confirmationExtensionUsed,
		IdempotencyKey:            db.ToStringPtr(idempotencyKey),
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}

	if autoReleaseAt.Valid {
		ts := autoReleaseAt.Time
		order.AutoReleaseAt = &ts
	}

	if confirmationExtendedAt.Valid {
		ts := confirmationExtendedAt.Time
		order.ConfirmationExtendedAt = &ts
	}

	if completedAt.Valid {
		ts := completedAt.Time
		order.CompletedAt = &ts
	}

	if paymentExpiresAt.Valid {
		order.PaymentExpiresAt = paymentExpiresAt.Time
	}

	return order, nil
}

// GetByIdempotencyKey retrieves an order by buyer_id and idempotency key without locking.
// Idempotency is scoped per-buyer, so both buyer_id and key are required.
// Returns nil if no order found with the given buyer and key.
//
// Implementation note: this performs a lightweight id lookup and then delegates
// to GetByID for the full hydrated entity, so column/Scan layout stays in one
// place.
func (r *OrderRepository) GetByIdempotencyKey(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	idempotencyKey string,
) (*entity.Order, error) {
	var orderID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM orders
		WHERE buyer_id = $1 AND idempotency_key = $2
	`, buyerID, idempotencyKey).Scan(&orderID)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get order by buyer and idempotency key failed: %w", err)
	}

	return r.GetByID(ctx, tx, orderID)
}

// GetByShippingQuoteID retrieves an order by its shipping quote ID.
// SCENARIO 6 FIX: Prevents double-order attacks from same shipping quote.
// Returns nil if no order found with the given shipping quote.
func (r *OrderRepository) GetByShippingQuoteID(
	ctx context.Context,
	tx db.Tx,
	shippingQuoteID uuid.UUID,
) (*entity.Order, error) {
	var id, buyerID, sellerID, sourceID uuid.UUID
	var shippingSetupID *uuid.UUID // NULLABLE: nil when using shipping quote
	var negotiationID *uuid.UUID
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var serviceFeeAmount, totalPayableAmount int64
	var status, escrowStatus, sourceType string
	var shippingSetupName, shippingTransportType sql.NullString // NULLABLE in DB
	var trackingNumber, shippingNote *string
	var orderNum *string
	var autoReleaseAt *time.Time
	var confirmationExtendedAt *time.Time
	var completedAt *int64
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
	var createdAt, updatedAt int64

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id,
		       source_type, source_id, negotiation_id,
		       quantity, unit_price, subtotal, shipping_total, commission_percent,
		       commission_amount, service_fee_amount, total_payable_amount, status, escrow_status,
		       auto_release_at, has_dispute, confirmation_extension_used, idempotency_key,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
	       tracking_number, shipping_note, order_number,
		       preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by,
		       shipping_destination,
		       shipping_source, shipping_origin_snapshot,
		       shipping_quote_id, shipping_quote_price,
		       created_at, updated_at
		FROM orders
		WHERE shipping_quote_id = $1
	`, shippingQuoteID).Scan(
		&id, &buyerID, &sellerID,
		&sourceType, &sourceID, &negotiationID,
		&quantity, &unitPrice, &subtotal, &shippingTotal, &commissionPercent,
		&commissionAmount, &serviceFeeAmount, &totalPayableAmount, &status, &escrowStatus,
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
			return nil, nil // No order with this shipping quote
		}
		return nil, fmt.Errorf("get order by shipping quote id failed: %w", err)
	}

	// Unmarshal address snapshot from JSONB
	var shippingDestination *addressentity.AddressSnapshot
	if addressSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(addressSnapshotJSON, &snapshot); err == nil {
			shippingDestination = &snapshot
		}
	}

	// Unmarshal shipping origin snapshot from JSONB
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
		ServiceFeeAmount:          money.New(serviceFeeAmount),
		TotalPayableAmount:        money.New(totalPayableAmount),
		ShippingSetupID:          shippingSetupID,
		ShippingSetupName:        shippingSetupName.String,
		ShippingTransportType:     shippingTransportType.String,
		TrackingNumber:            trackingNumber,
		ShippingNote:              nil,
		OrderNumber:               orderNum,
		ShippingSource:            shippingSourcePtr,
		ShippingOrigin:            shippingOrigin,
		ShippingQuoteID:           shippingQuoteIDPtr,
		ShippingQuotePrice:        shippingQuotePricePtr,
		PreparationTimeSnapshot:   preparationTimeSnapshot.String,
		PreparationNoteSnapshot:   preparationNoteSnapshot,
		ReadyToShipBy:             readyToShipBy,
		ShippingDestination:       shippingDestination,
		Status:                    entity.Status(status),
		EscrowStatus:              entity.EscrowStatus(escrowStatus),
		HasDispute:                hasDispute,
		ConfirmationExtensionUsed: confirmationExtensionUsed,
		IdempotencyKey:            idempotencyKeyPtr,
		CreatedAt:                 time.Unix(createdAt, 0),
		UpdatedAt:                 time.Unix(updatedAt, 0),
	}

	if autoReleaseAt != nil {
		ts := *autoReleaseAt
		order.AutoReleaseAt = &ts
	}

	if confirmationExtendedAt != nil {
		ts := *confirmationExtendedAt
		order.ConfirmationExtendedAt = &ts
	}

	if completedAt != nil {
		ts := time.Unix(*completedAt, 0)
		order.CompletedAt = &ts
	}

	return order, nil
}

// CountValidOrdersByShippingQuoteID counts orders in quote-blocking statuses for a shipping quote.
// Used for shipping quote reactivation validation to prevent duplicate orders.
func (r *OrderRepository) CountValidOrdersByShippingQuoteID(
	ctx context.Context,
	tx db.Tx,
	shippingQuoteID uuid.UUID,
) (int64, error) {
	var count int64

	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE shipping_quote_id = $1
		  AND status = ANY($2::text[])
	`, shippingQuoteID, quoteBlockingOrderStatuses()).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("count valid orders by shipping quote id failed: %w", err)
	}

	return count, nil
}

// GetByPricingTokenID retrieves an order by its pricing token ID.
func (r *OrderRepository) GetByPricingTokenID(
	ctx context.Context,
	tx db.Tx,
	pricingTokenID uuid.UUID,
) (*entity.Order, error) {
	var id, buyerID, sellerID, sourceID uuid.UUID
	var shippingSetupID sql.NullString
	var negotiationID sql.NullString
	var storedPricingTokenID sql.NullString
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var serviceFeeAmount, totalPayableAmount int64
	var status, escrowStatus, sourceType string
	var shippingSetupName, shippingTransportType sql.NullString // NULLABLE in DB
	var trackingNumber sql.NullString
	var proofType sql.NullString
	var shippingProofMedia sql.NullString
	var shippingNote sql.NullString
	var orderNum sql.NullString
	var autoReleaseAt sql.NullTime
	var confirmationExtendedAt sql.NullTime
	var completedAt sql.NullTime
	var hasDispute bool
	var confirmationExtensionUsed bool
	var idempotencyKey sql.NullString
	var preparationTimeSnapshot sql.NullString // NULLABLE in DB
	var preparationNoteSnapshot sql.NullString
	var readyToShipBy sql.NullTime
	var addressSnapshotJSON []byte
	var createdAt, updatedAt time.Time

	var shippingSourceDB sql.NullString
	var originSnapshotJSON []byte
	var shippingQuoteIDDB sql.NullString
	var shippingQuotePriceDB sql.NullInt64

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id,
		       source_type, source_id, negotiation_id,
		       quantity, unit_price, subtotal, shipping_total,
		       commission_percent, commission_amount, service_fee_amount, total_payable_amount,
		       status, escrow_status, auto_release_at, has_dispute,
		       confirmation_extension_used, confirmation_extended_at, idempotency_key,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
	       tracking_number, proof_type, shipping_proof_media, shipping_note,
		       order_number,
		       preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by, address_snapshot,
		       pricing_token_id,
		       shipping_source, shipping_origin_snapshot,
		       shipping_quote_id, shipping_quote_price,
		       completed_at, created_at, updated_at
		FROM orders
		WHERE pricing_token_id = $1
	`, pricingTokenID).Scan(
		&id, &buyerID, &sellerID,
		&sourceType, &sourceID, &negotiationID,
		&quantity, &unitPrice, &subtotal, &shippingTotal,
		&commissionPercent, &commissionAmount, &serviceFeeAmount, &totalPayableAmount,
		&status, &escrowStatus, &autoReleaseAt, &hasDispute, &confirmationExtensionUsed, &confirmationExtendedAt, &idempotencyKey,
		&shippingSetupID, &shippingSetupName, &shippingTransportType,
		&trackingNumber, &proofType, &shippingProofMedia, &shippingNote,
		&orderNum,
		&preparationTimeSnapshot, &preparationNoteSnapshot, &readyToShipBy, &addressSnapshotJSON,
		&storedPricingTokenID,
		&shippingSourceDB, &originSnapshotJSON,
		&shippingQuoteIDDB, &shippingQuotePriceDB,
		&completedAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get order by pricing token id failed: %w", err)
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
		NegotiationID:             db.ToUUIDPtr(negotiationID),
		PricingTokenID:            db.ToUUIDPtr(storedPricingTokenID),
		Quantity:                  quantity,
		UnitPrice:                 money.New(unitPrice),
		Subtotal:                  money.New(subtotal),
		ShippingTotal:             money.New(shippingTotal),
		CommissionPercent:         commissionPercent,
		CommissionAmount:          money.New(commissionAmount),
		ServiceFeeAmount:          money.New(serviceFeeAmount),
		TotalPayableAmount:        money.New(totalPayableAmount),
		ShippingSetupID:          db.ToUUIDPtr(shippingSetupID),
		ShippingSetupName:        shippingSetupName.String,
		ShippingTransportType:     shippingTransportType.String,

		ProofType:                 db.ToStringPtr(proofType),
		TrackingNumber:            db.ToStringPtr(trackingNumber),
		ShippingProofMedia:        db.ToStringPtr(shippingProofMedia),
		ShippingNote:              db.ToStringPtr(shippingNote),
		OrderNumber:               db.ToStringPtr(orderNum),
		ShippingSource:            db.ToStringPtr(shippingSourceDB),
		ShippingOrigin:            shippingOrigin,
		ShippingQuoteID:           db.ToUUIDPtr(shippingQuoteIDDB),
		ShippingQuotePrice:        db.ToInt64Ptr(shippingQuotePriceDB),
		PreparationTimeSnapshot:   preparationTimeSnapshot.String,
		PreparationNoteSnapshot:   db.ToStringPtr(preparationNoteSnapshot),
		ReadyToShipBy:             db.ToTimePtr(readyToShipBy),
		ShippingDestination:       shippingDestination,
		Status:                    entity.Status(status),
		EscrowStatus:              entity.EscrowStatus(escrowStatus),
		HasDispute:                hasDispute,
		ConfirmationExtensionUsed: confirmationExtensionUsed,
		IdempotencyKey:            db.ToStringPtr(idempotencyKey),
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}

	if autoReleaseAt.Valid {
		ts := autoReleaseAt.Time
		order.AutoReleaseAt = &ts
	}

	if confirmationExtendedAt.Valid {
		ts := confirmationExtendedAt.Time
		order.ConfirmationExtendedAt = &ts
	}

	if completedAt.Valid {
		ts := completedAt.Time
		order.CompletedAt = &ts
	}

	return order, nil
}

// CreateOrderItemTx persists an order item within a transaction.
// This is called after CreateOrderTx to add line items to the order.
func (r *OrderRepository) CreateOrderItemTx(
	ctx context.Context,
	tx db.Tx,
	orderItem *entity.OrderItem,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO order_items (
			id, order_id, product_id, name, unit_price_snapshot, quantity, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		orderItem.ID,
		orderItem.OrderID,
		orderItem.ProductID,
		orderItem.Name,
		orderItem.UnitPriceSnapshot.Int64(),
		orderItem.Quantity,
		orderItem.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create order item failed: %w", err)
	}

	return nil
}

// UpdateStatusTx persists order status and escrow status changes.
// Also persists shipping proof information when order is marked as shipped.
//
// IMMUTABILITY: Shipping proof fields (proof_type, tracking_number, shipping_proof_media)
// are immutable once the order status is 'shipped' or later. The SQL CASE statement
// prevents modification of these fields after shipping.
func (r *OrderRepository) UpdateStatusTx(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders
		SET status = $2,
		    escrow_status = $3,
		    auto_release_at = $4,
		    has_dispute = $5,
		    confirmation_extension_used = $6,
		    confirmation_extended_at = $7,
		    proof_type = CASE WHEN status IN ('shipped', 'delivered', 'completed', 'dispute_open', 'partially_refunded')
		                     THEN proof_type  -- Immutable: keep existing value
		                     ELSE $8 END,     -- Allow update only if not shipped yet
		    tracking_number = CASE WHEN status IN ('shipped', 'delivered', 'completed', 'dispute_open', 'partially_refunded')
		                         THEN tracking_number  -- Immutable: keep existing value
		                         ELSE $9 END,          -- Allow update only if not shipped yet
		    shipping_proof_media = CASE WHEN status IN ('shipped', 'delivered', 'completed', 'dispute_open', 'partially_refunded')
		                           THEN shipping_proof_media  -- Immutable: keep existing value
		                           ELSE $10 END,             -- Allow update only if not shipped yet
		    shipping_note = $11,
		    completed_at = $12,
		    updated_at = $13
		WHERE id = $1
	`,
		order.ID,
		string(order.Status),
		string(order.EscrowStatus),
		order.AutoReleaseAt,
		order.HasDispute,
		order.ConfirmationExtensionUsed,
		order.ConfirmationExtendedAt,
		order.ProofType,
		order.TrackingNumber,
		order.ShippingProofMedia,
		order.ShippingNote,
		order.CompletedAt,
		order.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update order status failed: %w", err)
	}

	return nil
}

// UpdatePaymentSelectionTx persists the already-calculated buyer-facing order
// projections once CorePaymentHandler.CreatePayment has resolved the payment
// method and fee. The handler computes the canonical payment values first;
// this repository method only stores the projection rows for
// service_fee_amount and total_payable_amount.
func (r *OrderRepository) UpdatePaymentSelectionTx(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	serviceFeeAmount money.Money,
	totalPayableAmount money.Money,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders
		SET service_fee_amount = $2,
		    total_payable_amount = $3,
		    updated_at = NOW()
		WHERE id = $1
	`,
		orderID,
		serviceFeeAmount.Int64(),
		totalPayableAmount.Int64(),
	)
	if err != nil {
		return fmt.Errorf("update order payment selection failed: %w", err)
	}
	return nil
}

// FindOrdersForAutoComplete returns IDs of orders that are due for auto-completion.
// Uses FOR UPDATE SKIP LOCKED to support concurrent workers.
//
// BUSINESS RULE: Timer starts at SHIPPED, so both shipped and delivered orders can auto-complete.
// The timer-based auto-complete is the source of truth, not the delivery status.
//
// CRITICAL SAFETY: Query excludes disputed orders AND orders with active refunds
// at the database level to prevent race conditions.
//
// Query conditions:
// - status IN ('shipped', 'delivered') - timer starts at shipped
// - escrow_status = 'holding'
// - has_dispute = false (CRITICAL - prevents auto-completing disputed orders)
// - no active refund (H2-F2a - prevents releasing escrow while refund in flight)
// - auto_release_at <= NOW()
func (r *OrderRepository) FindOrdersForAutoComplete(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	// Use FOR UPDATE SKIP LOCKED to allow concurrent workers to process different orders
	// Each worker locks a subset of orders, preventing duplicate processing
	//
	// H2-F2a: NOT EXISTS subquery excludes orders where a refund row exists in
	// a non-terminal status. Terminal statuses (refunded, admin_released) do NOT
	// block because the money movement is already complete.
	query := `
		SELECT id
		FROM orders o
		WHERE o.status IN ('shipped', 'delivered')
		  AND o.escrow_status = 'holding'
		  AND o.has_dispute = false
		  AND o.auto_release_at <= NOW()
		  AND NOT EXISTS (
		      SELECT 1 FROM refunds r
		      WHERE r.order_id = o.id
		        AND r.status NOT IN ('refunded', 'admin_released')
		  )
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("find orders for auto-complete failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan order id failed: %w", err)
		}
		orderIDs = append(orderIDs, id)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate order ids failed: %w", rows.Err())
	}

	return orderIDs, nil
}

// FindOverdueOrdersForCancel returns IDs of orders that are overdue for shipment.
// Uses FOR UPDATE SKIP LOCKED to support concurrent workers.
//
// 🔥 PHASE 2: AUTO-CANCEL (CRITICAL)
//
// This method finds orders that should be auto-cancelled due to shipment timeout.
//
// Query conditions:
// - status = 'paid' (only paid orders that haven't shipped)
// - escrow_status = 'holding' (only orders with escrow held)
// - ready_to_ship_by IS NOT NULL (deadline is set)
// - ready_to_ship_by + INTERVAL '2 days' < NOW() (grace period exceeded)
//
// The grace period (2 days) is defined by entity.FulfillmentGracePeriodDays
func (r *OrderRepository) FindOverdueOrdersForCancel(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	// Use FOR UPDATE SKIP LOCKED to allow concurrent workers to process different orders
	// Each worker locks a subset of orders, preventing duplicate processing
	query := `
			SELECT id
			FROM orders
			WHERE status = 'paid'
			  AND escrow_status = 'holding'
			  AND ready_to_ship_by IS NOT NULL
			  AND ready_to_ship_by + (INTERVAL '1 day' * 2) < NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("find overdue orders for cancel failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan order id failed: %w", err)
		}
		orderIDs = append(orderIDs, id)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate order ids failed: %w", rows.Err())
	}

	return orderIDs, nil
}

// CreateShippingProofTx creates a shipping proof within a transaction.
func (r *OrderRepository) CreateShippingProofTx(
	ctx context.Context,
	tx db.Tx,
	proof *entity.ShippingProof,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO shipping_proofs (
			id, order_id, seller_id, media_url, created_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, proof.ID, proof.OrderID, proof.SellerID, proof.MediaURL, proof.CreatedAt)

	if err != nil {
		return fmt.Errorf("create shipping proof failed: %w", err)
	}

	return nil
}

// GetShippingProofsByOrderID retrieves all shipping proofs for an order.
func (r *OrderRepository) GetShippingProofsByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) ([]*entity.ShippingProof, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, order_id, seller_id, media_url, created_at
		FROM shipping_proofs
		WHERE order_id = $1
		ORDER BY created_at DESC
	`, orderID)

	if err != nil {
		return nil, fmt.Errorf("get shipping proofs failed: %w", err)
	}
	defer rows.Close()

	var proofs []*entity.ShippingProof
	for rows.Next() {
		var proof entity.ShippingProof
		err := rows.Scan(
			&proof.ID,
			&proof.OrderID,
			&proof.SellerID,
			&proof.MediaURL,
			&proof.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan shipping proof failed: %w", err)
		}
		proofs = append(proofs, &proof)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate shipping proofs failed: %w", rows.Err())
	}

	return proofs, nil
}

// GetByOrderNumber retrieves an order by its human-readable order number.
func (r *OrderRepository) GetByOrderNumber(
	ctx context.Context,
	tx db.Tx,
	orderNumber string,
) (*entity.Order, error) {
	var id, buyerID, sellerID, sourceID uuid.UUID
	var shippingSetupID *uuid.UUID // NULLABLE: nil when using shipping quote
	var negotiationID *uuid.UUID
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var serviceFeeAmount, totalPayableAmount int64
	var status, escrowStatus, sourceType string
	var shippingSetupName, shippingTransportType sql.NullString // NULLABLE in DB
	var trackingNumber, shippingNote *string
	var orderNum *string
	var autoReleaseAt *time.Time
	var confirmationExtendedAt *time.Time
	var completedAt *int64
	var hasDispute bool
	var confirmationExtensionUsed bool
	var idempotencyKey *string
	var preparationTimeSnapshot sql.NullString // NULLABLE in DB
	var preparationNoteSnapshot *string
	var readyToShipBy *time.Time
	var addressSnapshotJSON []byte
	var originSnapshotJSON []byte
	var shippingSourcePtr *string
	var shippingQuoteIDPtr *uuid.UUID
	var shippingQuotePricePtr *int64
	var createdAt, updatedAt int64

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id,
		       source_type, source_id, negotiation_id,
		       quantity, unit_price, subtotal, shipping_total,
		       commission_percent, commission_amount, service_fee_amount, total_payable_amount,
		       status, escrow_status, auto_release_at, has_dispute,
		       confirmation_extension_used, confirmation_extended_at, idempotency_key,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
		       tracking_number, shipping_note,
		       order_number,
		       preparation_time_snapshot, preparation_note_snapshot, ready_to_ship_by, address_snapshot,
		       shipping_source, shipping_origin_snapshot,
		       shipping_quote_id, shipping_quote_price,
		       created_at, updated_at
		FROM orders
		WHERE order_number = $1
	`, orderNumber).Scan(
		&id, &buyerID, &sellerID,
		&sourceType, &sourceID, &negotiationID,
		&quantity, &unitPrice, &subtotal, &shippingTotal,
		&commissionPercent, &commissionAmount, &serviceFeeAmount, &totalPayableAmount,
		&status, &escrowStatus, &autoReleaseAt, &hasDispute, &confirmationExtensionUsed, &confirmationExtendedAt, &idempotencyKey,
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
			return nil, fmt.Errorf("order not found: %s", orderNumber)
		}
		return nil, fmt.Errorf("get order by order number failed: %w", err)
	}

	// Unmarshal address snapshot from JSONB
	var shippingDestination *addressentity.AddressSnapshot
	if addressSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(addressSnapshotJSON, &snapshot); err == nil {
			shippingDestination = &snapshot
		}
	}

	// Unmarshal shipping origin snapshot from JSONB
	var shippingOrigin *addressentity.AddressSnapshot
	if originSnapshotJSON != nil {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(originSnapshotJSON, &snapshot); err == nil {
			shippingOrigin = &snapshot
		}
	}

	order := &entity.Order{
		ID:                     id,
		BuyerID:                buyerID,
		SellerID:               sellerID,
		SourceType:             entity.OrderSourceType(sourceType),
		SourceID:               sourceID,
		NegotiationID:          negotiationID,
		Quantity:               quantity,
		UnitPrice:              money.New(unitPrice),
		Subtotal:               money.New(subtotal),
		ShippingTotal:          money.New(shippingTotal),
		CommissionPercent:      commissionPercent,
		CommissionAmount:       money.New(commissionAmount),
		ServiceFeeAmount:       money.New(serviceFeeAmount),
		TotalPayableAmount:     money.New(totalPayableAmount),
		ShippingSetupID:       shippingSetupID,
		ShippingSetupName:     shippingSetupName.String,
		ShippingTransportType:  shippingTransportType.String,
		// Shipping Confirmation (canonical fields)
		TrackingNumber: trackingNumber,
		ShippingNote:   shippingNote,
		OrderNumber:    orderNum,
		// Shipping Source + Origin + Quote
		ShippingSource:     shippingSourcePtr,
		ShippingOrigin:     shippingOrigin,
		ShippingQuoteID:    shippingQuoteIDPtr,
		ShippingQuotePrice: shippingQuotePricePtr,
		// Shipping Readiness Snapshot
		PreparationTimeSnapshot:   preparationTimeSnapshot.String,
		PreparationNoteSnapshot:   preparationNoteSnapshot,
		ReadyToShipBy:             readyToShipBy,
		ShippingDestination:       shippingDestination,
		Status:                    entity.Status(status),
		EscrowStatus:              entity.EscrowStatus(escrowStatus),
		HasDispute:                hasDispute,
		ConfirmationExtensionUsed: confirmationExtensionUsed,
		IdempotencyKey:            idempotencyKey,
		CreatedAt:                 time.Unix(createdAt, 0),
		UpdatedAt:                 time.Unix(updatedAt, 0),
	}

	if autoReleaseAt != nil {
		ts := *autoReleaseAt
		order.AutoReleaseAt = &ts
	}

	if confirmationExtendedAt != nil {
		ts := *confirmationExtendedAt
		order.ConfirmationExtendedAt = &ts
	}

	if completedAt != nil {
		ts := time.Unix(*completedAt, 0)
		order.CompletedAt = &ts
	}

	return order, nil
}
