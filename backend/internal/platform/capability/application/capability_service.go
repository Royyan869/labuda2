// Package application provides the capability management service for admin operations.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/repository"
)

// CapabilityService handles capability management for admin operations.
//
// DESIGN PRINCIPLES:
// - EXPLICIT: Only valid capabilities can be assigned
// - SAFE: Cannot remove own critical capabilities
// - AUDITABLE: All operations are logged
// - PERFORMANT: No N+1 queries
type CapabilityService struct {
	capabilityRepo capabilityRepo.CapabilityRepository
	auditLogger    audit.AdminAuditLogger
}

// NewCapabilityService creates a new CapabilityService.
func NewCapabilityService(
	capabilityRepo capabilityRepo.CapabilityRepository,
	auditLogger audit.AdminAuditLogger,
) *CapabilityService {
	return &CapabilityService{
		capabilityRepo: capabilityRepo,
		auditLogger:    auditLogger,
	}
}

// ============================================================
// REQUEST/RESPONSE DTOs
// ============================================================

// CapabilityDefinition represents a capability definition for UI display.
type CapabilityDefinition struct {
	// Capability is the capability string (e.g., "finance.withdraw.read")
	Capability string `json:"capability"`

	// Category is the capability category for grouping (e.g., "Finance")
	Category string `json:"category"`

	// Description is a human-readable description
	Description string `json:"description"`

	// Critical indicates if this is a critical capability that shouldn't be removed without caution
	Critical bool `json:"critical"`
}

// UserCapabilityInfo represents a user's capability with metadata.
type UserCapabilityInfo struct {
	// Capability is the capability string
	Capability string `json:"capability"`

	// GrantedBy is the user who granted this capability (nullable)
	GrantedBy *uuid.UUID `json:"granted_by,omitempty"`

	// GrantedAt is when this capability was granted
	GrantedAt time.Time `json:"granted_at"`
}

// AssignCapabilityRequest represents a request to assign a capability.
type AssignCapabilityRequest struct {
	// Capability is the capability string to assign
	Capability string `json:"capability" binding:"required"`
}

// ErrCapabilityAuthorityRequired is returned when the actor lacks the
// capability-management authority needed to mutate capabilities.
type ErrCapabilityAuthorityRequired struct {
	ActorID    uuid.UUID
	Capability string
	Operation  string
}

func (e *ErrCapabilityAuthorityRequired) Error() string {
	return fmt.Sprintf("actor %s lacks capability %s for %s", e.ActorID, e.Capability, e.Operation)
}

// ============================================================
// SERVICE METHODS
// ============================================================

