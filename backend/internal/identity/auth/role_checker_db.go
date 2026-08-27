package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/pkg/db"
)

// RoleCheckerDB implements RoleChecker interface using PostgreSQL database.
// This allows immediate role revocation without waiting for Firebase token refresh.
type RoleCheckerDB struct {
	db                   *db.DB
	adminAuditLogger     audit.AdminAuditLogger
	accountStatusChecker *AccountStatusCheckerDB
}

// NewRoleCheckerDB creates a new database-backed role checker.
func NewRoleCheckerDB(database *db.DB, adminAuditLogger audit.AdminAuditLogger) *RoleCheckerDB {
	return &RoleCheckerDB{
		db:                   database,
		adminAuditLogger:     adminAuditLogger,
		accountStatusChecker: NewAccountStatusCheckerDB(database),
	}
}

// IsAdmin checks if the user has admin role in the database.
func (rc *RoleCheckerDB) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	if IsSystemCaller(userID) {
		return true, nil
	}

	var role string
	err := rc.db.Pool().QueryRow(ctx, "SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	if err != nil {
		return false, fmt.Errorf("failed to query user role: %w", err)
	}

	return role == "admin", nil
}

// HasActiveSellerCapability checks if the user has active seller capability.
//
// Seller capability requires all of:
// 1. Account must be operational
// 2. Seller profile must exist
// 3. Subscription interval must satisfy started_at <= now < expires_at
//
// This is the canonical check for market-facing seller operations.
func (rc *RoleCheckerDB) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	if IsSystemCaller(userID) {
		return true, nil
	}

	// ============================================================
	// GATE 1: ACCOUNT STATUS CHECK (DB-backed truth)
	// ============================================================
	// Sellers who are suspended or banned CANNOT operate, regardless of subscription.
	// This uses the backend-authoritative DB account status, not Firebase claims.
	if err := rc.accountStatusChecker.EnsureActive(ctx, userID); err != nil {
		// Account is suspended, banned, or inactive
		// Return false to indicate no capability - the error contains the reason
		return false, nil
	}

	// ============================================================
	// GATE 2: SELLER PROFILE CHECK
	// ============================================================
	// Check if user has seller profile
	var hasSellerProfile bool
	err := rc.db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM seller_profiles WHERE user_id = $1 LIMIT 1
		)
	`, userID).Scan(&hasSellerProfile)
	if err != nil {
		return false, fmt.Errorf("failed to check seller profile: %w", err)
	}

	if !hasSellerProfile {
		return false, nil
	}

	// ============================================================
	// GATE 3: SUBSCRIPTION TIME BOUNDS CHECK
	// ============================================================
	var subscriptionStatus string
	var startedAt, expiresAt time.Time
	err = rc.db.Pool().QueryRow(ctx, `
		SELECT status, started_at, expires_at
		FROM seller_subscriptions
		WHERE user_id = $1
		  AND status = 'active'
		  AND started_at <= NOW()
		  AND NOW() < expires_at
		ORDER BY started_at DESC
		LIMIT 1
	`, userID).Scan(&subscriptionStatus, &startedAt, &expiresAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check subscription: %w", err)
	}

	return true, nil
}

// hasTimeBoundedSellerCapability is the runtime fail-safe for seller authority.
// A subscription grants capability only while it is active and the current
// time is within the subscription interval.
func hasTimeBoundedSellerCapability(status string, startedAt, expiresAt, now time.Time) bool {
	if status != "active" {
		return false
	}
	if now.Before(startedAt) {
		return false
	}
	return now.Before(expiresAt)
}

// HasSellerProfile checks if the user has a seller profile.
//
// This is the canonical seller identity query. It only checks seller_profiles
// existence and does not read account lifecycle, subscription, verification,
// or tier state.
func (rc *RoleCheckerDB) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	var hasProfile bool
	err := rc.db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM seller_profiles
			WHERE user_id = $1
		)
	`, userID).Scan(&hasProfile)
	if err != nil {
		return false, fmt.Errorf("failed to check seller profile: %w", err)
	}

	return hasProfile, nil
}

// SetRole updates the user's role in the database.
// This is used by admin operations to promote/demote users.
// The callerID is logged for audit purposes.
// The role change and audit log are performed atomically in a single transaction.
//
// SLICE 5: Added no self-escalation guard - users cannot assign admin role to themselves.
func (rc *RoleCheckerDB) SetRole(ctx context.Context, callerID uuid.UUID, userID uuid.UUID, role string) error {
	validRoles := map[string]bool{
		"user":  true,
		"admin": true,
	}

	if !validRoles[role] {
		return fmt.Errorf("invalid role: %s", role)
	}

	// SLICE 5: No self-escalation guard in service layer
	// Users cannot assign the admin role to themselves
	// This is a safety boundary in case handler check is bypassed
	if callerID == userID {
		if role == "admin" {
			return fmt.Errorf("self-escalation blocked: cannot assign elevated role to self")
		}
	}

	return rc.db.WithTx(ctx, func(tx db.Tx) error {
		// Get old role for audit log
		var oldRole string
		err := tx.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", userID).Scan(&oldRole)
		if err != nil {
			return fmt.Errorf("failed to query current role: %w", err)
		}

		result, err := tx.Exec(ctx, "UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2", role, userID)
		if err != nil {
			return fmt.Errorf("failed to update user role: %w", err)
		}

		rowsAffected := result.RowsAffected()
		if rowsAffected == 0 {
			return fmt.Errorf("user not found: %s", userID)
		}

		// Log admin action (ATOMIC - within transaction)
		// If audit logging fails, the entire transaction rolls back
		if err := rc.adminAuditLogger.LogTx(ctx, tx, callerID,
			audit.ActionRoleChanged,
			audit.TargetTypeUser,
			userID,
			map[string]interface{}{
				"old_role": oldRole,
				"new_role": role,
			},
		); err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}

		return nil
	})
}

// GetRole retrieves the user's current role from the database.
func (rc *RoleCheckerDB) GetRole(ctx context.Context, userID uuid.UUID) (string, error) {
	var role string
	err := rc.db.Pool().QueryRow(ctx, "SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("failed to query user role: %w", err)
	}

	return role, nil
}
