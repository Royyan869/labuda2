package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	discountentity "github.com/labuda/backend/internal/pricing/discount/entity"
	"github.com/labuda/backend/internal/pricing/token/entity"
	pricingtokenrepo "github.com/labuda/backend/internal/pricing/token/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/shopspring/decimal"
)

// PricingTokenRepositoryImpl handles pricing token persistence using pgx-based DB layer.
type PricingTokenRepositoryImpl struct{}

// NewPricingTokenRepository creates a new PricingTokenRepository.
func NewPricingTokenRepository() pricingtokenrepo.PricingTokenRepository {
	return &PricingTokenRepositoryImpl{}
}

// CreateTx persists a new pricing token within a transaction.
func (r *PricingTokenRepositoryImpl) CreateTx(
	ctx context.Context,
	tx db.Tx,
	token *entity.PricingToken,
) error {
	// Convert optional fields to database-compatible types
	var discountCode *string
	var discountType *string
	var discountValue *decimal.Decimal
	if token.DiscountCode != nil && *token.DiscountCode != "" {
		discountCode = token.DiscountCode
		if token.DiscountType != nil {
			dt := string(*token.DiscountType)
			discountType = &dt
		}
		discountValue = token.DiscountValue
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO pricing_tokens (
			id, token, user_id, product_id, source_type, source_id, quantity,
			unit_price, subtotal, shipping_total,
			commission_percent, commission_amount, escrow_amount,
			service_fee_amount, total_payable_amount,
			discount_code, discount_type, discount_value, discount_amount,
			shipping_option_id, shipping_option_name, shipping_transport_type,
			address_id, address_snapshot,
			negotiation_id, auction_id,
			coins_used, max_coins_allowed, order_value_for_coins,
			is_used, used_at, order_id,
			expires_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
		        $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35)
	`,
		token.ID,
		token.Token,
		token.UserID,
		token.ProductID,
		token.SourceType,
		token.SourceID,
		token.Quantity,
		token.UnitPrice.Int64(),
		token.Subtotal.Int64(),
		token.ShippingTotal.Int64(),
		token.CommissionPercent,
		token.CommissionAmount.Int64(),
		token.EscrowAmount.Int64(),
		token.ServiceFeeAmount.Int64(),
		token.TotalPayableAmount.Int64(),
		discountCode,
		discountType,
		discountValue,
		token.DiscountAmount.Int64(),
		token.ShippingSetupID,
		token.ShippingSetupName,
		token.ShippingTransportType,
		token.AddressID,
		token.AddressSnapshot,
		token.NegotiationID,
		token.AuctionID,
		token.CoinsUsed,
		token.MaxCoinsAllowed,
		token.OrderValueForCoins,
		token.IsUsed,
		token.UsedAt,
		token.OrderID,
		token.ExpiresAt,
		token.CreatedAt,
		token.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create pricing token failed: %w", err)
	}

	return nil
}

// GetByToken retrieves a pricing token by its token UUID.
func (r *PricingTokenRepositoryImpl) GetByToken(
	ctx context.Context,
	tx db.Tx,
	token uuid.UUID,
) (*entity.PricingToken, error) {
	var id, tokenValue, userID, productID, shippingSetupID, addressID uuid.UUID
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var escrowAmount, serviceFeeAmount, totalPayableAmount, discountAmount int64
	var coinsUsed, maxCoinsAllowed, orderValueForCoins int64
	var tokenSourceType string
	var tokenSourceID uuid.UUID
	var discountCode *string
	var discountTypeStr *string
	var discountValueStr *string
	var shippingSetupName, shippingTransportType string
	var negotiationID, auctionID *uuid.UUID
	var isUsed bool
	var usedAt *time.Time
	var orderID *uuid.UUID
	var expiresAt, createdAt, updatedAt time.Time
	var addressSnapshot []byte

	err := tx.QueryRow(ctx, `
		SELECT id, token, user_id, product_id, source_type, source_id, quantity,
		       unit_price, subtotal, shipping_total,
		       commission_percent, commission_amount, escrow_amount,
		       service_fee_amount, total_payable_amount,
		       discount_code, discount_type, discount_value, discount_amount,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
		       address_id, address_snapshot,
		       negotiation_id, auction_id,
		       coins_used, max_coins_allowed, order_value_for_coins,
		       is_used, used_at, order_id,
		       expires_at, created_at, updated_at
		FROM pricing_tokens
		WHERE token = $1
	`, token).Scan(
		&id, &tokenValue, &userID, &productID, &tokenSourceType, &tokenSourceID, &quantity,
		&unitPrice, &subtotal, &shippingTotal,
		&commissionPercent, &commissionAmount, &escrowAmount,
		&serviceFeeAmount, &totalPayableAmount,
		&discountCode, &discountTypeStr, &discountValueStr, &discountAmount,
		&shippingSetupID, &shippingSetupName, &shippingTransportType,
		&addressID, &addressSnapshot,
		&negotiationID, &auctionID,
		&coinsUsed, &maxCoinsAllowed, &orderValueForCoins,
		&isUsed, &usedAt, &orderID,
		&expiresAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, entity.ErrTokenNotFound
		}
		return nil, fmt.Errorf("get pricing token failed: %w", err)
	}

	// Parse optional discount fields
	var discountType *discountentity.DiscountType
	if discountTypeStr != nil {
		dt := discountentity.DiscountType(*discountTypeStr)
		discountType = &dt
	}

	var discountValue *decimal.Decimal
	if discountValueStr != nil {
		dv, _ := decimal.NewFromString(*discountValueStr)
		discountValue = &dv
	}

	return &entity.PricingToken{
		ID:                     id,
		Token:                  tokenValue,
		UserID:                 userID,
		ProductID:              productID,
		SourceType:             tokenSourceType,
		SourceID:               tokenSourceID,
		NegotiationID:          negotiationID,
		AuctionID:              auctionID,
		Quantity:               quantity,
		UnitPrice:              money.New(unitPrice),
		Subtotal:               money.New(subtotal),
		ShippingTotal:          money.New(shippingTotal),
		CommissionPercent:      commissionPercent,
		CommissionAmount:       money.New(commissionAmount),
		EscrowAmount:           money.New(escrowAmount),
		ServiceFeeAmount:       money.New(serviceFeeAmount),
		TotalPayableAmount:     money.New(totalPayableAmount),
		DiscountCode:           discountCode,
		DiscountType:           discountType,
		DiscountValue:          discountValue,
		DiscountAmount:         money.New(discountAmount),
		CoinsUsed:              coinsUsed,
		MaxCoinsAllowed:        maxCoinsAllowed,
		OrderValueForCoins:     orderValueForCoins,
		ShippingSetupID:      shippingSetupID,
		ShippingSetupName:    shippingSetupName,
		ShippingTransportType: shippingTransportType,
		AddressID:             addressID,
		AddressSnapshot:       addressSnapshot,
		IsUsed:                isUsed,
		UsedAt:                usedAt,
		OrderID:               orderID,
		ExpiresAt:             expiresAt,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}, nil
}

// GetByTokenForUpdate retrieves a pricing token with FOR UPDATE lock.
func (r *PricingTokenRepositoryImpl) GetByTokenForUpdate(
	ctx context.Context,
	tx db.Tx,
	token uuid.UUID,
) (*entity.PricingToken, error) {
	var id, tokenValue, userID, productID, shippingSetupID, addressID uuid.UUID
	var quantity int
	var unitPrice, subtotal, shippingTotal, commissionPercent, commissionAmount int64
	var escrowAmount, serviceFeeAmount, totalPayableAmount, discountAmount int64
	var coinsUsed, maxCoinsAllowed, orderValueForCoins int64
	var tokenSourceType string
	var tokenSourceID uuid.UUID
	var discountCode *string
	var discountTypeStr *string
	var discountValueStr *string
	var shippingSetupName, shippingTransportType string
	var negotiationID, auctionID *uuid.UUID
	var isUsed bool
	var usedAt *time.Time
	var orderID *uuid.UUID
	var expiresAt, createdAt, updatedAt time.Time
	var addressSnapshot []byte

	err := tx.QueryRow(ctx, `
		SELECT id, token, user_id, product_id, source_type, source_id, quantity,
		       unit_price, subtotal, shipping_total,
		       commission_percent, commission_amount, escrow_amount,
		       service_fee_amount, total_payable_amount,
		       discount_code, discount_type, discount_value, discount_amount,
		       shipping_option_id, shipping_option_name, shipping_transport_type,
		       address_id, address_snapshot,
		       negotiation_id, auction_id,
		       coins_used, max_coins_allowed, order_value_for_coins,
		       is_used, used_at, order_id,
		       expires_at, created_at, updated_at
		FROM pricing_tokens
		WHERE token = $1
		FOR UPDATE
	`, token).Scan(
		&id, &tokenValue, &userID, &productID, &tokenSourceType, &tokenSourceID, &quantity,
		&unitPrice, &subtotal, &shippingTotal,
		&commissionPercent, &commissionAmount, &escrowAmount,
		&serviceFeeAmount, &totalPayableAmount,
		&discountCode, &discountTypeStr, &discountValueStr, &discountAmount,
		&shippingSetupID, &shippingSetupName, &shippingTransportType,
		&addressID, &addressSnapshot,
		&negotiationID, &auctionID,
		&coinsUsed, &maxCoinsAllowed, &orderValueForCoins,
		&isUsed, &usedAt, &orderID,
		&expiresAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, entity.ErrTokenNotFound
		}
		return nil, fmt.Errorf("get pricing token for update failed: %w", err)
	}

	// Parse optional discount fields
	var discountType *discountentity.DiscountType
	if discountTypeStr != nil {
		dt := discountentity.DiscountType(*discountTypeStr)
		discountType = &dt
	}

	var discountValue *decimal.Decimal
	if discountValueStr != nil {
		dv, _ := decimal.NewFromString(*discountValueStr)
		discountValue = &dv
	}

	return &entity.PricingToken{
		ID:                     id,
		Token:                  tokenValue,
		UserID:                 userID,
		ProductID:              productID,
		SourceType:             tokenSourceType,
		SourceID:               tokenSourceID,
		NegotiationID:          negotiationID,
		AuctionID:              auctionID,
		Quantity:               quantity,
		UnitPrice:              money.New(unitPrice),
		Subtotal:               money.New(subtotal),
		ShippingTotal:          money.New(shippingTotal),
		CommissionPercent:      commissionPercent,
		CommissionAmount:       money.New(commissionAmount),
		EscrowAmount:           money.New(escrowAmount),
		ServiceFeeAmount:       money.New(serviceFeeAmount),
		TotalPayableAmount:     money.New(totalPayableAmount),
		DiscountCode:           discountCode,
		DiscountType:           discountType,
		DiscountValue:          discountValue,
		DiscountAmount:         money.New(discountAmount),
		CoinsUsed:              coinsUsed,
		MaxCoinsAllowed:        maxCoinsAllowed,
		OrderValueForCoins:     orderValueForCoins,
		ShippingSetupID:      shippingSetupID,
		ShippingSetupName:    shippingSetupName,
		ShippingTransportType: shippingTransportType,
		AddressID:             addressID,
		AddressSnapshot:        addressSnapshot,
		IsUsed:                 isUsed,
		UsedAt:                 usedAt,
		OrderID:                orderID,
		ExpiresAt:              expiresAt,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}

// MarkAsUsedTx marks a token as used and links it to an order within a transaction.
func (r *PricingTokenRepositoryImpl) MarkAsUsedTx(
	ctx context.Context,
	tx db.Tx,
	tokenID uuid.UUID,
	orderID uuid.UUID,
) error {
	result, err := tx.Exec(ctx, `
		UPDATE pricing_tokens
		SET is_used = true,
		    used_at = NOW(),
		    order_id = $2,
		    updated_at = NOW()
		WHERE id = $1 AND is_used = false
	`, tokenID, orderID)

	if err != nil {
		return fmt.Errorf("mark pricing token as used failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrTokenAlreadyUsed
	}

	return nil
}

// DeleteExpiredTokensTx deletes expired tokens that are older than the specified duration.
func (r *PricingTokenRepositoryImpl) DeleteExpiredTokensTx(
	ctx context.Context,
	tx db.Tx,
	olderThan string,
) (int64, error) {
	result, err := tx.Exec(ctx, `
		DELETE FROM pricing_tokens
		WHERE expires_at < NOW() - INTERVAL '`+olderThan+`'
	`)

	if err != nil {
		return 0, fmt.Errorf("delete expired pricing tokens failed: %w", err)
	}

	return result.RowsAffected(), nil
}


