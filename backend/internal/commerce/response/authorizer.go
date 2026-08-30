// Package response provides the canonical Commerce Response resource
// validation boundary. It validates that referenced commerce resources
// exist and are in a valid state for display/reference in Create Content,
// Comment, and Chat contexts.
//
// This package does NOT enforce:
//   - Ownership (caller does not need to own the resource)
//   - Seller capability (caller does not need seller subscription)
//   - Commerce lifecycle mutations
//   - Feed / Social Repost / Internal Share
//   - Profile
//   - Chat room membership
//   - Comment permission
//   - Content creation permission
//   - HTTP / JSON / UI
//
// Ownership and lifecycle authority remains exclusively in the commerce
// domain (For Sale / Auction CRUD services).
package response

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/pkg/db"
)

// Sentinel errors for Commerce Response reference validation.
var (
	// ErrResourceNotFound is returned when the referenced resource does not exist.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrResourceNotDisplayable is returned when the resource exists but its
	// state does not permit display/reference in Commerce Response contexts
	// (e.g. draft, sold, ended, withdrawn).
	ErrResourceNotDisplayable = errors.New("resource not valid for Commerce Response")
)

// ResourceType identifies a commerce resource eligible for Commerce Response
// reference/display.
type ResourceType string

const (
	// ResourceTypeForSale identifies a fixed-price sale resource.
	ResourceTypeForSale ResourceType = "for_sale"

	// ResourceTypeAuction identifies an auction resource.
	ResourceTypeAuction ResourceType = "auction"
)

// Narrow domain interfaces for the commerce resources the validator reads.

// ForSaleGetter loads a ForSale aggregate by ID.
type ForSaleGetter interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*forSaleEntity.ForSale, error)
}

// AuctionGetter loads an Auction aggregate by ID.
type AuctionGetter interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error)
}

// Validator is the canonical Commerce Response resource reference validator.
// It validates that:
//
//	resource exists
//	AND resource state is valid for display/reference
//
// It does NOT enforce ownership or seller capability. Those authorities
// remain exclusively in the commerce domain (For Sale / Auction CRUD).
type Validator interface {
	// ValidateReference checks that the specified commerce resource exists
	// and is in a state that permits display/reference. Returns nil on
	// success, or a typed sentinel error on failure.
	ValidateReference(ctx context.Context, tx db.Tx, resourceType ResourceType, resourceID uuid.UUID) error
}

// commerceResourceValidator implements Validator using canonical domain services.
type commerceResourceValidator struct {
	fpsGetter     ForSaleGetter
	auctionGetter AuctionGetter
}

// NewValidator creates the canonical Commerce Response resource reference validator.
func NewValidator(
	fpsGetter ForSaleGetter,
	auctionGetter AuctionGetter,
) Validator {
	return &commerceResourceValidator{
		fpsGetter:     fpsGetter,
		auctionGetter: auctionGetter,
	}
}

// ValidateReference dispatches to the type-specific validation chain.
func (v *commerceResourceValidator) ValidateReference(
	ctx context.Context, tx db.Tx,
	resourceType ResourceType, resourceID uuid.UUID,
) error {
	switch resourceType {
	case ResourceTypeForSale:
		return v.validateForSale(ctx, tx, resourceID)
	case ResourceTypeAuction:
		return v.validateAuction(ctx, tx, resourceID)
	default:
		return fmt.Errorf("unsupported commerce resource type for Commerce Response: %s", resourceType)
	}
}

// validateForSale checks:
//   - resource exists
//   - status == active (IsRepostable for ForSale returns true only for active)
func (v *commerceResourceValidator) validateForSale(
	ctx context.Context, tx db.Tx, fpsID uuid.UUID,
) error {
	fps, err := v.fpsGetter.GetByID(ctx, tx, fpsID)
	if err != nil {
		return ErrResourceNotFound
	}

	// ForSale: only active is valid for display/reference.
	// IsRepostable returns true only for active status.
	if !fps.Status.IsRepostable() {
		return ErrResourceNotDisplayable
	}

	return nil
}

// validateAuction checks:
//   - resource exists
//   - status in {scheduled, active} (IsRepostable for Auction)
func (v *commerceResourceValidator) validateAuction(
	ctx context.Context, tx db.Tx, auctionID uuid.UUID,
) error {
	auc, err := v.auctionGetter.GetByID(ctx, tx, auctionID)
	if err != nil {
		return ErrResourceNotFound
	}

	// Auction: scheduled or active is valid for display/reference.
	// IsRepostable returns true for scheduled and active.
	if !auc.Status.IsRepostable() {
		return ErrResourceNotDisplayable
	}

	return nil
}
