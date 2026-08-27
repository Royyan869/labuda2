// Package repository provides database access for auth refresh sessions.
package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	"github.com/labuda/backend/pkg/db"
)

// ErrSessionNotFound is returned when no matching session row exists.
var ErrSessionNotFound = errors.New("refresh session not found")

// ErrSessionNotActive is returned when an operation requires status='active'
// but the session is in a terminal state.
var ErrSessionNotActive = errors.New("refresh session is not active")

// HashRefreshToken returns the canonical SHA-256 hex digest of a raw refresh token string.
//
// This is the ONLY function that should be used to derive token_hash for
// auth_refresh_sessions. Raw token strings must never be stored.
//
// Usage:
//
//	hash := HashRefreshToken(rawJWT)
//	session, err := repo.FindActiveByTokenHash(ctx, tx, hash)
func HashRefreshToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}

// RefreshSessionRepository manages the auth_refresh_sessions table.
// All methods accept a db.Tx for transactional control; callers own commit/rollback.
type RefreshSessionRepository struct{}

// NewRefreshSessionRepository creates a new RefreshSessionRepository.
func NewRefreshSessionRepository() *RefreshSessionRepository {
	return &RefreshSessionRepository{}
}

// Create inserts a new active refresh session.
//
// Preconditions:
//   - s.Status must be RefreshSessionStatusActive.
//   - s.TokenHash must be the SHA-256 hex from HashRefreshToken (never raw token).
//   - s.JTI must be unique (DB enforces UNIQUE constraint).
//   - s.TokenHash must be unique (DB enforces UNIQUE constraint).
func (r *RefreshSessionRepository) Create(ctx context.Context, tx db.Tx, s *authentity.RefreshSession) error {
	if s.Status != authentity.RefreshSessionStatusActive {
		return fmt.Errorf("refresh session create: status must be active, got %s", s.Status)
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO auth_refresh_sessions (
			id, user_id, family_id, jti, token_hash, status,
			issued_at, expires_at,
			consumed_at, revoked_at, reuse_detected_at, replaced_by_jti,
			fcm_token_id, device_id, device_name, platform, app_version,
			ip_hash, user_agent,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8,
			$9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			$18, $19,
			$20, $21
		)
	`,
		s.ID, s.UserID, s.FamilyID, s.JTI, s.TokenHash, string(s.Status),
		s.IssuedAt, s.ExpiresAt,
		db.ToNullTime(s.ConsumedAt), db.ToNullTime(s.RevokedAt),
		db.ToNullTime(s.ReuseDetectedAt), nullUUIDFromPtr(s.ReplacedByJTI),
		nullUUIDFromPtr(s.FCMTokenID),
		db.ToNullString(s.DeviceID), db.ToNullString(s.DeviceName),
		db.ToNullString(s.Platform), db.ToNullString(s.AppVersion),
		db.ToNullString(s.IPHash), db.ToNullString(s.UserAgent),
		s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("refresh session create: %w", err)
	}
	return nil
}

// FindActiveByTokenHash returns the single active session matching tokenHash.
//
// tokenHash must be the SHA-256 hex from HashRefreshToken.
// Returns ErrSessionNotFound if no active row exists for this hash.
// Returns ErrSessionNotActive if the session exists but is not active
// (this distinguishes a revoked/consumed replay from a missing session).
func (r *RefreshSessionRepository) FindActiveByTokenHash(ctx context.Context, tx db.Tx, tokenHash string) (*authentity.RefreshSession, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id, user_id, family_id, jti, token_hash, status,
			issued_at, expires_at,
			consumed_at, revoked_at, reuse_detected_at, replaced_by_jti,
			fcm_token_id, device_id, device_name, platform, app_version,
			ip_hash, user_agent,
			created_at, updated_at
		FROM auth_refresh_sessions
		WHERE token_hash = $1
	`, tokenHash)

	s, err := scanRefreshSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("refresh session find by token hash: %w", err)
	}

	if s.Status != authentity.RefreshSessionStatusActive {
		return nil, ErrSessionNotActive
	}
	return s, nil
}

