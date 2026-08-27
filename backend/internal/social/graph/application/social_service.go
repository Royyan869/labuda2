package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/social/graph"
	infraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// SocialService handles social graph operations.
//
// STRICT BOUNDARY RULES:
// - NO direct financial mutations
// - NO ledger modifications
// - NO trade/offer/withdraw mutations
// - Emits outbox events for notification delivery
// - Only manages follow and block relationships
// - Block overrides follow (atomic cleanup)
type SocialService struct {
	db        Transactor
	repo      social.SocialRepository
	outboxRepo OutboxInserter
}

// OutboxInserter defines the interface for inserting outbox events.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewSocialService creates a new SocialService.
func NewSocialService(
	db Transactor,
	repo social.SocialRepository,
	outboxRepo OutboxInserter,
) *SocialService {
	return &SocialService{
		db:        db,
		repo:      repo,
		outboxRepo: outboxRepo,
	}
}

// Follow creates a follow relationship.
//
// Transaction flow:
// 1. BEGIN
// 2. Acquire advisory lock on relationship (A,B)
// 3. Check block A->B OR B->A (normal SELECT)
// 4. Insert follow
// 5. Insert outbox event for notification
// 6. COMMIT (advisory lock auto-released)
//
// Business rules:
// - Cannot follow if blocked in either direction
// - Duplicate follow is safe (no error)
// - Self-follow is prevented by CHECK constraint
// - Uses advisory locks to prevent race conditions between block and follow
func (s *SocialService) Follow(
	ctx context.Context,
	followerID, followingID uuid.UUID,
) error {
	if followerID == followingID {
		return fmt.Errorf("cannot follow self")
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Acquire advisory lock on the relationship
		// This prevents concurrent follow/block operations between the same users
		if err := s.repo.AcquireFollowLock(ctx, tx, followerID, followingID); err != nil {
			return fmt.Errorf("failed to acquire follow lock: %w", err)
		}

		// Step 2: Check for existing block (either direction)
		// Safe now because we hold the advisory lock
		blocked, err := s.repo.ExistsBlock(ctx, tx, followerID, followingID)
		if err != nil {
			return fmt.Errorf("failed to check block: %w", err)
		}
		if blocked {
			return fmt.Errorf("cannot follow: block exists between users")
		}

		// Step 3: Insert follow (idempotent due to ON CONFLICT)
		if err := s.repo.InsertFollow(ctx, tx, followerID, followingID); err != nil {
			return fmt.Errorf("failed to insert follow: %w", err)
		}

		// Step 4: Insert outbox event for notification delivery
		// EventType: events.EventUserFollowed
		// AggregateType: "user"
		// AggregateID: followingID (the user being followed)
		payload := map[string]any{
			"actor_id":    followerID,
			"recipient_id": followingID,
		}
		// Idempotency key format: user.followed.{recipientID}.{actorID}
		idempotencyKey := fmt.Sprintf("user.followed.%s.%s", followingID, followerID)
		if err := s.outboxRepo.InsertTx(ctx, tx, events.EventUserFollowed, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}

		return nil
	})
}

// Unfollow removes a follow relationship.
//
// Transaction flow:
// 1. BEGIN
// 2. Delete follow
// 3. Insert outbox event for notification
// 4. COMMIT
//
// Business rules:
// - Idempotent (no error if follow doesn't exist)
// - Emits event for notification delivery
func (s *SocialService) Unfollow(
	ctx context.Context,
	followerID, followingID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		if err := s.repo.DeleteFollow(ctx, tx, followerID, followingID); err != nil {
			return fmt.Errorf("failed to delete follow: %w", err)
		}

		// Insert outbox event for notification delivery
		// EventType: events.EventUserUnfollowed
		// AggregateType: "user"
		// AggregateID: followingID (the user being unfollowed)
		payload := map[string]any{
			"actor_id":    followerID,
			"recipient_id": followingID,
		}
		// Idempotency key format: user.unfollowed.{recipientID}.{actorID}
		idempotencyKey := fmt.Sprintf("user.unfollowed.%s.%s", followingID, followerID)
		if err := s.outboxRepo.InsertTx(ctx, tx, events.EventUserUnfollowed, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}

		return nil
	})
}

// Block creates a block relationship and removes existing follows.
//
// Transaction flow:
// 1. BEGIN
// 2. Insert block
// 3. Delete follow A->B (if exists)
// 4. Delete follow B->A (if exists)
// 5. Insert outbox event for notification
// 6. COMMIT
//
// Business rules:
// - Block overrides follow (both directions cleaned up atomically)
// - Duplicate block is safe (no error)
// - Self-block is prevented by CHECK constraint
// - Emits event for notification delivery
func (s *SocialService) Block(
	ctx context.Context,
	blockerID, blockedID uuid.UUID,
) error {
	if blockerID == blockedID {
		return fmt.Errorf("cannot block self")
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Insert block (idempotent due to ON CONFLICT)
		if err := s.repo.InsertBlock(ctx, tx, blockerID, blockedID); err != nil {
			return fmt.Errorf("failed to insert block: %w", err)
		}

		// Remove follow relationships in both directions
		if err := s.repo.DeleteFollowBothDirections(ctx, tx, blockerID, blockedID); err != nil {
			return fmt.Errorf("failed to delete follows: %w", err)
		}

		// Insert outbox event for notification delivery
		// EventType: events.EventUserBlocked
		// AggregateType: "user"
		// AggregateID: blockedID (the user being blocked)
		payload := map[string]any{
			"actor_id":    blockerID,
			"recipient_id": blockedID,
		}
		// Idempotency key format: user.blocked.{recipientID}.{actorID}
		idempotencyKey := fmt.Sprintf("user.blocked.%s.%s", blockedID, blockerID)
		if err := s.outboxRepo.InsertTx(ctx, tx, events.EventUserBlocked, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}

		return nil
	})
}