// ListAllCapabilities returns all valid capability definitions.
//
// This returns a HARDODED list of valid capabilities, NOT from database.
// This ensures only defined capabilities can be assigned.
func (s *CapabilityService) ListAllCapabilities(ctx context.Context) []CapabilityDefinition {
	return []CapabilityDefinition{
		// FINANCE CLUSTER
		{
			Capability:  capability.CapFinanceWithdrawRead.String(),
			Category:    "Finance",
			Description: "Can view withdrawal requests",
			Critical:    false,
		},
		{
			Capability:  capability.CapFinanceWithdrawReview.String(),
			Category:    "Finance",
			Description: "Can review, approve, or reject withdrawal requests",
			Critical:    true,
		},
		{
			Capability:  capability.CapFinanceDisputeResolve.String(),
			Category:    "Finance",
			Description: "Can resolve financial disputes",
			Critical:    true,
		},
		// GOVERNANCE CLUSTER
		{
			Capability:  capability.CapGovernanceDashboardView.String(),
			Category:    "Governance",
			Description: "Can view admin dashboard",
			Critical:    false,
		},
		{
			Capability:  capability.CapGovernanceUserRead.String(),
			Category:    "Governance",
			Description: "Can view user details and lists",
			Critical:    false,
		},
		{
			Capability:  capability.CapGovernanceUserSuspend.String(),
			Category:    "Governance",
			Description: "Can suspend user accounts",
			Critical:    true,
		},
		{
			Capability:  capability.CapGovernanceUserBan.String(),
			Category:    "Governance",
			Description: "Can ban user accounts",
			Critical:    true,
		},
		{
			Capability:  capability.CapGovernanceUserActivate.String(),
			Category:    "Governance",
			Description: "Can activate suspended or banned user accounts",
			Critical:    true,
		},
		{
			Capability:  capability.CapGovernanceRoleAssign.String(),
			Category:    "Governance",
			Description: "Can assign roles to users",
			Critical:    true,
		},
		{
			Capability:  capability.CapGovernanceCapabilityAssign.String(),
			Category:    "Governance",
			Description: "Can grant or revoke user capabilities",
			Critical:    true,
		},
		{
			Capability:  capability.CapGovernanceAuditRead.String(),
			Category:    "Governance",
			Description: "Can view audit logs",
			Critical:    false,
		},

		// MODERATION CLUSTER
		{
			Capability:  capability.CapModerationCaseRead.String(),
			Category:    "Moderation",
			Description: "Can view moderation cases and reports",
			Critical:    false,
		},
		{
			Capability:  capability.CapModerationContentView.String(),
			Category:    "Moderation",
			Description: "Can view reported content",
			Critical:    false,
		},
		{
			Capability:  capability.CapModerationContentRemove.String(),
			Category:    "Moderation",
			Description: "Can remove content",
			Critical:    true,
		},
		{
			Capability:  capability.CapModerationCaseResolve.String(),
			Category:    "Moderation",
			Description: "Can resolve moderation cases",
			Critical:    true,
		},
		{
			Capability:  capability.CapModerationEvidenceRead.String(),
			Category:    "Moderation",
			Description: "Can view original hidden moderation evidence",
			Critical:    true,
		},
		{
			Capability:  capability.CapModerationAppealReview.String(),
			Category:    "Moderation",
			Description: "Can review moderation appeals",
			Critical:    true,
		},

		// PROMOTION CLUSTER
		{
			Capability:  capability.CapPromotionExternalProductReview.String(),
			Category:    "Promotion",
			Description: "Can review external product promotions",
			Critical:    true,
		},
		{
			Capability:  capability.CapPromotionPackageManage.String(),
			Category:    "Promotion",
			Description: "Can create, update, enable, and disable promotion packages",
			Critical:    true,
		},
		{
			Capability:  capability.CapPromotionCampaignView.String(),
			Category:    "Promotion",
			Description: "Can view active and historical promotion campaigns",
			Critical:    false,
		},
		{
			Capability:  capability.CapPromotionCampaignStop.String(),
			Category:    "Promotion",
			Description: "Can force-stop a running promotion campaign",
			Critical:    true,
		},

		// SELLER CLUSTER
		{
			Capability:  capability.CapSellerVerificationReview.String(),
			Category:    "Seller",
			Description: "Can review seller verification requests",
			Critical:    false,
		},

		// ORDER CLUSTER
		{
			Capability:  capability.CapOrderRead.String(),
			Category:    "Order",
			Description: "Can view all orders (admin)",
			Critical:    false,
		},

		// CONFIG CLUSTER
		{
			Capability:  capability.CapConfigView.String(),
			Category:    "Config",
			Description: "Can view platform configuration",
			Critical:    false,
		},
		{
			Capability:  capability.CapConfigUpdateGeneral.String(),
			Category:    "Config",
			Description: "Can update platform configuration",
			Critical:    true,
		},

		// SUPPORT CLUSTER
		{
			Capability:  capability.CapSupportTicketRead.String(),
			Category:    "Support",
			Description: "Can view all support tickets",
			Critical:    false,
		},
		{
			Capability:  capability.CapSupportTicketRespond.String(),
			Category:    "Support",
			Description: "Can respond to support tickets",
			Critical:    false,
		},
		{
			Capability:  capability.CapSupportTicketClaim.String(),
			Category:    "Support",
			Description: "Can claim support tickets",
			Critical:    false,
		},
		{
			Capability:  capability.CapSupportTicketResolve.String(),
			Category:    "Support",
			Description: "Can resolve support tickets",
			Critical:    false,
		},
		{
			Capability:  capability.CapSupportAdminAssign.String(),
			Category:    "Support",
			Description: "Can reassign tickets to admins",
			Critical:    false,
		},
		{
			Capability:  capability.CapSupportAdminRead.String(),
			Category:    "Support",
			Description: "Can view support admin statistics and lists",
			Critical:    false,
		},
	}
}

// GetUserCapabilities retrieves all active capabilities for a user.
func (s *CapabilityService) GetUserCapabilities(ctx context.Context, userID uuid.UUID) ([]UserCapabilityInfo, error) {
	caps, err := s.capabilityRepo.ListActiveCapabilities(ctx, nil, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user capabilities: %w", err)
	}

	result := make([]UserCapabilityInfo, len(caps))
	for i, cap := range caps {
		result[i] = UserCapabilityInfo{
			Capability: cap.Capability,
			GrantedBy:  cap.GrantedBy,
			GrantedAt:  cap.GrantedAt,
		}
	}

	return result, nil
}

