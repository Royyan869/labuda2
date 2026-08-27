package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CommentType represents the type of comment.
type CommentType string

const (
	// CommentTypeNormal is a regular text comment.
	CommentTypeNormal CommentType = "normal"
	// CommentTypeCommerceReference is a comment with an attached commerce reference.
	CommentTypeCommerceReference CommentType = "commerce_reference"
)

// IsValid checks if the comment type is valid.
func (t CommentType) IsValid() bool {
	switch t {
	case CommentTypeNormal, CommentTypeCommerceReference:
		return true
	default:
		return false
	}
}

// CommentTargetType represents the target entity type for generic comments.
//
// ZERO LEGACY MODE: Only 'content' is canonical for V1
// Comments are only for content (posts and requests)
type CommentTargetType string

const (
	// TargetContent targets a content post (canonical V1).
	TargetContent CommentTargetType = "content"
)

// IsValid checks if the target type is valid.
func (t CommentTargetType) IsValid() bool {
	return t == TargetContent
}

// Comment represents a comment on a target entity.
//
// ZERO LEGACY MODE: Only content target is canonical for V1
//
// SHARE ALIGNMENT V1: Commerce reference comments use ShareReference
// - type=commerce_reference must have reference set
// - reference.targetType = for_sale | auction
// - reference.targetId = canonical commerce resource ID
// - reference.preview = cached display data
type Comment struct {
	ID         uuid.UUID
	AuthorID   uuid.UUID
	Body       *string
	Type       CommentType
	Reference  *ShareReference   // Share reference for commerce_reference comments (fixed-price sale or auction)
	TargetID   uuid.UUID         // Target entity ID (corresponds to TargetType)
	TargetType CommentTargetType // Target entity type
	ParentID   *uuid.UUID        // Parent comment ID for threaded replies (null for top-level)
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

// ErrInvalidComment is returned when comment validation fails.
type ErrInvalidComment struct {
	Reason string
}

func (e *ErrInvalidComment) Error() string {
	return fmt.Sprintf("invalid comment: %s", e.Reason)
}

// ErrCommerceReferenceOnPost is returned when trying to add a commerce reference to non-request content.
type ErrCommerceReferenceOnPost struct {
	Reason string
}

func (e *ErrCommerceReferenceOnPost) Error() string {
	return fmt.Sprintf("commerce reference only allowed on request-type content: %s", e.Reason)
}

// NewComment creates a new normal comment.
// PRECONDITION: body must not be empty.
func NewComment(targetID, authorID uuid.UUID, body string) (*Comment, error) {
	if body == "" {
		return nil, &ErrInvalidComment{Reason: "body cannot be empty for normal comments"}
	}

	now := time.Now()
	return &Comment{
		ID:         uuid.New(),
		AuthorID:   authorID,
		Body:       &body,
		Type:       CommentTypeNormal,
		TargetID:   targetID,
		TargetType: TargetContent,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// NewCommerceReferenceComment creates a new commerce reference comment.
// Body is optional (can be empty for commerce-reference-only comments).
//
// SHARE ALIGNMENT V1: Accepts ShareReference instead of forSaleID
// - reference must have targetType = for_sale or auction
// - reference must have valid targetId
// - reference.preview contains cached display data
func NewCommerceReferenceComment(targetID, authorID uuid.UUID, reference *ShareReference, body *string) (*Comment, error) {
	now := time.Now()
	return &Comment{
		ID:         uuid.New(),
		AuthorID:   authorID,
		Body:       body,
		Type:       CommentTypeCommerceReference,
		Reference:  reference,
		TargetID:   targetID,
		TargetType: TargetContent,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Validate checks if the comment is valid according to its type.
func (c *Comment) Validate() error {
	if !c.Type.IsValid() {
		return &ErrInvalidComment{Reason: fmt.Sprintf("invalid comment type: %s", c.Type)}
	}

	switch c.Type {
	case CommentTypeNormal:
		if c.Body == nil || *c.Body == "" {
			return &ErrInvalidComment{Reason: "body cannot be empty for normal comments"}
		}
	case CommentTypeCommerceReference:
		if c.Reference == nil || !c.Reference.IsValid() || !c.Reference.TargetType.IsValid() {
			return &ErrInvalidComment{Reason: "reference is required for commerce reference comments"}
		}
	}

	return nil
}

// IsNormal returns true if this is a normal text comment.
func (c *Comment) IsNormal() bool {
	return c.Type == CommentTypeNormal
}

// IsCommerceReference returns true if this is a commerce reference comment.
func (c *Comment) IsCommerceReference() bool {
	return c.Type == CommentTypeCommerceReference
}

// IsDeleted returns true if the comment has been soft deleted.
func (c *Comment) IsDeleted() bool {
	return c.DeletedAt != nil
}

// HasParent returns true if this is a reply to another comment.
func (c *Comment) HasParent() bool {
	return c.ParentID != nil
}
