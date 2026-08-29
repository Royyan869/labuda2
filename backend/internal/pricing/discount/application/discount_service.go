// ============================================================================
// DISCOUNT SERVICE - DISCOUNT VALIDATION SOURCE OF TRUTH
// ============================================================================
//
// CANONICAL MODEL (DISCOUNT-003):
// - Discount is seller-funded and seller-created
// - Discount applicability is by SELLING SURFACE ONLY (for_sale / auction / both)
// - No specific item/surface targeting
// - Discount types: percentage, flat_amount
// - Validity: expiry-only (valid_until)
// - Usage: optional total_usage_limit (0 = unlimited)
// - Minimum purchase: optional min_purchase against final transaction price P
// - Anyone who knows the code may attempt to use it
// - PricingToken is the sole transaction-pricing authority
// - Discount does NOT modify forSale.price or auction price
// - Discount is ONLY calculated at checkout time, server-side
// - Discount applies to the FINAL TRANSACTION PRICE, not starting/reference price
//
// BUSINESS RULES:
// - ECONOMIC SAFETY: Discount percentage capped at 50% (MaxDiscountPercentage)
// - COMMISSION SAFETY: final_order_value >= commission_amount (enforced at PricingToken layer)
// - DUPLICATE PREVENTION: One discount per order (database constraint)
// - USAGE LIMITS: Total usage limit enforced
// - MIN PURCHASE: Evaluated against P (final product transaction price)
//
// NO DISCOUNT LOGIC SHALL EXIST OUTSIDE THIS SERVICE.
// ============================================================================
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/discount/entity"
	repositoryImpl "github.com/labuda/backend/internal/pricing/discount/infrastructure/repository"
	discountRepo "github.com/labuda/backend/internal/pricing/discount/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/shopspring/decimal"
)

// DiscountService handles discount operations.
type DiscountService struct {
	repo discountRepo.DiscountRepository
}

// NewDiscountService creates a NewDiscountService.
func NewDiscountService() *DiscountService {
	return &DiscountService{
		repo: repositoryImpl.NewDiscountRepository(),
	}
}

// ============================================================================
// CREATE
// ============================================================================

// CreateDiscountInput contains parameters for creating a discount.
type CreateDiscountInput struct {
	Code            string
	Type            entity.DiscountType
	Value           decimal.Decimal
	MinPurchase     decimal.Decimal
	AppliesTo       entity.DiscountAppliesTo
	SellerID        *uuid.UUID
	ValidUntil      time.Time
	TotalUsageLimit int
}

// CreateDiscount creates a new discount.
func (s *DiscountService) CreateDiscount(ctx context.Context, tx db.Tx, input CreateDiscountInput) (*entity.Discount, error) {
	discount, err := entity.NewDiscount(
		input.Code,
		input.Type,
		input.Value,
		input.MinPurchase,
		input.AppliesTo,
		input.SellerID,
		input.ValidUntil,
		input.TotalUsageLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create discount: %w", err)
	}

	if err := s.repo.Create(ctx, tx, discount); err != nil {
		return nil, fmt.Errorf("failed to save discount: %w", err)
	}

	return discount, nil
}

// ============================================================================
// UPDATE
// ============================================================================

// UpdateDiscountInput contains parameters for updating a discount.
type UpdateDiscountInput struct {
	ID              uuid.UUID
	Code            string
	Type            entity.DiscountType
	Value           decimal.Decimal
	MinPurchase     decimal.Decimal
	AppliesTo       entity.DiscountAppliesTo
	SellerID        *uuid.UUID
	ValidUntil      time.Time
	TotalUsageLimit int
	IsActive        bool
}

// UpdateDiscount updates an existing discount.
func (s *DiscountService) UpdateDiscount(ctx context.Context, tx db.Tx, input UpdateDiscountInput) (*entity.Discount, error) {
	existing, err := s.repo.GetByID(ctx, tx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get discount: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("discount not found: id=%s", input.ID)
	}

	updated, err := entity.NewDiscount(
		input.Code,
		input.Type,
		input.Value,
		input.MinPurchase,
		input.AppliesTo,
		input.SellerID,
		input.ValidUntil,
		input.TotalUsageLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create updated discount: %w", err)
	}

	updated.ID = existing.ID
	updated.CurrentUsageCount = existing.CurrentUsageCount
	updated.CreatedAt = existing.CreatedAt
	updated.IsActive = input.IsActive

	if err := s.repo.Update(ctx, tx, updated); err != nil {
		return nil, fmt.Errorf("failed to update discount: %w", err)
	}

	return updated, nil
}

// ============================================================================
// DEACTIVATE
// ============================================================================

// DeactivateDiscount deactivates a discount by ID.
func (s *DiscountService) DeactivateDiscount(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	if err := s.repo.Deactivate(ctx, tx, id); err != nil {
		return fmt.Errorf("failed to deactivate discount: %w", err)
	}
	return nil
}

// ============================================================================
// GET
// ============================================================================

// GetDiscountByCode retrieves a discount by code.
func (s *DiscountService) GetDiscountByCode(ctx context.Context, tx db.Tx, code string) (*entity.Discount, error) {
	discount, err := s.repo.GetByCode(ctx, tx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to get discount by code: %w", err)
	}
	if discount == nil {
		return nil, fmt.Errorf("discount not found: code=%s", code)
	}
	return discount, nil
}

// GetDiscountByID retrieves a discount by ID.
func (s *DiscountService) GetDiscountByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Discount, error) {
	discount, err := s.repo.GetByID(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get discount: %w", err)
	}
	if discount == nil {
		return nil, fmt.Errorf("discount not found: id=%s", id)
	}
	return discount, nil
}

