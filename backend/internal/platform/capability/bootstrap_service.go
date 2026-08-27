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

// Preset capability sets for common operational roles.
//
// PRESETS ARE CONVENIENCE ONLY, NOT ROLE MAPPINGS.
// They simply group related capabilities for assignment convenience.
const (
	// PresetFinanceReviewer grants capabilities for financial operations review
	PresetFinanceReviewer = "finance_reviewer"

	// PresetGovernanceBasic grants basic governance capabilities
	PresetGovernanceBasic = "governance_basic"

	// PresetModerationBasic grants basic moderation capabilities
	PresetModerationBasic = "moderation_basic"

	// PresetSellerVerification grants seller verification review capability
	PresetSellerVerification = "seller_verification"

	// PresetConfigManager grants configuration management capabilities
	PresetConfigManager = "config_manager"

	// PresetSupportAdmin grants support ticket management capabilities
	PresetSupportAdmin = "support_admin"
)

// GetPresetCapabilities returns the capability list for a given preset.
//
// Returns an error if the preset name is unknown.
// This ensures explicitness - unknown presets are rejected rather than silently ignored.
func GetPresetCapabilities(preset string) ([]string, error) {
	switch preset {
	case PresetFinanceReviewer:
		return []string{
			CapFinanceWithdrawRead.String(),
			CapFinanceWithdrawReview.String(),
			CapFinanceDisputeResolve.String(),
		}, nil

	case PresetGovernanceBasic:
		return []string{
			CapGovernanceAuditRead.String(),
			CapGovernanceUserSuspend.String(),
			CapGovernanceRoleAssign.String(),
		}, nil

	case PresetModerationBasic:
		return []string{
			CapModerationContentView.String(),
			CapModerationContentRemove.String(),
			CapModerationCaseResolve.String(),
		}, nil

	case PresetSellerVerification:
		return []string{
			CapSellerVerificationReview.String(),
		}, nil

	case PresetConfigManager:
		return []string{
			CapConfigView.String(),
			CapConfigUpdateGeneral.String(),
			CapConfigUpdateFinancial.String(),
		}, nil

	case PresetSupportAdmin:
		return []string{
			CapSupportTicketRespond.String(),
			CapSupportTicketClaim.String(),
			CapSupportTicketResolve.String(),
			CapSupportAdminAssign.String(),
		}, nil

	default:
		return nil, fmt.Errorf("unknown preset: %s", preset)
	}
}

// AssignInitialCapabilitiesFromPreset grants all capabilities from a preset to a user.
//
// This is a convenience method that combines GetPresetCapabilities and AssignInitialCapabilities.
//
// Returns an error if the preset name is unknown.
// The bootstrap result will contain details of the operation.
func (s *BootstrapService) AssignInitialCapabilitiesFromPreset(
	ctx context.Context,
	tx interface{},
	targetUserID uuid.UUID,
	preset string,
	grantedBy *uuid.UUID,
) (*BootstrapResult, error) {
	capabilities, err := GetPresetCapabilities(preset)
	if err != nil {
		return nil, err
	}

	return s.AssignInitialCapabilities(ctx, tx, targetUserID, capabilities, grantedBy)
}

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


