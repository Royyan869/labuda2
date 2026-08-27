package notification

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/notification/entity"
)

// Repository defines the interface for notification persistence.
type Repository interface {
	// Insert creates a new notification within a transaction.
	// Idempotent: duplicate (recipient_id, actor_id, type, entity_id) returns nil.
	Insert(ctx context.Context, tx interface{}, notification *entity.Notification) error

	// GetByID retrieves a notification by ID.
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Notification, error)

	// ListByRecipient retrieves notifications for a recipient.
	// Returns notifications ordered by created_at DESC (newest first).
	ListByRecipient(ctx context.Context, tx interface{}, recipientID uuid.UUID, limit int, offset int) ([]*entity.Notification, error)

	// MarkAsRead marks a notification as read.
	MarkAsRead(ctx context.Context, tx interface{}, id uuid.UUID) error

	// MarkAsReadByEntity marks notifications as read for a recipient by entity type and entity ID.
	// This is used for cross-domain sync (e.g., chat read → chat notifications read).
	// Only affects notifications matching: recipient_id, entity_type, and entity_id.
	MarkAsReadByEntity(ctx context.Context, tx interface{}, recipientID uuid.UUID, entityType string, entityID uuid.UUID) error

	// MarkAllAsRead marks all unread notifications for a recipient as read.
	MarkAllAsRead(ctx context.Context, tx interface{}, recipientID uuid.UUID) error

	// CountUnread counts unread notifications for a recipient.
	CountUnread(ctx context.Context, tx interface{}, recipientID uuid.UUID) (int, error)

	// Delete deletes a notification by ID.
	Delete(ctx context.Context, tx interface{}, id uuid.UUID) error

	// DeleteByActorAndRecipient deletes notifications by actor ID and recipient ID for a specific type.
	// This is used for social graph cleanup (unfollow, block).
	// Idempotent: no error if no matching notifications exist.
	DeleteByActorAndRecipient(ctx context.Context, tx interface{}, actorID, recipientID uuid.UUID, notificationType string) error

	// DeleteAllByActorAndRecipient deletes ALL notifications between two users (both directions).
	// This is used for block operations to clean up all social notifications.
	// Idempotent: no error if no matching notifications exist.
	DeleteAllByActorAndRecipient(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error
}


