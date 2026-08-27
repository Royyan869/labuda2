// Package application provides the admin application service.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/admin/repository"
	"github.com/labuda/backend/internal/platform/capability"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// AdminService handles admin operations.
//
// Responsibilities:
// - User management (list, view, suspend, activate, ban)
// - Dashboard metrics
// - Audit log queries
// - System alerts
type AdminService struct {
	db               Transactor
	repo             repository.AdminRepository
	adminAuditLogger audit.AdminAuditLogger
	outboxRepo       *outboxRepo.OutboxRepository
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewAdminService creates a new AdminService.
func NewAdminService(
	db Transactor,
	repo repository.AdminRepository,
	adminAuditLogger audit.AdminAuditLogger,
	outboxRepo *outboxRepo.OutboxRepository,
) *AdminService {
	return &AdminService{
		db:               db,
		repo:             repo,
		adminAuditLogger: adminAuditLogger,
		outboxRepo:       outboxRepo,
	}
}

// ============================================================================
// USER MANAGEMENT
// ============================================================================

// UserListResult represents the result of listing users.
type UserListResult struct {
	Users    []repository.UserSummary
	Total    int
	Page     int
	PageSize int
}

// ListUsers returns a paginated list of users with optional filtering.
func (s *AdminService) ListUsers(
	ctx context.Context,
	filters repository.UserListFilters,
) (*UserListResult, error) {
	var users []repository.UserSummary
	var total int

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		users, err = s.repo.ListUsers(ctx, tx, filters)
		if err != nil {
			return err
		}

		total, err = s.repo.CountUsers(ctx, tx, filters)
		return err
	})

	if err != nil {
		return nil, err
	}

	return &UserListResult{
		Users:    users,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	}, nil
}

// GetUserDetails returns detailed information about a specific user.
func (s *AdminService) GetUserDetails(ctx context.Context, userID uuid.UUID) (*repository.UserDetails, error) {
	var userDetails *repository.UserDetails

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		userDetails, err = s.repo.GetUserDetails(ctx, tx, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return userDetails, nil
}

// SuspendUserRequest represents the request for suspending a user.
type SuspendUserRequest struct {
	Reason       string
	DurationDays *int
}

// SuspendUser suspends a user account for a specified duration or indefinitely.
func (s *AdminService) SuspendUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetUserID uuid.UUID,
	req SuspendUserRequest,
) error {
	// 🔥 P0 SECURITY: Capability check for user suspension
	if !capability.HasCapability(ctx, capability.CapGovernanceUserSuspend.String()) {
		return fmt.Errorf("forbidden: missing capability %s", capability.CapGovernanceUserSuspend.String())
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Update user status to suspended
		if err := s.repo.UpdateUserStatus(ctx, tx, targetUserID, "suspended"); err != nil {
			return err
		}

		// Build metadata for audit log
		metadata := map[string]interface{}{
			"reason":          req.Reason,
			"capability_used": capability.CapGovernanceUserSuspend.String(),
		}
		if req.DurationDays != nil {
			metadata["duration_days"] = *req.DurationDays
		}

		// Log admin action (ATOMIC - within transaction)
		if err := s.adminAuditLogger.LogTx(ctx, tx, actorID,
			"user_suspended",
			"user",
			targetUserID,
			metadata,
		); err != nil {
			return err
		}

		return nil
	})
}

