package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	notificationrepo "github.com/labuda/backend/internal/interaction/notification"
	"github.com/labuda/backend/internal/interaction/notification/entity"
)

// notificationColumns is the canonical projection order for every read. The
// actor columns are appended after created_at so the positional binding stays
// stable: id, recipient_id, actor_id, type, entity_id, data, is_read,
// created_at, actor_kind, actor_display.
const notificationColumns = "id, recipient_id, actor_id, type, entity_id, data, is_read, created_at, actor_kind, actor_display"

// rowScanner is satisfied by both pgx.Row and pgx.CollectableRow.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanNotification reconstructs a Notification from the canonical projection.
func scanNotification(row rowScanner) (*entity.Notification, error) {
	var n entity.Notification
	var actorID *uuid.UUID
	var actorKind string
	var actorDisplay string

	err := row.Scan(
		&n.ID, &n.RecipientID, &actorID, &n.Type, &n.EntityID, &n.Data, &n.IsRead, &n.CreatedAt,
		&actorKind, &actorDisplay,
	)
	if err != nil {
		return nil, err
	}

	actor, err := entity.NewActor(entity.ActorKind(actorKind), actorID, actorDisplay)
	if err != nil {
		return nil, fmt.Errorf("scan notification actor: %w", err)
	}
	n.Actor = actor

	if n.Data == nil {
		n.Data = make(map[string]interface{})
	}

	return &n, nil
}

// NotificationRepository implements the notification repository.
type NotificationRepository struct{}

// NewNotificationRepository creates a new NotificationRepository.
func NewNotificationRepository() notificationrepo.Repository {
	return &NotificationRepository{}
}

// Insert creates a new notification within a transaction and reports whether a
// row was actually written.
//
// Idempotent: a replay of the same (recipient_id, actor_key, type, entity_id)
// returns (uuid.Nil, false, nil). actor_key is the generated
// COALESCE(actor_id, zero) column, so dedup keeps its exact old meaning for
// visible humans and becomes deterministic for system/anonymized actors.
func (r *NotificationRepository) Insert(ctx context.Context, tx interface{}, notification *entity.Notification) (uuid.UUID, bool, error) {
	if err := notification.Actor.Validate(); err != nil {
		return uuid.Nil, false, fmt.Errorf("insert notification: %w", err)
	}

	query := `
		INSERT INTO notifications (id, recipient_id, actor_id, type, entity_id, data, is_read, created_at, actor_kind, actor_display)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (recipient_id, actor_key, type, entity_id) DO NOTHING
		RETURNING id
	`

	id := notification.ID
	err := tx.(interface {
		QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	}).QueryRow(
		ctx, query,
		notification.ID, notification.RecipientID, notification.Actor.StorageUserID(),
		notification.Type, notification.EntityID, notification.Data,
		notification.IsRead, notification.CreatedAt,
		string(notification.Actor.Kind()), notification.Actor.Display(),
	).Scan(&id)

	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows for a duplicate — the
		// canonical idempotent no-op, not a failure.
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("insert notification failed: %w", err)
	}

	return id, true, nil
}

// GetByID retrieves a notification by ID.
func (r *NotificationRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Notification, error) {
	query := `
		SELECT ` + notificationColumns + `
		FROM notifications
		WHERE id = $1
	`

	n, err := scanNotification(tx.(interface {
		QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	}).QueryRow(ctx, query, id))

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &entity.ErrNotificationNotFound{NotificationID: id}
		}
		return nil, fmt.Errorf("get notification failed: %w", err)
	}

	return n, nil
}

// ListByRecipient retrieves notifications for a recipient ordered by created_at DESC.
func (r *NotificationRepository) ListByRecipient(ctx context.Context, tx interface{}, recipientID uuid.UUID, limit int, offset int) ([]*entity.Notification, error) {
	query := `
		SELECT ` + notificationColumns + `
		FROM notifications
		WHERE recipient_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := tx.(interface {
		Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	}).
		Query(ctx, query, recipientID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications failed: %w", err)
	}
	defer rows.Close()

	notifications, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*entity.Notification, error) {
		return scanNotification(row)
	})

	if err != nil {
		return nil, fmt.Errorf("scan notifications failed: %w", err)
	}

	return notifications, nil
}

// MarkAsRead marks a notification as read.
func (r *NotificationRepository) MarkAsRead(ctx context.Context, tx interface{}, id uuid.UUID) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE id = $1
	`

	result, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark notification as read failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &entity.ErrNotificationNotFound{NotificationID: id}
	}

	return nil
}

