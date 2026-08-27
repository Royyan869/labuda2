package policy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// BlockChecker defines the interface for checking block relationships.
// Implementations are responsible for managing their own DB connection;
// the notification policy layer has no transaction context to pass.
type BlockChecker interface {
	ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error)
}

// BlockPolicy defines how blocks affect notification delivery.
type BlockPolicy struct {
	blockChecker BlockChecker
}

// NewBlockPolicy creates a new BlockPolicy.
func NewBlockPolicy(checker BlockChecker) *BlockPolicy {
	return &BlockPolicy{
		blockChecker: checker,
	}
}

// BlockAction represents the action to take for a blocked notification.
type BlockAction struct {
	Deliver       bool   // Should notification be delivered?
	Anonymize     bool   // Should actor identity be removed?
	ActorDisplay  string // Display text when anonymized (e.g., "Seller", "Buyer")
	Reason        string // Reason for the decision (for logging)
}

// ShouldApplyBlock determines how to handle a notification when a block exists.
//
// POLICY:
// - Commerce/Moderation: bypass block BUT anonymize actor (use role-based display)
// - Social: respect block (don't deliver)
//
// SAFE ANONYMIZATION:
// - Instead of just setting actor_id = nil, we also set actor_display
// - This preserves context: "Seller shipped your order" vs "Someone shipped your order"
func (p *BlockPolicy) ShouldApplyBlock(
	ctx context.Context,
	actorID, recipientID uuid.UUID,
	category NotificationCategory,
) BlockAction {
	// No block checker - no filtering
	if p.blockChecker == nil {
		return BlockAction{
			Deliver:      true,
			Anonymize:    false,
			ActorDisplay: "",
			Reason:       "no_block_checker",
		}
	}

	// Check if block exists
	blocked, err := p.blockChecker.ExistsBlock(ctx, actorID, recipientID)
	if err != nil {
		// On error, fail open for commerce/moderation, fail closed for social
		switch category {
		case CommerceCritical, Moderation:
			return BlockAction{
				Deliver:      true,
				Anonymize:    true, // Safest option on error
				ActorDisplay: inferActorDisplay(category),
				Reason:       fmt.Sprintf("fail_open_anonymized: block_check_error: %v", err),
			}
		default:
			return BlockAction{
				Deliver:      false,
				Anonymize:    false,
				ActorDisplay: "",
				Reason:       fmt.Sprintf("fail_closed: block_check_error: %v", err),
			}
		}
	}

	// No block exists - deliver normally
	if !blocked {
		return BlockAction{
			Deliver:      true,
			Anonymize:    false,
			ActorDisplay: "",
			Reason:       "no_block",
		}
	}

	// Block exists - apply category-specific policy
	switch category {
	case CommerceCritical:
		// Commerce bypasses block but actor identity is hidden
		// Use role-based display to preserve context
		return BlockAction{
			Deliver:      true,
			Anonymize:    true,
			ActorDisplay: inferActorDisplay(category),
			Reason:       "commerce_bypass_block_anonymized",
		}

	case Moderation:
		// Moderation bypasses block with anonymization
		return BlockAction{
			Deliver:      true,
			Anonymize:    true,
			ActorDisplay: "Admin",
			Reason:       "moderation_bypass_block_anonymized",
		}

	case Social, Marketing:
		// Social and marketing respect blocks
		return BlockAction{
			Deliver:      false,
			Anonymize:    false,
			ActorDisplay: "",
			Reason:       fmt.Sprintf("block_%s", category),
		}

	default:
		// Unknown category - fail safe: deliver but anonymize
		return BlockAction{
			Deliver:      true,
			Anonymize:    true,
			ActorDisplay: "System",
			Reason:       "unknown_category_anonymized",
		}
	}
}

// inferActorDisplay attempts to infer the actor's display role from the notification type.
// This provides context without revealing identity.
func inferActorDisplay(category NotificationCategory) string {
	// For commerce notifications, we could infer role from notification type
	// but this requires more context. For now, use generic terms.
	switch category {
	case CommerceCritical:
		return "Penjual" // "Seller" - generic, doesn't identify specific person
	case Moderation:
		return "Admin"
	default:
		return "System"
	}
}

// InferActorDisplayFromNotificationType provides more specific role inference
// based on the actual notification type and data payload.
func InferActorDisplayFromNotificationType(notifyType string, actorID, recipientID uuid.UUID, data map[string]interface{}) string {
	switch notifyType {
	case "order.shipped":
		// Recipient is buyer, actor is seller
		return "Penjual"
	case "order.cancelled":
		// Actor is the one who cancelled
		return "Pihak lain"
	case "order.refunded", "order.partially_refunded":
		// System-initiated
		return "System"
	case "order.dispute_open":
		// Dispute opened
		return "Pembeli"
	// D1A: Refund/dispute lifecycle
	case "refund.opened":
		return "Pembeli"
	case "refund.escalated":
		return "System"
	case "dispute.resolved":
		return "Admin"
	case "moderation.content.removed", "moderation.comment.removed", "moderation.for_sale.removed":
		return "Admin"
	case "moderation.content.restored", "moderation.comment.restored", "moderation.for_sale.restored":
		return "Admin"
	default:
		return "System"
	}
}