// ActivateUser activates a suspended user account.
//
// BAN PERMANENCE: Banned accounts CANNOT be activated through this path.
// Use UnbanUser with governance.user.unban capability instead.
func (s *AdminService) ActivateUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetUserID uuid.UUID,
) error {
	// 🔥 P0 SECURITY: Capability check for user activation
	if !capability.HasCapability(ctx, capability.CapGovernanceUserActivate.String()) {
		return fmt.Errorf("forbidden: missing capability %s", capability.CapGovernanceUserActivate.String())
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Get current status for audit log
		currentStatus, err := s.repo.GetUserStatus(ctx, tx, targetUserID)
		if err != nil {
			return err
		}

		// BAN PERMANENCE GUARD: Banned users require explicit unban path.
		if currentStatus == "banned" {
			return &ErrCannotActivateBannedUser{}
		}

		// Update user status to active
		if err := s.repo.UpdateUserStatus(ctx, tx, targetUserID, "active"); err != nil {
			return err
		}

		// Log admin action (ATOMIC - within transaction)
		if err := s.adminAuditLogger.LogTx(ctx, tx, actorID,
			"user_activated",
			"user",
			targetUserID,
			map[string]interface{}{
				"previous_status": currentStatus,
				"capability_used": capability.CapGovernanceUserActivate.String(),
			},
		); err != nil {
			return err
		}

		return nil
	})
}

// BanUserRequest represents the request for banning a user.
type BanUserRequest struct {
	Reason string
}

// BanUser permanently bans a user account.
//
// MODERATION DOMAIN HARD LOCK:
// - Emits user.banned event for order processing
// - Event triggers safe refund logic in UserBanEventHandler
// - All operations are atomic within the transaction
func (s *AdminService) BanUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetUserID uuid.UUID,
	req BanUserRequest,
) error {
	// Prevent self-ban
	if actorID == targetUserID {
		return &BanSelfError{}
	}

	// 🔥 P0 SECURITY: Capability check for user banning
	if !capability.HasCapability(ctx, capability.CapGovernanceUserBan.String()) {
		return fmt.Errorf("forbidden: missing capability %s", capability.CapGovernanceUserBan.String())
	}

	// Unique per real ban transition; reused inside the transaction closure.
	banTransitionID := uuid.New()

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Get current status for audit log
		currentStatus, err := s.repo.GetUserStatus(ctx, tx, targetUserID)
		if err != nil {
			return err
		}

		// Already banned - idempotent
		if currentStatus == "banned" {
			return nil
		}

		// Update user status to banned
		if err := s.repo.UpdateUserStatus(ctx, tx, targetUserID, "banned"); err != nil {
			return err
		}

		// Log admin action (ATOMIC - within transaction)
		if err := s.adminAuditLogger.LogTx(ctx, tx, actorID,
			"user_banned",
			"user",
			targetUserID,
			map[string]interface{}{
				"reason":          req.Reason,
				"previous_status": currentStatus,
				"capability_used": capability.CapGovernanceUserBan.String(),
			},
		); err != nil {
			return err
		}

		// STEP 4 — EVENT IDEMPOTENCY:
		// Emit user.banned event for order processing
		// This event triggers the UserBanEventHandler which:
		// - Finds all active orders for the banned user
		// - Applies safe refund logic based on evidence
		// - Marks orders as processed to prevent double-handling
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"user_id":         targetUserID.String(),
				"previous_status": currentStatus,
				"reason":          req.Reason,
				"banned_by":       actorID.String(),
				"ban_event_id":    banTransitionID.String(),
			}
			idempotencyKey := userBannedTransitionIdempotencyKey(targetUserID, banTransitionID)
			if err := s.outboxRepo.InsertTx(
				ctx, tx,
				"user.banned",
				payload,
				idempotencyKey,
			); err != nil {
				return fmt.Errorf("failed to insert user.banned event: %w", err)
			}
		}

		return nil
	})
}

func userBannedTransitionIdempotencyKey(targetUserID, banTransitionID uuid.UUID) string {
	return fmt.Sprintf("%s.%s", targetUserID.String(), banTransitionID.String())
}

// BanSelfError is returned when an admin attempts to ban themselves.
type BanSelfError struct{}

func (e *BanSelfError) Error() string {
	return "cannot ban yourself"
}

// ErrCannotActivateBannedUser is returned when ActivateUser is called on
// a banned account. Ban reversal requires the explicit UnbanUser path
// with governance.user.unban capability.
type ErrCannotActivateBannedUser struct{}

