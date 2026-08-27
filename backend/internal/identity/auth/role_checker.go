package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
)

var (
	ErrUnauthorized            = errors.New("unauthorized: insufficient permissions")
	ErrAdminRequired           = errors.New("unauthorized: admin role required")
	ErrBuyerRequired           = errors.New("unauthorized: only buyer can perform this action")
	ErrSellerRequired          = errors.New("unauthorized: seller authority required to perform this action")
	ErrOwnerRequired           = errors.New("unauthorized: only account owner can perform this action")
	ErrInvalidCaller           = errors.New("unauthorized: invalid caller ID")
	ErrAccountSuspended        = errors.New("unauthorized: account is suspended")
	ErrAccountBanned           = errors.New("unauthorized: account is banned")
	ErrAccountInactive         = errors.New("unauthorized: account is not active")
	ErrAccountRemoved          = errors.New("unauthorized: account has been removed")
	ErrMarketAuthorityRequired = errors.New("unauthorized: active seller subscription required to perform market operations")
	ErrSellerNotReady          = errors.New("unauthorized: seller account not ready - active subscription required")
	ErrProfileNotComplete      = errors.New("unauthorized: profile not complete - required for checkout")
)

// SystemCallerID is the canonical special UUID used for system-initiated
// operations.
var SystemCallerID = audit.SystemCallerID

// IsSystemCaller returns true if the callerID is the system caller.
func IsSystemCaller(callerID uuid.UUID) bool {
	return audit.IsSystemCaller(callerID)
}

// RoleChecker defines the canonical authorization queries.
type RoleChecker interface {
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error)
	HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error)
}

// OwnershipValidator provides methods to validate resource ownership.
type OwnershipValidator struct{}

// NewOwnershipValidator creates a new ownership validator.
func NewOwnershipValidator() *OwnershipValidator {
	return &OwnershipValidator{}
}

// IsBuyer checks if the caller is the buyer of a trade.
func (v *OwnershipValidator) IsBuyer(callerID, buyerID uuid.UUID) bool {
	return callerID != uuid.Nil && callerID == buyerID
}

// IsSeller checks if the caller is the seller of a trade.
func (v *OwnershipValidator) IsSeller(callerID, sellerID uuid.UUID) bool {
	return callerID != uuid.Nil && callerID == sellerID
}

// IsParticipant checks if the caller is either the buyer or seller of a trade.
func (v *OwnershipValidator) IsParticipant(callerID, buyerID, sellerID uuid.UUID) bool {
	return v.IsBuyer(callerID, buyerID) || v.IsSeller(callerID, sellerID)
}

// IsOwner checks if the caller is the owner of a resource.
func (v *OwnershipValidator) IsOwner(callerID, ownerID uuid.UUID) bool {
	return callerID != uuid.Nil && callerID == ownerID
}

// ValidateCaller checks that callerID is not zero.
func ValidateCaller(callerID uuid.UUID) error {
	if callerID == uuid.Nil {
		return ErrInvalidCaller
	}
	return nil
}
