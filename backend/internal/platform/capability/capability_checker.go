// Package capability provides the capability checker service for fine-grained authorization.
package capability

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/platform/capability/repository"
)

// Checker provides capability checking operations.
//
// DESIGN PRINCIPLES:
// - Minimal interface: only HasCapability, HasAnyCapability, ListActiveCapabilities
// - No role fallback logic (use RoleChecker for roles)
// - No caching (can be added at higher layer if needed)
// - All checks are explicit and auditable
type Checker struct {
	repo repository.CapabilityRepository
}

// NewChecker creates a new capability checker.
func NewChecker(repo repository.CapabilityRepository) *Checker {
	return &Checker{
		repo: repo,
	}
}

// HasCapability checks if a user has a specific active capability.
//
// Returns:
// - true: User has the capability (revoked_at IS NULL)
// - false: User does not have the capability or it was revoked
// - error: Database or system error
//
// Example:
//   checker.HasCapability(ctx, userID, capability.CapFinanceWithdrawRead)
func (c *Checker) HasCapability(ctx context.Context, userID uuid.UUID, cap Capability) (bool, error) {
	// Validate capability string
	if !IsValid(cap.String()) {
		return false, &entity.ErrInvalidCapability{Capability: cap.String()}
	}

	// Use direct pool connection (no transaction needed for read)
	// The repository will handle the tx parameter appropriately
	return c.repo.HasCapability(ctx, nil, userID, cap.String())
}

// HasAnyCapability checks if a user has any of the specified capabilities.
//
// Returns true if at least one capability is active.
// Returns false if none of the capabilities are active.
//
// Example:
//   checker.HasAnyCapability(ctx, userID, []Capability{
//       capability.CapFinanceWithdrawRead,
//       capability.CapFinanceWithdrawReview,
//   })
func (c *Checker) HasAnyCapability(ctx context.Context, userID uuid.UUID, caps []Capability) (bool, error) {
	if len(caps) == 0 {
		return false, nil
	}

	// Convert to string slice
	capStrs := make([]string, len(caps))
	for i, cap := range caps {
		if !IsValid(cap.String()) {
			return false, &entity.ErrInvalidCapability{Capability: cap.String()}
		}
		capStrs[i] = cap.String()
	}

	return c.repo.HasAnyCapability(ctx, nil, userID, capStrs)
}

// HasAllCapabilities checks if a user has all of the specified capabilities.
//
// Returns true only if ALL capabilities are active.
// Returns false if any capability is missing.
//
// This is a convenience method built on top of HasAnyCapability.
// It iterates through all capabilities and checks each one.
func (c *Checker) HasAllCapabilities(ctx context.Context, userID uuid.UUID, caps []Capability) (bool, error) {
	if len(caps) == 0 {
		return true, nil // Empty set is trivially satisfied
	}

	for _, cap := range caps {
		has, err := c.HasCapability(ctx, userID, cap)
		if err != nil {
			return false, fmt.Errorf("check capability %s: %w", cap, err)
		}
		if !has {
			return false, nil
		}
	}

	return true, nil
}

// ListActiveCapabilities returns all active capabilities for a user.
//
// Only returns capabilities where revoked_at IS NULL.
// The result is ordered by granted_at ASC.
func (c *Checker) ListActiveCapabilities(ctx context.Context, userID uuid.UUID) ([]Capability, error) {
	userCaps, err := c.repo.ListActiveCapabilities(ctx, nil, userID)
	if err != nil {
		return nil, fmt.Errorf("list active capabilities: %w", err)
	}

	result := make([]Capability, len(userCaps))
	for i, uc := range userCaps {
		result[i] = Capability(uc.Capability)
	}

	return result, nil
}

// CountActiveCapabilities returns the number of active capabilities for a user.
func (c *Checker) CountActiveCapabilities(ctx context.Context, userID uuid.UUID) (int, error) {
	return c.repo.CountActiveCapabilities(ctx, nil, userID)
}

// ============================================================
// INTERNAL METHODS (for grant/revoke, NOT exposed to handlers yet)
// ============================================================

// grantCapability grants a capability to a user.
// This is an internal method - NOT exposed via API in Slice 1.
//
// This will be exposed through a dedicated admin endpoint in a future slice.
func (c *Checker) grantCapability(ctx context.Context, tx interface{}, userID uuid.UUID, cap Capability, grantedBy *uuid.UUID) (*entity.UserCapability, error) {
	if !IsValid(cap.String()) {
		return nil, &entity.ErrInvalidCapability{Capability: cap.String()}
	}

	// Create new capability grant
	userCap := entity.NewCapabilityGrant(userID, cap.String(), grantedBy)

	// Persist
	err := c.repo.Create(ctx, tx, userCap)
	if err != nil {
		return nil, fmt.Errorf("grant capability: %w", err)
	}

	return userCap, nil
}

// revokeCapability revokes a capability from a user.
// This is an internal method - NOT exposed via API in Slice 1.
//
// This will be exposed through a dedicated admin endpoint in a future slice.
func (c *Checker) revokeCapability(ctx context.Context, tx interface{}, userID uuid.UUID, cap Capability) error {
	if !IsValid(cap.String()) {
		return &entity.ErrInvalidCapability{Capability: cap.String()}
	}

	// Find the active capability
	userCap, err := c.repo.GetActiveCapability(ctx, tx, userID, cap.String())
	if err != nil {
		return fmt.Errorf("get active capability: %w", err)
	}
	if userCap == nil {
		return &entity.ErrCapabilityNotFound{
			UserID:     userID,
			Capability: cap.String(),
		}
	}

	// Revoke it
	err = c.repo.Revoke(ctx, tx, userCap.ID, nil)
	if err != nil {
		return fmt.Errorf("revoke capability: %w", err)
	}

	return nil
}


