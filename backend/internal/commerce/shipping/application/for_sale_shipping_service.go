package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/repository"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ProductShippingService handles product-shipping option management.
// This is a write-only service for setting shipping options on products.
type ProductShippingService struct {
	productRepo         ForSaleRepository
	shippingOptionRepo  shippingRepo.ShippingOptionRepository
	productShippingRepo shippingRepo.ProductShippingOptionRepository
	orderRepo           orderRepo.OrderRepository
}

// ForSaleRepository defines the interface for product persistence (validation only).
type ForSaleRepository interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error)
}

// NewProductShippingService creates a new ProductShippingService.
func NewProductShippingService(
	productRepo ForSaleRepository,
	shippingOptionRepo shippingRepo.ShippingOptionRepository,
	productShippingRepo shippingRepo.ProductShippingOptionRepository,
	orderRepo orderRepo.OrderRepository,
) *ProductShippingService {
	return &ProductShippingService{
		productRepo:         productRepo,
		shippingOptionRepo:  shippingOptionRepo,
		productShippingRepo: productShippingRepo,
		orderRepo:           orderRepo,
	}
}

// SetProductShippingOptionsInput contains the parameters for setting shipping options.
type SetProductShippingOptionsInput struct {
	// ProductID is the product to set shipping options for
	ProductID uuid.UUID

	// ShippingOptionIDs are the shipping option IDs to link (empty = clear all)
	ShippingOptionIDs []uuid.UUID

	// SellerID is the authenticated seller ID (for ownership check)
	SellerID uuid.UUID
}

// SetProductShippingOptions sets shipping options for a product.
//
// Behavior:
// 1. Validate product exists and belongs to seller
// 2. Validate all shippingOptionIDs exist
// 3. Delete existing rows from product_shipping_options where product_id = productID
// 4. Insert new rows (product_id, shipping_option_id) with sort_order
//
// This is an overwrite model - all existing options are replaced.
func (s *ProductShippingService) SetProductShippingOptions(
	ctx context.Context,
	tx db.Tx,
	input SetProductShippingOptionsInput,
) error {
	// Step 1: Validate product exists
	product, err := s.productRepo.GetByID(ctx, tx, input.ProductID)
	if err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	// Step 2: Verify ownership
	if product.SellerID != input.SellerID {
		return fmt.Errorf("forbidden: product does not belong to seller")
	}

	// Step 3: Check for active orders before allowing shipping option changes
	// Active orders: pending, paid, shipped, delivered
	activeOrderCount, err := s.orderRepo.CountActiveOrdersByProduct(ctx, tx, input.ProductID)
	if err != nil {
		return fmt.Errorf("failed to check for active orders: %w", err)
	}
	if activeOrderCount > 0 {
		return fmt.Errorf("cannot update shipping: active orders exist")
	}

	// Step 4: Validate all shipping option IDs exist (if provided)
	if len(input.ShippingOptionIDs) > 0 {
		for _, optionID := range input.ShippingOptionIDs {
			option, err := s.shippingOptionRepo.GetByID(ctx, tx, optionID)
			if err != nil {
				return fmt.Errorf("shipping option %s not found: %w", optionID, err)
			}
			// Verify option belongs to seller
			if option.SellerID != input.SellerID {
				return fmt.Errorf("shipping option %s does not belong to seller", optionID)
			}
		}
	}

	// Step 5: Delete existing options (simple overwrite model)
	if err := s.productShippingRepo.DeleteByProduct(ctx, tx, input.ProductID); err != nil {
		return fmt.Errorf("failed to delete existing shipping options: %w", err)
	}

	// Step 6: Insert new options (if any)
	if len(input.ShippingOptionIDs) > 0 {
		if err := s.productShippingRepo.CreateBulk(ctx, tx, input.ProductID, input.ShippingOptionIDs); err != nil {
			return fmt.Errorf("failed to create shipping options: %w", err)
		}
	}

	return nil
}

// GetProductShippingOptions retrieves all shipping options for a product.
func (s *ProductShippingService) GetProductShippingOptions(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) ([]*shippingEntity.ShippingOption, error) {
	return s.productShippingRepo.GetByProduct(ctx, tx, productID)
}

// ValidateSellableCreateShippingSelection validates that all provided shipping
// option IDs exist, belong to the seller, and each has at least one active
// (is_available=true) coverage row. Used at sellable (auction/for_sale) creation
// time, before the product row exists. Returns the validated option IDs.
//
// An option with zero active coverages cannot serve any buyer address, so
// linking it to a sellable surface would produce a marketplace item that no
// buyer can ever purchase — blocked only at checkout, with no early feedback.
// Failing here gives the seller actionable signal at the earliest opportunity.
func ValidateSellableCreateShippingSelection(
	ctx context.Context,
	tx db.Tx,
	optionRepo shippingRepo.ShippingOptionRepository,
	coverageRepo shippingRepo.ShippingCoverageRepository,
	sellerID uuid.UUID,
	shippingOptionIDs []uuid.UUID,
) ([]uuid.UUID, error) {
	for _, optionID := range shippingOptionIDs {
		option, err := optionRepo.GetByID(ctx, tx, optionID)
		if err != nil {
			return nil, fmt.Errorf("%w: option %s lookup failed: %v", ErrInvalidSellableCreateShippingSelection, optionID, err)
		}
		if option.SellerID != sellerID {
			return nil, fmt.Errorf("%w: option %s does not belong to seller", ErrInvalidSellableCreateShippingSelection, optionID)
		}
		// Require at least one active coverage so the option can serve buyers.
		coverages, err := coverageRepo.GetByShippingOption(ctx, tx, optionID)
		if err != nil {
			return nil, fmt.Errorf("%w: option %s coverage lookup failed: %v", ErrInvalidSellableCreateShippingSelection, optionID, err)
		}
		hasActive := false
		for _, c := range coverages {
			if c.IsAvailable {
				hasActive = true
				break
			}
		}
		if !hasActive {
			return nil, fmt.Errorf("%w: option %s has no active coverage configured", ErrInvalidSellableCreateShippingSelection, optionID)
		}
	}
	return shippingOptionIDs, nil
}

// LinkSellableCreateShippingSelection persists product-shipping-option links
// after the product row has been created. Pass the validated IDs from
// ValidateSellableCreateShippingSelection. No-ops when the list is empty.
func LinkSellableCreateShippingSelection(
	ctx context.Context,
	tx db.Tx,
	productShippingRepo shippingRepo.ProductShippingOptionRepository,
	productID uuid.UUID,
	shippingOptionIDs []uuid.UUID,
) error {
	if len(shippingOptionIDs) == 0 {
		return nil
	}
	return productShippingRepo.CreateBulk(ctx, tx, productID, shippingOptionIDs)
}
