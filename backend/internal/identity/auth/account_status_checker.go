package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
)

// AccountStatusChecker defines interface for checking user account status.
// This interface should be implemented by the user repository or a dedicated service.
type AccountStatusChecker interface {
	// EnsureActive checks if the user's account is active.
	// Returns ErrAccountSuspended, ErrAccountBanned, or ErrAccountInactive if not.
	EnsureActive(ctx context.Context, userID uuid.UUID) error

	// GetStatus retrieves the user's current account status.
	// Returns "active", "suspended", "banned", or error.
	GetStatus(ctx context.Context, userID uuid.UUID) (string, error)

	// IsBanned returns true if the user's account is banned.
	IsBanned(ctx context.Context, userID uuid.UUID) (bool, error)
}

// AccountStatusCheckerDB implements AccountStatusChecker interface using PostgreSQL database.
// This allows immediate account status enforcement without relying on Firebase claims.
type AccountStatusCheckerDB struct {
	db *db.DB
}

// NewAccountStatusCheckerDB creates a new database-backed account status checker.
func NewAccountStatusCheckerDB(database *db.DB) *AccountStatusCheckerDB {
	return &AccountStatusCheckerDB{db: database}
}

// EnsureActive checks if the user's account is active in the database.
// System callers bypass this check.
func (asc *AccountStatusCheckerDB) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	// System callers bypass account status checks
	if IsSystemCaller(userID) {
		return nil
	}

	// Validate caller ID is not nil
	if userID == uuid.Nil {
		return ErrInvalidCaller
	}

	var accountStatus string
	var deletedAt *time.Time
	err := asc.db.Pool().QueryRow(ctx, "SELECT account_status, deleted_at FROM users WHERE id = $1", userID).Scan(&accountStatus, &deletedAt)
	if err != nil {
		return classifyAccountStatusLookupError(err)
	}

	// Soft-deleted users are removed regardless of account_status.
	if deletedAt != nil {
		return ErrAccountRemoved
	}

	// Check account status and return appropriate error
	switch accountStatus {
	case "active":
		return nil
	case "suspended":
		return ErrAccountSuspended
	case "banned":
		return ErrAccountBanned
	default:
		// Handle any unexpected status values
		return ErrAccountInactive
	}
}

// classifyAccountStatusLookupError converts an account-status query error into
// its canonical account-state classification. A missing row means the account
// no longer exists (hard-deleted or never provisioned) — that is a removed
// account, not an internal failure. Anything else is a genuine lookup error.
func classifyAccountStatusLookupError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAccountRemoved
	}
	return fmt.Errorf("failed to query account status: %w", err)
}

// GetStatus retrieves the user's current account status from the database.
// Returns "removed" if deleted_at is set.
func (asc *AccountStatusCheckerDB) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	var accountStatus string
	var deletedAt *time.Time
	err := asc.db.Pool().QueryRow(ctx, "SELECT account_status, deleted_at FROM users WHERE id = $1", userID).Scan(&accountStatus, &deletedAt)
	if err != nil {
		return "", fmt.Errorf("failed to query account status: %w", err)
	}

	if deletedAt != nil {
		return "removed", nil
	}

	return accountStatus, nil
}

// IsBanned returns true if the user's account is banned.
// A removed (soft-deleted) user is NOT reported as "banned" — callers
// should use EnsureActive for the full gate.
func (asc *AccountStatusCheckerDB) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	if IsSystemCaller(userID) {
		return false, nil
	}

	var accountStatus string
	var deletedAt *time.Time
	err := asc.db.Pool().QueryRow(ctx, "SELECT account_status, deleted_at FROM users WHERE id = $1", userID).Scan(&accountStatus, &deletedAt)
	if err != nil {
		return false, fmt.Errorf("failed to query account status: %w", err)
	}

	// Removed user is not "banned" — it's removed.
	if deletedAt != nil {
		return false, nil
	}

	return accountStatus == "banned", nil
}

// EnsureActiveForOrderAction checks if both buyer and seller can perform order actions.
// This is the MODERATION DOMAIN HARD CHECK for all state-changing operations.
//
// STEP 1 — ROLE-BASED ACTION CONTROL:
// - banned users CANNOT initiate new actions
// - only system or counterparty can proceed
//
// Returns ErrAccountBanned if either party is banned.
// System caller bypasses this check.
func (asc *AccountStatusCheckerDB) EnsureActiveForOrderAction(ctx context.Context, actorID, counterpartyID uuid.UUID) error {
	// System callers bypass account status checks
	if IsSystemCaller(actorID) {
		return nil
	}

	// Check actor's account status
	if err := asc.EnsureActive(ctx, actorID); err != nil {
		return err
	}

	// CRITICAL: Also check counterparty's status for certain operations
	// This prevents completing orders where the other party is banned
	if counterpartyID != uuid.Nil {
		if err := asc.EnsureActive(ctx, counterpartyID); err != nil {
			// Return a specific error indicating the counterparty issue
			return &CounterpartyBannedError{
				CounterpartyID: counterpartyID,
				Reason:         "cannot proceed: counterparty account is not active",
			}
		}
	}

	return nil
}

// CounterpartyBannedError is returned when the counterparty's account is not active.
type CounterpartyBannedError struct {
	CounterpartyID uuid.UUID
	Reason         string
}

func (e *CounterpartyBannedError) Error() string {
	return fmt.Sprintf("counterparty %s: %s", e.CounterpartyID, e.Reason)
}

// IsCounterpartyBanned returns true if the error is a CounterpartyBannedError.
func IsCounterpartyBanned(err error) bool {
	_, ok := err.(*CounterpartyBannedError)
	return ok
}