// FindByTokenHash returns the single refresh session matching tokenHash,
// regardless of status.
//
// tokenHash must be the SHA-256 hex from HashRefreshToken.
// Returns ErrSessionNotFound if no row exists for this hash.
func (r *RefreshSessionRepository) FindByTokenHash(ctx context.Context, tx db.Tx, tokenHash string) (*authentity.RefreshSession, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id, user_id, family_id, jti, token_hash, status,
			issued_at, expires_at,
			consumed_at, revoked_at, reuse_detected_at, replaced_by_jti,
			fcm_token_id, device_id, device_name, platform, app_version,
			ip_hash, user_agent,
			created_at, updated_at
		FROM auth_refresh_sessions
		WHERE token_hash = $1
	`, tokenHash)

	s, err := scanRefreshSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("refresh session find by token hash: %w", err)
	}
	return s, nil
}

// ConsumeAndReplace atomically marks oldJTI as consumed and inserts newSession.
//
// This is the core rotation step: the caller has validated oldJTI is active,
// then calls ConsumeAndReplace within the same transaction to:
//  1. SET status='consumed', consumed_at=now(), replaced_by_jti=newSession.JTI on old row.
//  2. INSERT new active row (newSession) in the same family_id.
//
// Atomicity is guaranteed because both operations share the caller-supplied tx.
//
// Preconditions:
//   - newSession.FamilyID must equal the old session's family_id (caller's responsibility).
//   - newSession.Status must be active.
//   - oldJTI must currently be status='active' (DB does not enforce this; caller verifies via FindActiveByTokenHash).
func (r *RefreshSessionRepository) ConsumeAndReplace(ctx context.Context, tx db.Tx, oldJTI uuid.UUID, newSession *authentity.RefreshSession) error {
	// Step 1: mark old session consumed, link to replacement.
	tag, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status        = 'consumed',
		    consumed_at   = now(),
		    replaced_by_jti = $2,
		    updated_at    = now()
		WHERE jti = $1
		  AND status = 'active'
	`, oldJTI, newSession.JTI)
	if err != nil {
		return fmt.Errorf("refresh session consume: update old jti %s: %w", oldJTI, err)
	}
	if tag.RowsAffected() == 0 {
		// Old session was not active (race or reuse attack — caller should check FindActiveByTokenHash first).
		return fmt.Errorf("refresh session consume: jti %s was not active (rows affected=0)", oldJTI)
	}

	// Step 2: insert new active session in the same family.
	if err := r.Create(ctx, tx, newSession); err != nil {
		return fmt.Errorf("refresh session consume: insert replacement: %w", err)
	}
	return nil
}

// MarkReusedAndRevokeFamily marks reusedJTI as 'reused' and revokes ALL remaining
// active sessions in the family.
//
// Called when rotation attack detection fires: a previously-consumed token was
// replayed. The entire session family is compromised; all active tokens are revoked.
func (r *RefreshSessionRepository) MarkReusedAndRevokeFamily(ctx context.Context, tx db.Tx, familyID uuid.UUID, reusedJTI uuid.UUID) error {
	now := time.Now()

	// Step 1: mark the replayed token as reused.
	_, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status             = 'reused',
		    reuse_detected_at  = $2,
		    updated_at         = now()
		WHERE jti = $1
	`, reusedJTI, now)
	if err != nil {
		return fmt.Errorf("refresh session mark reused jti %s: %w", reusedJTI, err)
	}

	// Step 2: revoke all remaining active sessions in the family (defense-in-depth).
	_, err = tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status      = 'revoked',
		    revoked_at  = $2,
		    updated_at  = now()
		WHERE family_id = $1
		  AND status    = 'active'
	`, familyID, now)
	if err != nil {
		return fmt.Errorf("refresh session revoke family %s: %w", familyID, err)
	}
	return nil
}

// RevokeByJTI revokes a specific session by JTI, validating it belongs to userID.
//
// Used for single-session logout from a known device/jti.
// Returns ErrSessionNotFound if no active session with this JTI exists for the user.
func (r *RefreshSessionRepository) RevokeByJTI(ctx context.Context, tx db.Tx, userID uuid.UUID, jti uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status     = 'revoked',
		    revoked_at = now(),
		    updated_at = now()
		WHERE jti     = $1
		  AND user_id = $2
		  AND status  = 'active'
	`, jti, userID)
	if err != nil {
		return fmt.Errorf("refresh session revoke by jti %s: %w", jti, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeFamily revokes all active sessions in a family belonging to userID.
//
// Used for logout-this-device when the family_id is known.
func (r *RefreshSessionRepository) RevokeFamily(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status     = 'revoked',
		    revoked_at = now(),
		    updated_at = now()
		WHERE user_id   = $1
		  AND family_id = $2
		  AND status    = 'active'
	`, userID, familyID)
	if err != nil {
		return fmt.Errorf("refresh session revoke family %s for user %s: %w", familyID, userID, err)
	}
	return nil
}

