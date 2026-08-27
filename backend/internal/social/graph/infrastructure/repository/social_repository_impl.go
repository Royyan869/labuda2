package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/social/graph"
	"github.com/labuda/backend/pkg/db"
)

// SocialRepositoryImpl handles social graph persistence using pgx-based DB layer.
type SocialRepositoryImpl struct{}

// generatePairKey creates a deterministic key for a pair of UUIDs.
// The key is always the same regardless of UUID order (A:B == B:A).
// This ensures that advisory locks are consistent for bidirectional relationships.
func generatePairKey(a, b uuid.UUID) string {
	aStr := a.String()
	bStr := b.String()

	// Sort to ensure consistent key regardless of order
	if aStr < bStr {
		return aStr + ":" + bStr
	}
	return bStr + ":" + aStr
}

// NewSocialRepository creates a new SocialRepository.
func NewSocialRepository() social.SocialRepository {
	return &SocialRepositoryImpl{}
}

// AcquireFollowLock acquires an advisory lock for the relationship between two users.
// Uses pg_advisory_xact_lock which is automatically released at transaction end.
// The lock is bidirectional - AcquireFollowLock(A,B) and AcquireFollowLock(B,A) acquire the same lock.
// This prevents race conditions between follow and block operations.
func (r *SocialRepositoryImpl) AcquireFollowLock(
	ctx context.Context,
	tx interface{},
	userA, userB uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	// Generate consistent key for the pair
	key := generatePairKey(userA, userB)

	// Acquire advisory lock at transaction level
	// Lock is automatically released when transaction commits/rolls back
	_, err := dbTx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtext($1))
	`, key)

	if err != nil {
		return fmt.Errorf("acquire follow lock failed: %w", err)
	}

	return nil
}

// InsertFollow creates a new follow relationship.
// Uses ON CONFLICT DO NOTHING for idempotent behavior.
func (r *SocialRepositoryImpl) InsertFollow(
	ctx context.Context,
	tx interface{},
	followerID, followingID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO user_follows (follower_id, following_id)
		VALUES ($1, $2)
		ON CONFLICT (follower_id, following_id) DO NOTHING
	`, followerID, followingID)

	if err != nil {
		return fmt.Errorf("insert follow failed: %w", err)
	}

	return nil
}

// DeleteFollow removes a follow relationship.
func (r *SocialRepositoryImpl) DeleteFollow(
	ctx context.Context,
	tx interface{},
	followerID, followingID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		DELETE FROM user_follows
		WHERE follower_id = $1 AND following_id = $2
	`, followerID, followingID)

	if err != nil {
		return fmt.Errorf("delete follow failed: %w", err)
	}

	return nil
}

// DeleteFollowBothDirections removes follow relationships in both directions.
// Used when block is created to clean up follows atomically.
func (r *SocialRepositoryImpl) DeleteFollowBothDirections(
	ctx context.Context,
	tx interface{},
	userA, userB uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		DELETE FROM user_follows
		WHERE (follower_id = $1 AND following_id = $2)
		   OR (follower_id = $2 AND following_id = $1)
	`, userA, userB)

	if err != nil {
		return fmt.Errorf("delete follow both directions failed: %w", err)
	}

	return nil
}

// ExistsFollow checks if a follow relationship exists.
func (r *SocialRepositoryImpl) ExistsFollow(
	ctx context.Context,
	tx interface{},
	followerID, followingID uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	var exists bool
	err := dbTx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_follows
			WHERE follower_id = $1 AND following_id = $2
		)
	`, followerID, followingID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check follow exists failed: %w", err)
	}

	return exists, nil
}

// ListFollowers retrieves user IDs who follow the given user.
// Ordered by created_at DESC (newest first).
// If cursor is provided, paginates from that timestamp.
func (r *SocialRepositoryImpl) ListFollowers(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT follower_id
		FROM user_follows
		WHERE following_id = $1
	`
	args := []interface{}{userID}

	if cursor != nil {
		query += ` AND created_at < $2`
		args = append(args, *cursor)
		query += ` ORDER BY created_at DESC`
		argIdx := len(args) + 1
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	} else {
		argIdx := len(args) + 1
		query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, argIdx)
	}
	args = append(args, limit)

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list followers failed: %w", err)
	}
	defer rows.Close()

	var followers []uuid.UUID
	for rows.Next() {
		var followerID uuid.UUID
		if err := rows.Scan(&followerID); err != nil {
			return nil, fmt.Errorf("scan follower failed: %w", err)
		}
		followers = append(followers, followerID)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list followers scan failed: %w", rows.Err())
	}

	return followers, nil
}

// ListFollowing retrieves user IDs that the given user follows.
// Ordered by created_at DESC (newest first).
// If cursor is provided, paginates from that timestamp.
func (r *SocialRepositoryImpl) ListFollowing(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT following_id
		FROM user_follows
		WHERE follower_id = $1
	`
	args := []interface{}{userID}

	if cursor != nil {
		query += ` AND created_at < $2`
		args = append(args, *cursor)
		query += ` ORDER BY created_at DESC`
		argIdx := len(args) + 1
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	} else {
		argIdx := len(args) + 1
		query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, argIdx)
	}
	args = append(args, limit)

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list following failed: %w", err)
	}
	defer rows.Close()

	var following []uuid.UUID
	for rows.Next() {
		var followingID uuid.UUID
		if err := rows.Scan(&followingID); err != nil {
			return nil, fmt.Errorf("scan following failed: %w", err)
		}
		following = append(following, followingID)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list following scan failed: %w", rows.Err())
	}

	return following, nil
}

// InsertBlock creates a new block relationship.
// Uses ON CONFLICT DO NOTHING for idempotent behavior.
func (r *SocialRepositoryImpl) InsertBlock(
	ctx context.Context,
	tx interface{},
	blockerID, blockedID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id)
		VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING
	`, blockerID, blockedID)

	if err != nil {
		return fmt.Errorf("insert block failed: %w", err)
	}

	return nil
}

