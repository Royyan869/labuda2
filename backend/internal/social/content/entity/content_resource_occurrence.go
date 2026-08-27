package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ContentResourceOccurrenceOperation discriminates share_to_feed from
// direct_commerce_insert_content.
type ContentResourceOccurrenceOperation string

const (
	ContentResourceOccurrenceOperationShareToFeed                 ContentResourceOccurrenceOperation = "share_to_feed"
	ContentResourceOccurrenceOperationDirectCommerceInsertContent ContentResourceOccurrenceOperation = "direct_commerce_insert_content"
)

// IsValid reports whether the operation is canonical.
func (o ContentResourceOccurrenceOperation) IsValid() bool {
	switch o {
	case ContentResourceOccurrenceOperationShareToFeed,
		ContentResourceOccurrenceOperationDirectCommerceInsertContent:
		return true
	default:
		return false
	}
}

// ContentResourceOccurrenceResourceType identifies the canonical source
// resource domain.
type ContentResourceOccurrenceResourceType string

const (
	ContentResourceOccurrenceResourceTypeProfile        ContentResourceOccurrenceResourceType = "profile"
	ContentResourceOccurrenceResourceTypeContent        ContentResourceOccurrenceResourceType = "content"
	ContentResourceOccurrenceResourceTypeForSale ContentResourceOccurrenceResourceType = "for_sale"
	ContentResourceOccurrenceResourceTypeAuction        ContentResourceOccurrenceResourceType = "auction"
)

// ResourceType is the canonical commerce/content resource type alias used by
// newer comment-security tests and commerce-facing call sites.
type ResourceType = ContentResourceOccurrenceResourceType

const (
	ResourceTypeProfile        = ContentResourceOccurrenceResourceTypeProfile
	ResourceTypeContent        = ContentResourceOccurrenceResourceTypeContent
	ResourceTypeForSale = ContentResourceOccurrenceResourceTypeForSale
	ResourceTypeAuction        = ContentResourceOccurrenceResourceTypeAuction
)

// IsValid reports whether the resource type is canonical.
func (t ContentResourceOccurrenceResourceType) IsValid() bool {
	switch t {
	case ContentResourceOccurrenceResourceTypeProfile,
		ContentResourceOccurrenceResourceTypeContent,
		ContentResourceOccurrenceResourceTypeForSale,
		ContentResourceOccurrenceResourceTypeAuction:
		return true
	default:
		return false
	}
}

// CanDirectCommerceInsert reports whether the resource type is allowed for
// direct_commerce_insert_content.
func (t ContentResourceOccurrenceResourceType) CanDirectCommerceInsert() bool {
	return t == ContentResourceOccurrenceResourceTypeForSale ||
		t == ContentResourceOccurrenceResourceTypeAuction
}

// ContentResourceOccurrenceIdentity is the client-submitted wire identity for a
// content resource occurrence.
type ContentResourceOccurrenceIdentity struct {
	Operation    ContentResourceOccurrenceOperation    `json:"operation"`
	ResourceType ContentResourceOccurrenceResourceType `json:"resource_type"`
	ResourceID   uuid.UUID                             `json:"resource_id"`
}

// ContentResourceOccurrence is the persisted canonical resource occurrence
// attached to a content row.
//
// STRICT RULES:
// - Immutable after creation
// - Exactly one source FK is non-null
// - Actor derives from the authenticated caller
// - No client preview/snapshot authority
type ContentResourceOccurrence struct {
	ContentID              uuid.UUID
	ActorID                uuid.UUID
	Operation              ContentResourceOccurrenceOperation
	ProfileSourceID        *uuid.UUID
	ContentSourceID        *uuid.UUID
	ForSaleSourceID *uuid.UUID
	AuctionSourceID        *uuid.UUID
	CreatedAt              time.Time
}

// SourceID returns the non-nil source FK.
func (o *ContentResourceOccurrence) SourceID() uuid.UUID {
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
func (o *ContentResourceOccurrence) ResourceType() ContentResourceOccurrenceResourceType {
	switch {
	case o.ProfileSourceID != nil:
		return ContentResourceOccurrenceResourceTypeProfile
	case o.ContentSourceID != nil:
		return ContentResourceOccurrenceResourceTypeContent
	case o.ForSaleSourceID != nil:
		return ContentResourceOccurrenceResourceTypeForSale
	case o.AuctionSourceID != nil:
		return ContentResourceOccurrenceResourceTypeAuction
	default:
		return ""
	}
}

// NewContentResourceOccurrence creates a new immutable occurrence row model.
func NewContentResourceOccurrence(
	contentID uuid.UUID,
	actorID uuid.UUID,
	identity *ContentResourceOccurrenceIdentity,
) *ContentResourceOccurrence {
	occurrence := &ContentResourceOccurrence{
		ContentID: contentID,
		ActorID:   actorID,
		Operation: identity.Operation,
		CreatedAt: time.Now(),
	}

	switch identity.ResourceType {
	case ContentResourceOccurrenceResourceTypeProfile:
		occurrence.ProfileSourceID = &identity.ResourceID
	case ContentResourceOccurrenceResourceTypeContent:
		occurrence.ContentSourceID = &identity.ResourceID
	case ContentResourceOccurrenceResourceTypeForSale:
		occurrence.ForSaleSourceID = &identity.ResourceID
	case ContentResourceOccurrenceResourceTypeAuction:
		occurrence.AuctionSourceID = &identity.ResourceID
	default:
		panic(fmt.Sprintf("unsupported content resource type: %s", identity.ResourceType))
	}

	return occurrence
}
