// Package capability provides integration with the auth package for actor resolution.
package capability

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// UserStateQuerierAdapter implements the UserStateQuerier interface by querying
// the database directly. This provides all business state needed for the Actor.
type UserStateQuerierAdapter struct {
	db *db.DB
}

// NewUserStateQuerierAdapter creates a new UserStateQuerier that loads
// user state from the database.
func NewUserStateQuerierAdapter(database *db.DB) *UserStateQuerierAdapter {
	return &UserStateQuerierAdapter{
		db: database,
	}
}

// GetUserState returns the user's role, account status, profile state, and
// time-bounded seller subscription state.
//
// CRITICAL: All state is loaded atomically from a SINGLE query to prevent race conditions.
// The query joins users with seller_subscriptions to get all data in one database round-trip.
//
// Returns:
//   - role: user's role (user, admin)
//   - accountStatus: user's account status (active, suspended, banned)
//   - emailVerified: true if email is verified
//   - isIdentityComplete: true if user has established Layer B identity via username presence
//   - sellerStatus: nil if user has no seller subscription, otherwise the
//     coarsened status (active, expired)
//   - error: any error encountered during query
func (a *UserStateQuerierAdapter) GetUserState(ctx context.Context, userID uuid.UUID) (role, accountStatus string, emailVerified, isIdentityComplete bool, sellerStatus *string, err error) {
	// CRITICAL: SINGLE atomic query for all user state.
	// LEFT JOIN LATERAL fetches the latest seller subscription row so we can
	// coarsen the entitlement interval in one round-trip.
	query := `
		SELECT
			u.role,
			u.account_status,
			(u.email_verified_at IS NOT NULL) AS email_verified,
			(NULLIF(BTRIM(up.username), '') IS NOT NULL) AS has_username,
		COALESCE(ss.status::text, ''),
		COALESCE(ss.started_at, to_timestamp(0)),
		COALESCE(ss.expires_at, to_timestamp(0)),
		COALESCE(any_sub.has_subscription, false)
	FROM users u
	LEFT JOIN user_profiles up
		ON up.user_id = u.id
	LEFT JOIN LATERAL (
			SELECT status, started_at, expires_at
			FROM seller_subscriptions
			WHERE user_id = u.id
			  AND status = 'active'
			  AND started_at <= NOW()
			  AND NOW() < expires_at
			ORDER BY started_at DESC
			LIMIT 1
		) ss ON true
		LEFT JOIN LATERAL (
			SELECT true AS has_subscription
			FROM seller_subscriptions
			WHERE user_id = u.id
			LIMIT 1
		) any_sub ON true
		WHERE u.id = $1
	`
	var subscriptionStatus string
	var startedAt, expiresAt time.Time
	var hasSubscription bool

	err = a.db.Pool().QueryRow(ctx, query, userID).Scan(
		&role,
		&accountStatus,
		&emailVerified,
		&isIdentityComplete,
		&subscriptionStatus,
		&startedAt,
		&expiresAt,
		&hasSubscription,
	)
	if err != nil {
		return "", "", false, false, nil, fmt.Errorf("failed to query user state: %w", err)
	}

	if subscriptionStatus != "" {
		active := "active"
		sellerStatus = &active
	}

	if sellerStatus == nil && hasSubscription {
		expired := "expired"
		sellerStatus = &expired
	}

	return role, accountStatus, emailVerified, isIdentityComplete, sellerStatus, nil
}