// GetDiscountsBySeller retrieves all discounts for a seller.
func (s *DiscountService) GetDiscountsBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*entity.Discount, error) {
	discounts, err := s.repo.GetBySeller(ctx, tx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller discounts: %w", err)
	}
	return discounts, nil
}

// ListActiveDiscounts retrieves all currently active discounts.
func (s *DiscountService) ListActiveDiscounts(ctx context.Context, tx db.Tx) ([]*entity.Discount, error) {
	discounts, err := s.repo.ListActive(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active discounts: %w", err)
	}
	return discounts, nil
}

// ============================================================================
// VALIDATION
// ============================================================================

// ValidateDiscountInput contains parameters for validating a discount.
type ValidateDiscountInput struct {
	UserID      uuid.UUID
	Code        string
	Subtotal    int64
	ContextType entity.DiscountContextType
	SellerID    *uuid.UUID
}

// ValidateDiscountResult contains the result of discount validation.
type ValidateDiscountResult struct {
	Valid           bool
	Discount        *entity.Discount
	ValidationError error
}

// ValidateDiscount validates if a discount can be used by a user.
//
// Validation order:
// 1. Code exists
// 2. Active + not expired
// 3. Economic safety (50% cap)
// 4. Seller ownership
// 5. Surface applicability (for_sale/auction/both)
// 6. Min purchase met (against P)
// 7. Usage limits
func (s *DiscountService) ValidateDiscount(ctx context.Context, tx db.Tx, input ValidateDiscountInput) (*ValidateDiscountResult, error) {
	if !input.ContextType.IsValid() {
		return nil, fmt.Errorf("invalid discount context type: %s", input.ContextType)
	}

	discount, err := s.repo.GetByCode(ctx, tx, input.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get discount: %w", err)
	}
	if discount == nil {
		return &ValidateDiscountResult{
			Valid:    false,
			Discount: nil,
			ValidationError: &entity.DiscountValidationError{
				Code:   input.Code,
				Reason: "discount not found",
			},
		}, nil
	}

	// Active + expiry check
	if !discount.IsActiveNow() {
		var validationError error
		if !discount.IsActive {
			validationError = &entity.DiscountNotActiveError{Code: discount.Code}
		} else {
			now := time.Now()
			if now.After(discount.ValidUntil) {
				validationError = &entity.DiscountExpiredError{Code: discount.Code, ValidUntil: discount.ValidUntil}
			}
		}
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: validationError,
		}, nil
	}

	// Economic safety
	if err := discount.ValidateEconomicSafety(); err != nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: err,
		}, nil
	}

	// Seller ownership
	if discount.SellerID == nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "discount is missing seller ownership"},
		}, nil
	}
	if input.SellerID == nil || *input.SellerID != *discount.SellerID {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "discount is only valid for the owning seller"},
		}, nil
	}

	// Surface applicability
	if !discount.AppliesTo.AllowsContext(input.ContextType) {
		return &ValidateDiscountResult{
			Valid:    false,
			Discount: discount,
			ValidationError: &entity.DiscountValidationError{
				Code:   discount.Code,
				Reason: fmt.Sprintf("discount is not applicable to %s checkout", input.ContextType),
			},
		}, nil
	}

	// Min purchase check against P (final transaction product price)
	subtotalDec := decimal.NewFromInt(input.Subtotal)
	if err := discount.MeetsMinPurchase(subtotalDec); err != nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: err,
		}, nil
	}

	// Usage limits
	if err := discount.CanBeUsedBy(); err != nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: err,
		}, nil
	}

	return &ValidateDiscountResult{
		Valid:    true,
		Discount: discount,
	}, nil
}

// ============================================================================
// CALCULATION
// ============================================================================

// CalculateDiscount calculates the discount amount for a given discount and subtotal (P).
func (s *DiscountService) CalculateDiscount(discount *entity.Discount, subtotal int64) (*entity.DiscountApplicationResult, error) {
	subtotalDec := decimal.NewFromInt(subtotal)
	discountAmount := discount.CalculateDiscountAmount(subtotalDec)

	return entity.NewDiscountApplicationResult(discount, discountAmount, subtotalDec), nil
}

// ============================================================================
// USAGE RECORDING
// ============================================================================

// RecordUsage records a discount usage after successful order creation.
func (s *DiscountService) RecordUsage(ctx context.Context, tx db.Tx, discountID, userID, orderID uuid.UUID) error {
	existing, err := s.repo.GetUsageByUserAndOrder(ctx, tx, userID, orderID)
	if err != nil {
		return fmt.Errorf("failed to check existing usage: %w", err)
	}
	if existing != nil {
		return nil
	}

	usage := entity.NewDiscountUsage(discountID, userID, orderID)
	if err := s.repo.RecordUsage(ctx, tx, usage); err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	if err := s.repo.IncrementUsageCount(ctx, tx, discountID); err != nil {
		return fmt.Errorf("failed to increment usage count: %w", err)
	}

	return nil
}

// ============================================================================
// CHECKOUT INTEGRATION
// ============================================================================

// ApplyDiscountAtCheckout applies a discount at checkout time.
func (s *DiscountService) ApplyDiscountAtCheckout(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	code string,
	subtotal int64,
	contextType entity.DiscountContextType,
	sellerID *uuid.UUID,
) (*entity.DiscountApplicationResult, error) {
	validation, err := s.ValidateDiscount(ctx, tx, ValidateDiscountInput{
		UserID:      userID,
		Code:        code,
		Subtotal:    subtotal,
		ContextType: contextType,
		SellerID:    sellerID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to validate discount: %w", err)
	}

	if !validation.Valid {
		return nil, validation.ValidationError
	}

	result, err := s.CalculateDiscount(validation.Discount, subtotal)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate discount: %w", err)
	}

	return result, nil
}
