package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ResourceOccurrenceOperation discriminates share_to_chat from
// direct_commerce_insert_chat.
type ResourceOccurrenceOperation string

const (
	ResourceOccurrenceOperationShareToChat            ResourceOccurrenceOperation = "share_to_chat"
	ResourceOccurrenceOperationDirectCommerceInsertChat ResourceOccurrenceOperation = "direct_commerce_insert_chat"
)

// IsValid returns true for known operations.
func (o ResourceOccurrenceOperation) IsValid() bool {
	switch o {
	case ResourceOccurrenceOperationShareToChat, ResourceOccurrenceOperationDirectCommerceInsertChat:
		return true
	default:
		return false
	}
}

// ResourceOccurrenceResourceType identifies the canonical resource domain.
type ResourceOccurrenceResourceType string

const (
	ResourceOccurrenceResourceTypeProfile        ResourceOccurrenceResourceType = "profile"
	ResourceOccurrenceResourceTypeContent        ResourceOccurrenceResourceType = "content"
	ResourceOccurrenceResourceTypeForSale ResourceOccurrenceResourceType = "for_sale"
	ResourceOccurrenceResourceTypeAuction        ResourceOccurrenceResourceType = "auction"
)

// IsValid returns true for known resource types.
func (t ResourceOccurrenceResourceType) IsValid() bool {
	switch t {
	case ResourceOccurrenceResourceTypeProfile,
		ResourceOccurrenceResourceTypeContent,
		ResourceOccurrenceResourceTypeForSale,
		ResourceOccurrenceResourceTypeAuction:
		return true
	default:
		return false
	}
}

// CanDirectCommerceInsert returns true when the resource type is permitted
// for direct_commerce_insert_chat operations.
func (t ResourceOccurrenceResourceType) CanDirectCommerceInsert() bool {
	return t == ResourceOccurrenceResourceTypeForSale ||
		t == ResourceOccurrenceResourceTypeAuction
}

// ResourceOccurrenceIdentity is the client-submitted wire identity for a
// resource occurrence — operation + type + ID. No preview/snapshot/fallback.
type ResourceOccurrenceIdentity struct {
	Operation    ResourceOccurrenceOperation   `json:"operation"`
	ResourceType ResourceOccurrenceResourceType `json:"resource_type"`
	ResourceID   uuid.UUID                     `json:"resource_id"`
}

// ChatMessageResourceOccurrence is the persisted resource occurrence attached
// to a chat message.
//
// STRICT RULES:
// - Immutable after creation
// - Exactly one source FK is non-null (enforced by DB CHECK)
// - Actor derives from chat_messages.sender_id
// - Room derives from chat_messages.room_id
// - Fallback is server-built; never client-submitted
type ChatMessageResourceOccurrence struct {
	MessageID              uuid.UUID
	Operation              ResourceOccurrenceOperation
	ProfileSourceID        *uuid.UUID
	ContentSourceID        *uuid.UUID
	ForSaleSourceID *uuid.UUID
	AuctionSourceID        *uuid.UUID
	FallbackSnapshot       json.RawMessage
	CreatedAt              time.Time
}

// SourceID returns the non-nil source FK value.
func (o *ChatMessageResourceOccurrence) SourceID() uuid.UUID {
	switch {
	case o.ProfileSourceID != nil:
		return *o.ProfileSourceID
	case o.ContentSourceID != nil:
		return *o.ContentSourceID
	case o.ForSaleSourceID != nil:
		return *o.ForSaleSourceID
	case o.AuctionSourceID != nil:
		return *o.AuctionSourceID
	default:
		return uuid.Nil
	}
}

// ResourceType derives the resource type from the non-null FK.
func (o *ChatMessageResourceOccurrence) ResourceType() ResourceOccurrenceResourceType {
	switch {
	case o.ProfileSourceID != nil:
		return ResourceOccurrenceResourceTypeProfile
	case o.ContentSourceID != nil:
		return ResourceOccurrenceResourceTypeContent
	case o.ForSaleSourceID != nil:
		return ResourceOccurrenceResourceTypeForSale
	case o.AuctionSourceID != nil:
		return ResourceOccurrenceResourceTypeAuction
	default:
		return ""
	}
}

// NewChatMessageResourceOccurrence creates a new occurrence.
func NewChatMessageResourceOccurrence(
	messageID uuid.UUID,
	operation ResourceOccurrenceOperation,
	resourceType ResourceOccurrenceResourceType,
	resourceID uuid.UUID,
	fallbackSnapshot json.RawMessage,
) *ChatMessageResourceOccurrence {
	o := &ChatMessageResourceOccurrence{
		MessageID:        messageID,
		Operation:        operation,
		FallbackSnapshot: fallbackSnapshot,
		CreatedAt:        time.Now(),
	}
	switch resourceType {
	case ResourceOccurrenceResourceTypeProfile:
		o.ProfileSourceID = &resourceID
	case ResourceOccurrenceResourceTypeContent:
		o.ContentSourceID = &resourceID
	case ResourceOccurrenceResourceTypeForSale:
		o.ForSaleSourceID = &resourceID
	case ResourceOccurrenceResourceTypeAuction:
		o.AuctionSourceID = &resourceID
	}
	return o
}
