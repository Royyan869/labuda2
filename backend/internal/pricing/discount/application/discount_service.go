// ============================================================================
// DISCOUNT SERVICE - DISCOUNT VALIDATION SOURCE OF TRUTH
// ============================================================================
//
// SOURCE OF TRUTH: All discount validation must go through this service.
//
// DOMAIN RULE: This service is the SINGLE SOURCE OF TRUTH for discount operations.
// - Discount does NOT modify forSale.price
// - Discount is ONLY calculated at checkout time
// - Discount does NOT touch ledger directly
// - OrderService remains the only creator of orders
//
// BUSINESS RULES:
// - ECONOMIC SAFETY: Discount capped at 50% (MaxDiscountPercentage)
// - COMMISSION SAFETY: final_order_value >= commission_amount
// - DUPLICATE PREVENTION: One discount per order (database constraint)
// - USAGE LIMITS: Per-user and total usage limits enforced
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

// NewDiscountService creates a new DiscountService.
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
	MaxDiscount     *decimal.Decimal
	AppliesTo       entity.DiscountAppliesTo
	TargetMode      entity.DiscountTargetMode
	SellerID        *uuid.UUID
	ForSaleIDs      []uuid.UUID
	AuctionIDs      []uuid.UUID
	ValidFrom       time.Time
	ValidUntil      time.Time
	MaxUsagePerUser int
	TotalUsageLimit int
}

// CreateDiscount creates a new discount.
func (s *DiscountService) CreateDiscount(ctx context.Context, tx db.Tx, input CreateDiscountInput) (*entity.Discount, error) {
	discount, err := entity.NewDiscount(
		input.Code,
		input.Type,
		input.Value,
		input.MinPurchase,
		input.MaxDiscount,
		input.AppliesTo,
		input.TargetMode,
		input.SellerID,
		input.ForSaleIDs,
		input.AuctionIDs,
		input.ValidFrom,
		input.ValidUntil,
		input.MaxUsagePerUser,
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
	MaxDiscount     *decimal.Decimal
	AppliesTo       entity.DiscountAppliesTo
	TargetMode      entity.DiscountTargetMode
	SellerID        *uuid.UUID
	ForSaleIDs      []uuid.UUID
	AuctionIDs      []uuid.UUID
	ValidFrom       time.Time
	ValidUntil      time.Time
	MaxUsagePerUser int
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
		input.MaxDiscount,
		input.AppliesTo,
		input.TargetMode,
		input.SellerID,
		input.ForSaleIDs,
		input.AuctionIDs,
		input.ValidFrom,
		input.ValidUntil,
		input.MaxUsagePerUser,
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
	ForSaleID   *uuid.UUID
	AuctionID   *uuid.UUID
}

// ValidateDiscountResult contains the result of discount validation.
type ValidateDiscountResult struct {
	Valid           bool
	Discount        *entity.Discount
	UserUsageCount  int
	ValidationError error
}

// ValidateDiscount validates if a discount can be used by a user.
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

	if !discount.IsActiveNow() {
		var validationError error
		if !discount.IsActive {
			validationError = &entity.DiscountNotActiveError{Code: discount.Code}
		} else {
			now := time.Now()
			if now.Before(discount.ValidFrom) {
				validationError = &entity.DiscountValidationError{Code: discount.Code, Reason: "discount not yet valid"}
			} else if now.After(discount.ValidUntil) {
				validationError = &entity.DiscountExpiredError{Code: discount.Code, ValidUntil: discount.ValidUntil}
			}
		}
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: validationError,
		}, nil
	}

	if err := discount.ValidateEconomicSafety(); err != nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			ValidationError: err,
		}, nil
	}

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

	switch input.ContextType {
	case entity.DiscountContextForSale:
		if input.ForSaleID == nil {
			return &ValidateDiscountResult{
				Valid:           false,
				Discount:        discount,
				ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "forSale checkout requires forSale id"},
			}, nil
		}
		if discount.TargetMode == entity.DiscountTargetModeSelectedItems {
			if len(discount.ForSaleIDs) == 0 {
				return &ValidateDiscountResult{
					Valid:           false,
					Discount:        discount,
					ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "forSale discount has no forSale targets configured"},
				}, nil
			}
			if !uuidSliceContains(discount.ForSaleIDs, *input.ForSaleID) {
				return &ValidateDiscountResult{
					Valid:           false,
					Discount:        discount,
					ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "discount is not applicable to this forSale"},
				}, nil
			}
		}
	case entity.DiscountContextAuction:
		if input.AuctionID == nil {
			return &ValidateDiscountResult{
				Valid:           false,
				Discount:        discount,
				ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "auction checkout requires auction id"},
			}, nil
		}
		if discount.TargetMode == entity.DiscountTargetModeSelectedItems {
			if len(discount.AuctionIDs) == 0 {
				return &ValidateDiscountResult{
					Valid:           false,
					Discount:        discount,
					ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "auction discount has no auction targets configured"},
				}, nil
			}
			if !uuidSliceContains(discount.AuctionIDs, *input.AuctionID) {
				return &ValidateDiscountResult{
					Valid:           false,
					Discount:        discount,
					ValidationError: &entity.DiscountValidationError{Code: discount.Code, Reason: "discount is not applicable to this auction"},
				}, nil
			}
		}
	}

	userUsageCount, err := s.repo.CountUsageByUser(ctx, tx, discount.ID, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user usage count: %w", err)
	}

	if err := discount.CanBeUsedBy(userUsageCount); err != nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			UserUsageCount:  userUsageCount,
			ValidationError: err,
		}, nil
	}

	subtotalDec := decimal.NewFromInt(input.Subtotal)
	if err := discount.MeetsMinPurchase(subtotalDec); err != nil {
		return &ValidateDiscountResult{
			Valid:           false,
			Discount:        discount,
			UserUsageCount:  userUsageCount,
			ValidationError: err,
		}, nil
	}

	return &ValidateDiscountResult{
		Valid:          true,
		Discount:       discount,
		UserUsageCount: userUsageCount,
	}, nil
}

