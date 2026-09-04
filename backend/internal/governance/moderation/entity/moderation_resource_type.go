package entity

// ResourceType represents the type of resource being moderated.
type ResourceType string

const (
	// ResourceTypeContent represents content posts.
	// Enforcement: content_service soft-deletes on "moderation.content.removed"
	// V1 Appeal: auto-restore on approval via "moderation.content.restored" outbox event.
	ResourceTypeContent ResourceType = "content"

	// ResourceTypeComment represents comments on content.
	// Enforcement: comment_service soft-deletes on "moderation.comment.removed"
	// V1 Appeal: auto-restore on approval via "moderation.comment.restored" outbox event.
	ResourceTypeComment ResourceType = "comment"

	// ResourceTypeForSale represents fixed-price sale surfaces.
	// Enforcement: for_sale_service marks as inactive/hidden on "moderation.for_sale.removed"
	// V1 Appeal: record-only. Approval is administrative; no auto-restoration.
	ResourceTypeForSale ResourceType = "for_sale"

	// ResourceTypeAuction represents auction forSales.
	// Enforcement: auction_service pauses/removes on "moderation.auction.removed"
	// V1 Appeal: record-only. Auction bids/timing are unrecoverable; no auto-restoration.
	ResourceTypeAuction ResourceType = "auction"

	// ResourceTypeUser represents user accounts.
	// Enforcement: user_service suspends on "moderation.user.suspended"
	// V1 Appeal: record-only. Account reinstatement is manual admin action.
	// Suspended users may appeal (POST /api/v1/appeals uses RequireAuth only).
	ResourceTypeUser ResourceType = "user"

)

// IsValid returns true if the resource type is a defined constant.
func (t ResourceType) IsValid() bool {
	switch t {
	case ResourceTypeContent, ResourceTypeComment, ResourceTypeForSale, ResourceTypeAuction, ResourceTypeUser:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (t ResourceType) String() string {
	return string(t)
}