func (e *ErrCannotActivateBannedUser) Error() string {
	return "cannot activate banned user: use explicit unban path with governance.user.unban capability"
}

// ErrCannotUnbanNonBannedUser is returned when UnbanUser is called on a
// user whose account_status is not "banned".
type ErrCannotUnbanNonBannedUser struct {
	CurrentStatus string
}

func (e *ErrCannotUnbanNonBannedUser) Error() string {
	return fmt.Sprintf("cannot unban user: current status is %q, not banned", e.CurrentStatus)
}

// UnbanUserRequest represents the request for unbanning a user.
type UnbanUserRequest struct {
	Reason string
}

// UnbanUser explicitly reverses a ban, setting account_status back to active.
//
// This is the ONLY path that can reverse a ban. ActivateUser rejects banned
// accounts. Requires governance.user.unban capability (separate from activate).
//
// NOTE: Unbanning does NOT restore side effects of the original ban:
// - Refunded orders remain refunded
// - Closed disputes remain closed
// - Removed for_sale items are NOT re-listed
// - Seller must re-create auctions manually
func (s *AdminService) UnbanUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetUserID uuid.UUID,
	req UnbanUserRequest,
) error {
	// 🔥 P0 SECURITY: Capability check — separate from activate
	if !capability.HasCapability(ctx, capability.CapGovernanceUserUnban.String()) {
		return fmt.Errorf("forbidden: missing capability %s", capability.CapGovernanceUserUnban.String())
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Get current status
		currentStatus, err := s.repo.GetUserStatus(ctx, tx, targetUserID)
		if err != nil {
			return err
		}

		// STRICT: Only banned users can be unbanned
		if currentStatus != "banned" {
			return &ErrCannotUnbanNonBannedUser{CurrentStatus: currentStatus}
		}

		// Update user status to active
		if err := s.repo.UpdateUserStatus(ctx, tx, targetUserID, "active"); err != nil {
			return err
		}

		// Log admin action (ATOMIC - within transaction)
		if err := s.adminAuditLogger.LogTx(ctx, tx, actorID,
			"user_unbanned",
			"user",
			targetUserID,
			map[string]interface{}{
				"reason":          req.Reason,
				"previous_status": currentStatus,
				"capability_used": capability.CapGovernanceUserUnban.String(),
			},
		); err != nil {
			return err
		}

		return nil
	})
}

// ============================================================================
// DASHBOARD METRICS
// ============================================================================

// DashboardData represents the dashboard data response.
type DashboardData struct {
	Metrics     repository.DashboardMetrics
	GeneratedAt time.Time
}

// GetDashboard returns platform metrics and summary statistics.
func (s *AdminService) GetDashboard(ctx context.Context) (*DashboardData, error) {
	var metrics *repository.DashboardMetrics

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		metrics, err = s.repo.GetDashboardMetrics(ctx, tx)
		return err
	})

	if err != nil {
		return nil, err
	}

	return &DashboardData{
		Metrics:     *metrics,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// ============================================================================
// AUDIT LOGS
// ============================================================================

// AuditLogListResult represents the result of listing audit logs.
type AuditLogListResult struct {
	Logs     []repository.AuditLogEntry
	Total    int
	Page     int
	PageSize int
}

// ListAuditLogs returns a paginated list of audit logs with optional filtering.
func (s *AdminService) ListAuditLogs(
	ctx context.Context,
	filters repository.AuditLogFilters,
) (*AuditLogListResult, error) {
	var logs []repository.AuditLogEntry
	var total int

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		logs, err = s.repo.ListAuditLogs(ctx, tx, filters)
		if err != nil {
			return err
		}

		total, err = s.repo.CountAuditLogs(ctx, tx, filters)
		return err
	})

	if err != nil {
		return nil, err
	}

	return &AuditLogListResult{
		Logs:     logs,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	}, nil
}