// DeleteBlock removes a block relationship.
func (r *SocialRepositoryImpl) DeleteBlock(
	ctx context.Context,
	tx interface{},
	blockerID, blockedID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		DELETE FROM user_blocks
		WHERE blocker_id = $1 AND blocked_id = $2
	`, blockerID, blockedID)

	if err != nil {
		return fmt.Errorf("delete block failed: %w", err)
	}

	return nil
}

// ExistsBlock checks if a block relationship exists in either direction.
// Returns true if userA blocked userB OR userB blocked userA.
func (r *SocialRepositoryImpl) ExistsBlock(
	ctx context.Context,
	tx interface{},
	userA, userB uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	var exists bool
	err := dbTx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)
	`, userA, userB).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check block exists failed: %w", err)
	}

	return exists, nil
}

// IsBlockedBy checks if blockerID has blocked targetID (directional check).
// Returns true if blockerID -> targetID block exists.
func (r *SocialRepositoryImpl) IsBlockedBy(
	ctx context.Context,
	tx interface{},
	blockerID, targetID uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	var exists bool
	err := dbTx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE blocker_id = $1 AND blocked_id = $2
		)
	`, blockerID, targetID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check directional block failed: %w", err)
	}

	return exists, nil
}

// scanUUIDRow scans a single uuid.UUID from a row.
func (r *SocialRepositoryImpl) scanUUIDRow(rows pgx.Rows) (uuid.UUID, error) {
	var id uuid.UUID
	err := rows.Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("scan uuid failed: %w", err)
	}
	return id, nil
}

// InsertMute creates a new mute relationship.
// Uses ON CONFLICT DO NOTHING for idempotent behavior.
func (r *SocialRepositoryImpl) InsertMute(
	ctx context.Context,
	tx interface{},
	muterID, mutedID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO user_mutes (muter_id, muted_id)
		VALUES ($1, $2)
		ON CONFLICT (muter_id, muted_id) DO NOTHING
	`, muterID, mutedID)

	if err != nil {
		return fmt.Errorf("insert mute failed: %w", err)
	}

	return nil
}

// DeleteMute removes a mute relationship.
func (r *SocialRepositoryImpl) DeleteMute(
	ctx context.Context,
	tx interface{},
	muterID, mutedID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		DELETE FROM user_mutes
		WHERE muter_id = $1 AND muted_id = $2
	`, muterID, mutedID)

	if err != nil {
		return fmt.Errorf("delete mute failed: %w", err)
	}

	return nil
}

// ExistsMute checks if a mute relationship exists.
func (r *SocialRepositoryImpl) ExistsMute(
	ctx context.Context,
	tx interface{},
	muterID, mutedID uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	var exists bool
	err := dbTx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_mutes
			WHERE muter_id = $1 AND muted_id = $2
		)
	`, muterID, mutedID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check mute exists failed: %w", err)
	}

	return exists, nil
}

// ListMuted retrieves user IDs that the given user has muted.
// Ordered by created_at DESC (newest first).
// If cursor is provided, paginates from that timestamp.
func (r *SocialRepositoryImpl) ListMuted(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT muted_id
		FROM user_mutes
		WHERE muter_id = $1
	`
	args := []interface{}{userID}

	if cursor != nil {
		query += ` AND created_at < $2`
		args = append(args, *cursor)
		query += ` ORDER BY created_at DESC`
		argIdx := len(args) + 1
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	} else {
		argIdx := len(args) + 1
		query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, argIdx)
	}
	args = append(args, limit)

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list muted failed: %w", err)
	}
	defer rows.Close()

	var muted []uuid.UUID
	for rows.Next() {
		var mutedID uuid.UUID
		if err := rows.Scan(&mutedID); err != nil {
			return nil, fmt.Errorf("scan muted failed: %w", err)
		}
		muted = append(muted, mutedID)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list muted scan failed: %w", rows.Err())
	}

	return muted, nil
}

// ListBlocked retrieves user IDs that the given user has blocked.
// Ordered by created_at DESC (newest first).
// If cursor is provided, paginates from that timestamp.
func (r *SocialRepositoryImpl) ListBlocked(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT blocked_id
		FROM user_blocks
		WHERE blocker_id = $1
	`
	args := []interface{}{userID}

	if cursor != nil {
		query += ` AND created_at < $2`
		args = append(args, *cursor)
		query += ` ORDER BY created_at DESC`
		argIdx := len(args) + 1
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	} else {
		argIdx := len(args) + 1
		query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, argIdx)
	}
	args = append(args, limit)

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list blocked failed: %w", err)
	}
	defer rows.Close()

	var blocked []uuid.UUID
	for rows.Next() {
		var blockedID uuid.UUID
		if err := rows.Scan(&blockedID); err != nil {
			return nil, fmt.Errorf("scan blocked failed: %w", err)
		}
		blocked = append(blocked, blockedID)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list blocked scan failed: %w", rows.Err())
	}

	return blocked, nil
}