// HasCapability returns whether a user currently holds an active capability.
func (s *CapabilityService) HasCapability(ctx context.Context, userID uuid.UUID, capabilityStr string) (bool, error) {
	return s.capabilityRepo.HasCapability(ctx, nil, userID, capabilityStr)
}

// AssignCapability assigns a capability to a user.
//
// VALIDATION:
// - Capability must be valid (defined in capability package)
// - User must not already have this capability
// - Actor cannot grant a capability to themselves (self-escalation guard,
//   mirroring identity/auth.RoleCheckerDB.SetRole's self-escalation guard)
// - Audit log is written
//
// SELF-GRANT: Forbidden. An admin with governance.capability.assign can only
// grant capabilities to other users — granting to oneself must go through
// another authorized operator. This is a preventive guard, not a 4-eyes
// workflow: a second admin still only needs the same single capability to
// approve it.
func (s *CapabilityService) AssignCapability(
	ctx context.Context,
	userID uuid.UUID,
	capabilityStr string,
	grantedBy uuid.UUID,
) error {
	if grantedBy == uuid.Nil {
		return &ErrCapabilityAuthorityRequired{
			ActorID:    grantedBy,
			Capability: capability.CapGovernanceCapabilityAssign.String(),
			Operation:  "assign",
		}
	}
	hasAuthority, err := s.HasCapability(ctx, grantedBy, capability.CapGovernanceCapabilityAssign.String())
	if err != nil {
		return fmt.Errorf("failed to verify capability authority: %w", err)
	}
	if !hasAuthority {
		return &ErrCapabilityAuthorityRequired{
			ActorID:    grantedBy,
			Capability: capability.CapGovernanceCapabilityAssign.String(),
			Operation:  "assign",
		}
	}
	if grantedBy == userID {
		return &ErrSelfCapabilityGrantForbidden{
			ActorID:    grantedBy,
			Capability: capabilityStr,
		}
	}
	// Validate capability string
	if !capability.IsValid(capabilityStr) {
		return &ErrInvalidCapability{
			Capability: capabilityStr,
		}
	}

	// Check if user already has this capability
	existing, err := s.capabilityRepo.GetActiveCapability(ctx, nil, userID, capabilityStr)
	if err != nil {
		return fmt.Errorf("failed to check existing capability: %w", err)
	}

	if existing != nil {
		return &ErrDuplicateCapability{
			UserID:     userID,
			Capability: capabilityStr,
		}
	}

	// Create capability grant
	newCap := capabilityEntity.NewCapabilityGrant(userID, capabilityStr, &grantedBy)

	// Persist to database
	err = s.capabilityRepo.Create(ctx, nil, newCap)
	if err != nil {
		// Check for duplicate capability error
		if _, isDup := err.(*capabilityEntity.ErrDuplicateCapability); isDup {
			return &ErrDuplicateCapability{
				UserID:     userID,
				Capability: capabilityStr,
			}
		}
		return fmt.Errorf("failed to assign capability: %w", err)
	}

	// Write audit log
	s.auditLogger.LogSafe(ctx, grantedBy,
		"capability_assigned", "user_capability", userID,
		map[string]interface{}{
			"capability": capabilityStr,
			"action":     "assign",
		},
	)

	return nil
}

// RevokeCapability revokes a capability from a user.
//
// SAFETY RULES:
// - Cannot revoke own critical capability if it's the last one
// - Audit log is written
func (s *CapabilityService) RevokeCapability(
	ctx context.Context,
	targetUserID uuid.UUID,
	capabilityStr string,
	revokedBy uuid.UUID,
) error {
	if revokedBy == uuid.Nil {
		return &ErrCapabilityAuthorityRequired{
			ActorID:    revokedBy,
			Capability: capability.CapGovernanceCapabilityAssign.String(),
			Operation:  "revoke",
		}
	}
	hasAuthority, err := s.HasCapability(ctx, revokedBy, capability.CapGovernanceCapabilityAssign.String())
	if err != nil {
		return fmt.Errorf("failed to verify capability authority: %w", err)
	}
	if !hasAuthority {
		return &ErrCapabilityAuthorityRequired{
			ActorID:    revokedBy,
			Capability: capability.CapGovernanceCapabilityAssign.String(),
			Operation:  "revoke",
		}
	}
	// Validate capability string
	if !capability.IsValid(capabilityStr) {
		return &ErrInvalidCapability{
			Capability: capabilityStr,
		}
	}

	// Get the active capability
	activeCap, err := s.capabilityRepo.GetActiveCapability(ctx, nil, targetUserID, capabilityStr)
	if err != nil {
		return fmt.Errorf("failed to get active capability: %w", err)
	}

	if activeCap == nil {
		return &ErrCapabilityNotFound{
			UserID:     targetUserID,
			Capability: capabilityStr,
		}
	}

	// SAFETY RULE: Cannot revoke own critical capability
	if revokedBy == targetUserID {
		if isCriticalCapability(capabilityStr) {
			// Check if user has other critical capabilities
			userCaps, err := s.capabilityRepo.ListActiveCapabilities(ctx, nil, targetUserID)
			if err != nil {
				return fmt.Errorf("failed to check user capabilities: %w", err)
			}

			criticalCount := 0
			for _, cap := range userCaps {
				if isCriticalCapability(cap.Capability) {
					criticalCount++
				}
			}

			// If this is the last critical capability, prevent revocation
			if criticalCount <= 1 {
				return &ErrCannotRevokeOwnCriticalCapability{
					UserID:     targetUserID,
					Capability: capabilityStr,
				}
			}
		}
	}

	// Revoke the capability
	err = s.capabilityRepo.Revoke(ctx, nil, activeCap.ID, nil)
	if err != nil {
		return fmt.Errorf("failed to revoke capability: %w", err)
	}

	// Write audit log
	s.auditLogger.LogSafe(ctx, revokedBy,
		"capability_revoked", "user_capability", targetUserID,
		map[string]interface{}{
			"capability": capabilityStr,
			"action":     "revoke",
		},
	)

	return nil
}