// RevokeFamilyCount revokes all active sessions in a family belonging to userID
// and returns the number of rows updated.
func (r *RefreshSessionRepository) RevokeFamilyCount(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status     = 'revoked',
		    revoked_at = now(),
		    updated_at = now()
		WHERE user_id   = $1
		  AND family_id = $2
		  AND status    = 'active'
	`, userID, familyID)
	if err != nil {
		return 0, fmt.Errorf("refresh session revoke family %s for user %s: %w", familyID, userID, err)
	}
	return tag.RowsAffected(), nil
}

// RevokeAllForUser revokes ALL active sessions for a user.
//
// Used on: logout-all, password change, account suspension, account ban.
// Does NOT affect other users.
func (r *RefreshSessionRepository) RevokeAllForUser(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status     = 'revoked',
		    revoked_at = now(),
		    updated_at = now()
		WHERE user_id = $1
		  AND status  = 'active'
	`, userID)
	if err != nil {
		return fmt.Errorf("refresh session revoke all for user %s: %w", userID, err)
	}
	return nil
}

// RevokeAllForUserCount revokes ALL active sessions for a user and returns the
// number of rows updated.
//
// Used on: logout-all, password change, account suspension, account ban.
// Does NOT affect other users.
func (r *RefreshSessionRepository) RevokeAllForUserCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET status     = 'revoked',
		    revoked_at = now(),
		    updated_at = now()
		WHERE user_id = $1
		  AND status  = 'active'
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("refresh session revoke all for user %s: %w", userID, err)
	}
	return tag.RowsAffected(), nil
}

// ListActiveByUser returns all currently active sessions for a user, most recent first.
//
// Used for the future "Login Sessions" UI (Phase 3).
// Only returns sessions with status='active' and expires_at in the future.
func (r *RefreshSessionRepository) ListActiveByUser(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*authentity.RefreshSession, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id, user_id, family_id, jti, token_hash, status,
			issued_at, expires_at,
			consumed_at, revoked_at, reuse_detected_at, replaced_by_jti,
			fcm_token_id, device_id, device_name, platform, app_version,
			ip_hash, user_agent,
			created_at, updated_at
		FROM auth_refresh_sessions
		WHERE user_id  = $1
		  AND status   = 'active'
		  AND expires_at > now()
		ORDER BY issued_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("refresh session list active for user %s: %w", userID, err)
	}
	defer rows.Close()

	var sessions []*authentity.RefreshSession
	for rows.Next() {
		s, err := scanRefreshSession(rows)
		if err != nil {
			return nil, fmt.Errorf("refresh session list scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("refresh session list rows: %w", err)
	}
	return sessions, nil
}

// DeleteExpired removes all sessions with expires_at before the given cutoff,
// regardless of status. Returns the number of rows deleted.
//
// Intended for a periodic cleanup worker to prevent unbounded table growth.
func (r *RefreshSessionRepository) DeleteExpired(ctx context.Context, tx db.Tx, before time.Time) (int64, error) {
	tag, err := tx.Exec(ctx, `
		DELETE FROM auth_refresh_sessions
		WHERE expires_at < $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("refresh session delete expired: %w", err)
	}
	return tag.RowsAffected(), nil
}

// --- internal helpers ---

// scanner is the common interface satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanRefreshSession scans one row from auth_refresh_sessions into a RefreshSession.
func scanRefreshSession(row scanner) (*authentity.RefreshSession, error) {
	var s authentity.RefreshSession
	var statusStr string

	// Nullable fields
	var consumedAt, revokedAt, reuseDetectedAt sql.NullTime
	var replacedByJTIStr, fcmTokenIDStr sql.NullString
	var deviceID, deviceName, platform, appVersion, ipHash, userAgent sql.NullString

	err := row.Scan(
		&s.ID, &s.UserID, &s.FamilyID, &s.JTI, &s.TokenHash, &statusStr,
		&s.IssuedAt, &s.ExpiresAt,
		&consumedAt, &revokedAt, &reuseDetectedAt, &replacedByJTIStr,
		&fcmTokenIDStr, &deviceID, &deviceName, &platform, &appVersion,
		&ipHash, &userAgent,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.Status = authentity.RefreshSessionStatus(statusStr)
	s.ConsumedAt = db.ToTimePtr(consumedAt)
	s.RevokedAt = db.ToTimePtr(revokedAt)
	s.ReuseDetectedAt = db.ToTimePtr(reuseDetectedAt)
	s.ReplacedByJTI = db.ToUUIDPtr(replacedByJTIStr)
	s.FCMTokenID = db.ToUUIDPtr(fcmTokenIDStr)
	s.DeviceID = db.ToStringPtr(deviceID)
	s.DeviceName = db.ToStringPtr(deviceName)
	s.Platform = db.ToStringPtr(platform)
	s.AppVersion = db.ToStringPtr(appVersion)
	s.IPHash = db.ToStringPtr(ipHash)
	s.UserAgent = db.ToStringPtr(userAgent)

	return &s, nil
}

// nullUUIDFromPtr converts *uuid.UUID to sql.NullString for nullable UUID columns.
func nullUUIDFromPtr(id *uuid.UUID) sql.NullString {
	if id == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: id.String(), Valid: true}
}