// Unblock removes a block relationship.
//
// Transaction flow:
// 1. BEGIN
// 2. Delete block
// 3. Insert outbox event for notification
// 4. COMMIT
//
// Business rules:
// - Idempotent (no error if block doesn't exist)
// - Does NOT restore follow (follow must be re-created explicitly)
// - Emits event for notification delivery
func (s *SocialService) Unblock(
	ctx context.Context,
	blockerID, blockedID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		if err := s.repo.DeleteBlock(ctx, tx, blockerID, blockedID); err != nil {
			return fmt.Errorf("failed to delete block: %w", err)
		}

		// Insert outbox event for notification delivery
		// EventType: events.EventUserUnblocked
		// AggregateType: "user"
		// AggregateID: blockedID (the user being unblocked)
		payload := map[string]any{
			"actor_id":    blockerID,
			"recipient_id": blockedID,
		}
		// Idempotency key format: user.unblocked.{recipientID}.{actorID}
		idempotencyKey := fmt.Sprintf("user.unblocked.%s.%s", blockedID, blockerID)
		if err := s.outboxRepo.InsertTx(ctx, tx, events.EventUserUnblocked, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}

		return nil
	})
}

// IsFollowing checks if user A follows user B.
func (s *SocialService) IsFollowing(
	ctx context.Context,
	followerID, followingID uuid.UUID,
) (bool, error) {
	var following bool
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		following, err = s.repo.ExistsFollow(ctx, tx, followerID, followingID)
		return err
	})

	if err != nil {
		return false, err
	}
	return following, nil
}

// IsBlocked checks if a block exists between two users (either direction).
func (s *SocialService) IsBlocked(
	ctx context.Context,
	userA, userB uuid.UUID,
) (bool, error) {
	var blocked bool
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		blocked, err = s.repo.ExistsBlock(ctx, tx, userA, userB)
		return err
	})

	if err != nil {
		return false, err
	}
	return blocked, nil
}

// ListFollowers retrieves user IDs who follow the given user.
// Returns followers ordered by created_at DESC (newest first).
// Use cursor for pagination (pass createdAt of last item from previous page).
func (s *SocialService) ListFollowers(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	var followers []uuid.UUID
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		followers, err = s.repo.ListFollowers(ctx, tx, userID, limit, cursor)
		return err
	})

	if err != nil {
		return nil, err
	}
	return followers, nil
}

// ListFollowing retrieves user IDs that the given user follows.
// Returns following ordered by created_at DESC (newest first).
// Use cursor for pagination (pass createdAt of last item from previous page).
func (s *SocialService) ListFollowing(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	var following []uuid.UUID
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		following, err = s.repo.ListFollowing(ctx, tx, userID, limit, cursor)
		return err
	})

	if err != nil {
		return nil, err
	}
	return following, nil
}

// Mute creates a mute relationship.
//
// Business rules:
// - Mute hides content but does NOT prevent interactions
// - Duplicate mute is safe (no error)
// - Self-mute is prevented by CHECK constraint
func (s *SocialService) Mute(
	ctx context.Context,
	muterID, mutedID uuid.UUID,
) error {
	if muterID == mutedID {
		return fmt.Errorf("cannot mute self")
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		if err := s.repo.InsertMute(ctx, tx, muterID, mutedID); err != nil {
			return fmt.Errorf("failed to insert mute: %w", err)
		}
		return nil
	})
}

// Unmute removes a mute relationship.
//
// Business rules:
// - Simple delete operation
// - Idempotent (no error if mute doesn't exist)
func (s *SocialService) Unmute(
	ctx context.Context,
	muterID, mutedID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		if err := s.repo.DeleteMute(ctx, tx, muterID, mutedID); err != nil {
			return fmt.Errorf("failed to delete mute: %w", err)
		}
		return nil
	})
}

// IsMuted checks if user A has muted user B.
func (s *SocialService) IsMuted(
	ctx context.Context,
	muterID, mutedID uuid.UUID,
) (bool, error) {
	var muted bool
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		muted, err = s.repo.ExistsMute(ctx, tx, muterID, mutedID)
		return err
	})

	if err != nil {
		return false, err
	}
	return muted, nil
}

// ListMuted retrieves user IDs that the given user has muted.
// Returns muted users ordered by created_at DESC (newest first).
func (s *SocialService) ListMuted(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	var muted []uuid.UUID
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		muted, err = s.repo.ListMuted(ctx, tx, userID, limit, cursor)
		return err
	})

	if err != nil {
		return nil, err
	}
	return muted, nil
}

// ListBlocked retrieves user IDs that the given user has blocked.
// Returns blocked users ordered by created_at DESC (newest first).
func (s *SocialService) ListBlocked(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	cursor *time.Time,
) ([]uuid.UUID, error) {
	var blocked []uuid.UUID
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		blocked, err = s.repo.ListBlocked(ctx, tx, userID, limit, cursor)
		return err
	})

	if err != nil {
		return nil, err
	}
	return blocked, nil
}

// NewSocialServiceWithDefaults creates a SocialService with default repository.
// Convenience function for common usage pattern.
func NewSocialServiceWithDefaults(db Transactor, outboxRepo OutboxInserter) *SocialService {
	return NewSocialService(
		db,
		infraRepo.NewSocialRepository(),
		outboxRepo,
	)
}