// ListUsersByCapability returns user IDs holding an active capability.
// Delegates to repository which excludes revoked capabilities and deleted users.
// Banned/suspended users are intentionally included — the notification policy
// layer handles account status at delivery time (CommerceCritical bypasses).
func (s *CapabilityService) ListUsersByCapability(ctx context.Context, capabilityStr string) ([]uuid.UUID, error) {
	if !capability.IsValid(capabilityStr) {
		return nil, &ErrInvalidCapability{Capability: capabilityStr}
	}
	return s.capabilityRepo.ListUsersByCapability(ctx, nil, capabilityStr)
}

// ============================================================
// HELPER FUNCTIONS
// ============================================================

// isCriticalCapability returns true if the capability is marked as critical.
func isCriticalCapability(capStr string) bool {
	switch capability.Capability(capStr) {
	case capability.CapFinanceWithdrawReview,
		capability.CapFinanceDisputeResolve,
		capability.CapGovernanceUserSuspend,
		capability.CapGovernanceUserBan,
		capability.CapGovernanceUserActivate,
		capability.CapGovernanceRoleAssign,
		capability.CapGovernanceCapabilityAssign,
		capability.CapModerationContentRemove,
		capability.CapModerationCaseResolve,
		capability.CapModerationAppealReview,
		capability.CapConfigUpdateGeneral,
		capability.CapConfigUpdateFinancial:
		return true
	default:
		return false
	}
}

// ============================================================
// ERRORS
// ============================================================

// ErrInvalidCapability is returned when an invalid capability string is provided.
type ErrInvalidCapability struct {
	Capability string
}

func (e *ErrInvalidCapability) Error() string {
	return fmt.Sprintf("invalid capability: %s", e.Capability)
}

// ErrDuplicateCapability is returned when attempting to assign a capability that the user already has.
type ErrDuplicateCapability struct {
	UserID     uuid.UUID
	Capability string
}

func (e *ErrDuplicateCapability) Error() string {
	return fmt.Sprintf("user already has capability: %s", e.Capability)
}

// ErrCapabilityNotFound is returned when attempting to revoke a capability that the user doesn't have.
type ErrCapabilityNotFound struct {
	UserID     uuid.UUID
	Capability string
}

func (e *ErrCapabilityNotFound) Error() string {
	return fmt.Sprintf("user does not have capability: %s", e.Capability)
}

// ErrCannotRevokeOwnCriticalCapability is returned when an admin tries to revoke their own critical capability.
type ErrCannotRevokeOwnCriticalCapability struct {
	UserID     uuid.UUID
	Capability string
}

func (e *ErrCannotRevokeOwnCriticalCapability) Error() string {
	return "cannot revoke own critical capability"
}

// ErrSelfCapabilityGrantForbidden is returned when an actor attempts to
// grant a capability to themselves. Self-escalation must go through another
// authorized operator, mirroring the role self-escalation guard in
// identity/auth.RoleCheckerDB.SetRole.
type ErrSelfCapabilityGrantForbidden struct {
	ActorID    uuid.UUID
	Capability string
}

func (e *ErrSelfCapabilityGrantForbidden) Error() string {
	return fmt.Sprintf("self-escalation blocked: actor %s cannot grant capability %s to themselves", e.ActorID, e.Capability)
}