// MarkAsReadByEntity marks notifications as read for a recipient by entity type and entity ID.
// This is used for cross-domain sync (e.g., chat read → chat notifications read).
// Only affects notifications matching: recipient_id, type (entityType), and entity_id.
func (r *NotificationRepository) MarkAsReadByEntity(ctx context.Context, tx interface{}, recipientID uuid.UUID, entityType string, entityID uuid.UUID) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE recipient_id = $1 AND type = $2 AND entity_id = $3 AND is_read = false
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).Exec(ctx, query, recipientID, entityType, entityID)
	if err != nil {
		return fmt.Errorf("mark notifications as read by entity failed: %w", err)
	}

	return nil
}

// MarkAllAsRead marks all unread notifications for a recipient as read.
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, tx interface{}, recipientID uuid.UUID) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE recipient_id = $1 AND is_read = false
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).Exec(ctx, query, recipientID)
	if err != nil {
		return fmt.Errorf("mark all notifications as read failed: %w", err)
	}

	return nil
}

// CountUnread counts unread notifications for a recipient.
func (r *NotificationRepository) CountUnread(ctx context.Context, tx interface{}, recipientID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM notifications
		WHERE recipient_id = $1 AND is_read = false
	`

	var count int
	err := tx.(interface {
		QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	}).
		QueryRow(ctx, query, recipientID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications failed: %w", err)
	}

	return count, nil
}

// Delete deletes a notification by ID.
func (r *NotificationRepository) Delete(ctx context.Context, tx interface{}, id uuid.UUID) error {
	query := `DELETE FROM notifications WHERE id = $1`

	result, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete notification failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &entity.ErrNotificationNotFound{NotificationID: id}
	}

	return nil
}

// DeleteContentLikedNotification removes the content.liked notification a
// liker produced on a content. Called on UNLIKE (inside the unlike transaction)
// so a later LIKE is a new occurrence and can notify again.
// Idempotent: no error if no matching notification exists.
func (r *NotificationRepository) DeleteContentLikedNotification(ctx context.Context, tx interface{}, likerID, contentID uuid.UUID) error {
	query := `
		DELETE FROM notifications
		WHERE actor_id = $1 AND type = 'content.liked' AND entity_id = $2
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, likerID, contentID)
	if err != nil {
		return fmt.Errorf("delete content.liked notification failed: %w", err)
	}

	return nil
}

// DeleteByActorAndRecipient deletes notifications by actor ID and recipient ID for a specific type.
// This is used for social graph cleanup (unfollow, block).
// Idempotent: no error if no matching notifications exist.
func (r *NotificationRepository) DeleteByActorAndRecipient(ctx context.Context, tx interface{}, actorID, recipientID uuid.UUID, notificationType string) error {
	query := `
		DELETE FROM notifications
		WHERE actor_id = $1 AND recipient_id = $2 AND type = $3
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, actorID, recipientID, notificationType)
	if err != nil {
		return fmt.Errorf("delete notifications by actor and recipient failed: %w", err)
	}

	return nil
}

// DeleteAllByActorAndRecipient deletes ALL notifications between two users (both directions).
// This is used for block operations to clean up all social notifications.
// Idempotent: no error if no matching notifications exist.
func (r *NotificationRepository) DeleteAllByActorAndRecipient(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
	query := `
		DELETE FROM notifications
		WHERE (actor_id = $1 AND recipient_id = $2)
		   OR (actor_id = $2 AND recipient_id = $1)
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, userA, userB)
	if err != nil {
		return fmt.Errorf("delete all notifications between users failed: %w", err)
	}

	return nil
}
