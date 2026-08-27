package policy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// AccountStatusChecker defines the interface for checking user account status.
type AccountStatusChecker interface {
	GetStatus(ctx context.Context, userID uuid.UUID) (string, error)
}

// AccountStatusFilter defines filtering rules by account status.
type AccountStatusFilter struct {
	statusChecker AccountStatusChecker
}

// NewAccountStatusFilter creates a new AccountStatusFilter.
func NewAccountStatusFilter(checker AccountStatusChecker) *AccountStatusFilter {
	return &AccountStatusFilter{
		statusChecker: checker,
	}
}

// DeliveryDecision represents the filtering decision for a notification.
type DeliveryDecision struct {
	AllowDB    bool  // Allow storing in database (in-app notification)
	AllowPush  bool  // Allow sending push notification
	Reason     string // Reason for the decision (for logging)
}

// ShouldDeliver determines if a notification should be delivered based on:
// 1. Recipient's account status
// 2. Notification category
// 3. Notification type (for push priority filtering)
//
// FAIL-SAFE BEHAVIOR:
// - If account status check fails: Allow DB only, block push
// - This ensures no unsafe pushes are sent when we're uncertain about status
func (f *AccountStatusFilter) ShouldDeliver(
	ctx context.Context,
	recipientID uuid.UUID,
	category NotificationCategory,
	notifyType string,
) DeliveryDecision {
	// Get account status
	status, err := f.statusChecker.GetStatus(ctx, recipientID)
	if err != nil {
		// FAIL-SAFE: On error, allow DB only but block push
		// This prevents sending pushes to users who shouldn't receive them
		// while still ensuring the notification is available in-app
		return DeliveryDecision{
			AllowDB:   true,
			AllowPush: false,
			Reason:    fmt.Sprintf("fail_safe_db_only: status_check_error: %v", err),
		}
	}

	switch status {
	case "active":
		// Active users receive all notifications
		// Push is sent only for priority types (dispute, withdrawal, support)
		return DeliveryDecision{
			AllowDB:   true,
			AllowPush: RequiresPushByType(notifyType),
			Reason:    "active_user",
		}

	case "suspended", "banned":
		switch category {
		case CommerceCritical, Moderation:
			// Suspended/banned users still receive critical and moderation notifications
			// Push is sent only for priority types (dispute, withdrawal, support)
			return DeliveryDecision{
				AllowDB:   true,
				AllowPush: RequiresPushByType(notifyType),
				Reason:    fmt.Sprintf("critical_to_%s", status),
			}
		case Social, Marketing:
			// Suspended/banned users do NOT receive social or marketing notifications
			return DeliveryDecision{
				AllowDB:   false,
				AllowPush: false,
				Reason:    fmt.Sprintf("block_%s_for_%s", category, status),
			}
		default:
			// Unknown category - block for suspended/banned users
			return DeliveryDecision{
				AllowDB:   false,
				AllowPush: false,
				Reason:    fmt.Sprintf("block_unknown_for_%s", status),
			}
		}

	case "removed":
		// Soft-deleted users receive no notifications.
		// GetStatus() returns "removed" when deleted_at != nil.
		// Writing in-app notifications for deleted accounts is wasteful and
		// leaks activity into rows that will never be read.
		return DeliveryDecision{
			AllowDB:   false,
			AllowPush: false,
			Reason:    "deleted_user",
		}

	default:
		// Unknown account status - fail safe
		return DeliveryDecision{
			AllowDB:   true,
			AllowPush: false,
			Reason:    fmt.Sprintf("fail_safe_db_only: unknown_status_%s", status),
		}
	}
}

// ShouldDeliverFromActor checks if a social notification should be delivered
// based on the actor's (sender's) current account status at delivery time.
//
// Called for Social/Marketing categories after the block check passes.
// Prevents banned or deleted actors from having their identity present in
// delivered notifications, even for historical outbox events.
//
// FAIL-SAFE: On status-check error, fail closed (drop) — we cannot verify
// the actor is safe to surface.
func (f *AccountStatusFilter) ShouldDeliverFromActor(
	ctx context.Context,
	actorID uuid.UUID,
	category NotificationCategory,
) DeliveryDecision {
	status, err := f.statusChecker.GetStatus(ctx, actorID)
	if err != nil {
		return DeliveryDecision{
			AllowDB:   false,
			AllowPush: false,
			Reason:    fmt.Sprintf("fail_closed_actor: status_check_error: %v", err),
		}
	}

	switch status {
	case "active":
		return DeliveryDecision{AllowDB: true, AllowPush: true, Reason: "active_actor"}
	case "suspended":
		// Message was committed before suspension; CHAT-1 prevents new sends.
		// Deliver in-app only — push for suspended actor is suppressed.
		return DeliveryDecision{AllowDB: true, AllowPush: false, Reason: "suspended_actor_committed"}
	default:
		// banned, deleted, or unknown: drop to prevent identity leak.
		return DeliveryDecision{
			AllowDB:   false,
			AllowPush: false,
			Reason:    fmt.Sprintf("drop_actor_%s", status),
		}
	}
}


