// Package capability provides the bootstrap service for assigning initial capabilities.
package capability

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/repository"
)

// BootstrapResult summarizes the outcome of a bootstrap operation.
type BootstrapResult struct {
	// Created is the count of newly granted capabilities
	Created int

	// SkippedExisting is the count of capabilities that were already active
	SkippedExisting int

	// Invalid is the count of invalid capability strings
	Invalid int

	// Errors contains any errors that occurred during processing
	Errors []BootstrapError
}

// BootstrapError represents an error that occurred for a specific capability.
type BootstrapError struct {
	// Capability is the capability string that caused the error
	Capability string

	// Reason is the error reason
	Reason string
}

// BootstrapService handles initial capability assignment for users.
//
// DESIGN PRINCIPLES:
// - EXPLICIT: Capabilities are granted explicitly, no implicit role mapping
// - IDEMPOTENT: Safe to run multiple times, skips existing active capabilities
// - SAFE: Never revokes existing capabilities, never has hidden side effects
// - AUDITABLE: All grants track who granted the capability
//
// This is a BOOTSTRAP service, NOT a general-purpose capability management system.
// Use this for initial setup only, not for ongoing capability administration.
type BootstrapService struct {
	repo capabilityRepo.CapabilityRepository
}

// NewBootstrapService creates a new BootstrapService.
func NewBootstrapService(repo capabilityRepo.CapabilityRepository) *BootstrapService {
	return &BootstrapService{
		repo: repo,
	}
}

// AssignInitialCapabilities grants capabilities to a target user.
//
// This method is IDEMPOTENT:
// - Valid capabilities that are already active are skipped (not errors)
// - Invalid capability strings are reported but don't abort the entire operation
//
// Parameters:
// - ctx: Context for the operation
// - tx: Transaction to use (must be non-nil)
// - targetUserID: The user receiving capabilities
// - capabilities: List of capability strings to grant
// - grantedBy: The user granting capabilities (nil for system grants)
//
// Returns:
// - BootstrapResult with counts and any errors
//
// Example:
//
//	result, err := service.AssignInitialCapabilities(ctx, tx, userID, []string{
//	    "finance.withdraw.review",
//	    "finance.withdraw.read",
//	}, nil)
func (s *BootstrapService) AssignInitialCapabilities(
	ctx context.Context,
	tx interface{},
	targetUserID uuid.UUID,
	capabilities []string,
	grantedBy *uuid.UUID,
) (*BootstrapResult, error) {
	result := &BootstrapResult{
		Errors: make([]BootstrapError, 0),
	}

	for _, capStr := range capabilities {
		// Validate capability string
		if !IsValid(capStr) {
			result.Invalid++
			result.Errors = append(result.Errors, BootstrapError{
				Capability: capStr,
				Reason:     "invalid capability string",
			})
			continue
		}

		// Check if user already has this capability active
		hasExisting, err := s.repo.HasCapability(ctx, tx, targetUserID, capStr)
		if err != nil {
			result.Errors = append(result.Errors, BootstrapError{
				Capability: capStr,
				Reason:     fmt.Sprintf("check existing failed: %v", err),
			})
			continue
		}

		if hasExisting {
			// Skip - capability already active
			result.SkippedExisting++
			continue
		}

		// Grant the capability
		userCap := entity.NewCapabilityGrant(targetUserID, capStr, grantedBy)
		err = s.repo.Create(ctx, tx, userCap)
		if err != nil {
			// Check for duplicate error (race condition)
			if _, isDup := err.(*entity.ErrDuplicateCapability); isDup {
				result.SkippedExisting++
				continue
			}
			result.Errors = append(result.Errors, BootstrapError{
				Capability: capStr,
				Reason:     fmt.Sprintf("grant failed: %v", err),
			})
			continue
		}

		result.Created++
	}

	return result, nil
}

// PURGED: PresetBootstrapAdmin.
//
// There is no "minimum bootstrap" capability model. Bootstrap-admin (initial
// system setup and disaster recovery) grants the entire canonical universe via
// capability.AllCapabilityStrings() and therefore yields derived full access.
// Keeping a second, smaller bootstrap authority would be a hidden alternate
// model for how an admin gets power — exactly what the canonical authority
// graph forbids. Do not reintroduce it.

// ValidateCapabilities checks if all capability strings in a list are valid.
//
// Returns:
// - valid: list of valid capability strings
// - invalid: list of invalid capability strings
func ValidateCapabilities(capabilities []string) (valid []string, invalid []string) {
	valid = make([]string, 0, len(capabilities))
	invalid = make([]string, 0)

	for _, capStr := range capabilities {
		if IsValid(capStr) {
			valid = append(valid, capStr)
		} else {
			invalid = append(invalid, capStr)
		}
	}

	return valid, invalid
}