// ============================================================================
// CALCULATION
// ============================================================================

// CalculateDiscount calculates the discount amount for a given discount and subtotal.
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

// ApplicableDiscount represents a discount that can be applied with its validation result.
type ApplicableDiscount struct {
	Discount         *entity.Discount
	ValidationResult *ValidateDiscountResult
	DiscountAmount   decimal.Decimal
}

// FindBestApplicableDiscount finds the best applicable discount for an order.
func (s *DiscountService) FindBestApplicableDiscount(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	subtotal int64,
	contextType entity.DiscountContextType,
	sellerID *uuid.UUID,
	forSaleID *uuid.UUID,
	auctionID *uuid.UUID,
) (*ApplicableDiscount, error) {
	allDiscounts, err := s.repo.ListActive(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active discounts: %w", err)
	}

	subtotalDec := decimal.NewFromInt(subtotal)
	var applicable []*ApplicableDiscount

	for _, discount := range allDiscounts {
		validation, err := s.ValidateDiscount(ctx, tx, ValidateDiscountInput{
			UserID:      userID,
			Code:        discount.Code,
			Subtotal:    subtotal,
			ContextType: contextType,
			SellerID:    sellerID,
			ForSaleID:   forSaleID,
			AuctionID:   auctionID,
		})
		if err != nil || !validation.Valid {
			continue
		}

		discountAmount := validation.Discount.CalculateDiscountAmount(subtotalDec)
		applicable = append(applicable, &ApplicableDiscount{
			Discount:         validation.Discount,
			ValidationResult: validation,
			DiscountAmount:   discountAmount,
		})
	}

	if len(applicable) == 0 {
		return nil, nil
	}

	best := applicable[0]
	for i := 1; i < len(applicable); i++ {
		if applicable[i].Discount.IsBetterThan(best.Discount, subtotalDec, contextType) {
			best = applicable[i]
		}
	}

	return best, nil
}

// ApplyDiscountAtCheckout applies a discount at checkout time.
func (s *DiscountService) ApplyDiscountAtCheckout(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	code string,
	subtotal int64,
	contextType entity.DiscountContextType,
	sellerID *uuid.UUID,
	forSaleID *uuid.UUID,
	auctionID *uuid.UUID,
) (*entity.DiscountApplicationResult, error) {
	validation, err := s.ValidateDiscount(ctx, tx, ValidateDiscountInput{
		UserID:      userID,
		Code:        code,
		Subtotal:    subtotal,
		ContextType: contextType,
		SellerID:    sellerID,
		ForSaleID:   forSaleID,
		AuctionID:   auctionID,
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

func uuidSliceContains(values []uuid.UUID, target uuid.UUID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}


