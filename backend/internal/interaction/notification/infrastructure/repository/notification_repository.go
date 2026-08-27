package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	notificationrepo "github.com/labuda/backend/internal/interaction/notification"
	"github.com/labuda/backend/internal/interaction/notification/entity"
)

// NotificationRepository implements the notification repository.
type NotificationRepository struct{}

// NewNotificationRepository creates a new NotificationRepository.
func NewNotificationRepository() notificationrepo.Repository {
	return &NotificationRepository{}
}

// Insert creates a new notification within a transaction.
// Idempotent: duplicate (recipient_id, actor_id, type, entity_id) returns nil.
func (r *NotificationRepository) Insert(ctx context.Context, tx interface{}, notification *entity.Notification) error {
	query := `
		INSERT INTO notifications (id, recipient_id, actor_id, type, entity_id, data, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (recipient_id, actor_id, type, entity_id) DO NOTHING
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).Exec(
		ctx, query,
		notification.ID, notification.RecipientID, notification.ActorID,
		notification.Type, notification.EntityID, notification.Data,
		notification.IsRead, notification.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert notification failed: %w", err)
	}

	return nil
}

// GetByID retrieves a notification by ID.
func (r *NotificationRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Notification, error) {
	query := `
		SELECT id, recipient_id, actor_id, type, entity_id, data, is_read, created_at
		FROM notifications
		WHERE id = $1
	`

	var n entity.Notification
	err := tx.(interface {
		QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	}).
		QueryRow(ctx, query, id).Scan(
		&n.ID, &n.RecipientID, &n.ActorID, &n.Type, &n.EntityID, &n.Data, &n.IsRead, &n.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &entity.ErrNotificationNotFound{NotificationID: id}
		}
		return nil, fmt.Errorf("get notification failed: %w", err)
	}

	return &n, nil
}

// ListByRecipient retrieves notifications for a recipient ordered by created_at DESC.
func (r *NotificationRepository) ListByRecipient(ctx context.Context, tx interface{}, recipientID uuid.UUID, limit int, offset int) ([]*entity.Notification, error) {
	query := `
		SELECT id, recipient_id, actor_id, type, entity_id, data, is_read, created_at
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
		var n entity.Notification
		err := row.Scan(
			&n.ID, &n.RecipientID, &n.ActorID, &n.Type, &n.EntityID, &n.Data, &n.IsRead, &n.CreatedAt,
		)
		if n.Data == nil {
			n.Data = make(map[string]interface{})
		}
		return &n, err
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
