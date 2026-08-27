// DOMAIN: SOCIAL
// NOTE: User-generated content.

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Content represents user-generated content.
type Content struct {
	ID               uuid.UUID
	AuthorID         uuid.UUID
	Status           Status
	Visibility       Visibility
	Caption          *string
	City             *string
	Province         *string
	IsHidden         bool
	OriginalAuthorID *uuid.UUID
	Tags             []string // Hashtags from content_hashtags; nil when not loaded
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	CurrentStatus Status
	TargetStatus  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid content status transition: %s -> %s",
		e.CurrentStatus, e.TargetStatus)
}

// ContentNotActiveError is returned when attempting operation on non-active content.
type ContentNotActiveError struct {
	Status Status
}

func (e *ContentNotActiveError) Error() string {
	return fmt.Sprintf("content not active: current status=%s", e.Status)
}

// AlreadyDeletedError is returned when attempting to modify deleted content.
type AlreadyDeletedError struct {
	ContentID uuid.UUID
}

func (e *AlreadyDeletedError) Error() string {
	return fmt.Sprintf("content is already deleted: content_id=%s", e.ContentID)
}

// NewContent creates a new active content with caption.
func NewContent(authorID uuid.UUID, caption string) *Content {
	now := time.Now()
	return &Content{
		ID:         uuid.New(),
		AuthorID:   authorID,
		Status:     StatusActive,
		Visibility: VisibilityPublic,
		Caption:    &caption,
		IsHidden:   false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Delete transitions content to deleted state.
// ENFORCES: Deleted content cannot transition back (terminal state).
func (c *Content) Delete() error {
	// Guard: Already deleted?
	if c.Status == StatusDeleted {
		return nil // Idempotent - already deleted
	}

	// Validate state transition
	if !CanTransition(c.Status, StatusDeleted) {
		return &InvalidTransitionError{
			CurrentStatus: c.Status,
			TargetStatus:  StatusDeleted,
		}
	}

	now := time.Now()
	c.Status = StatusDeleted
	c.DeletedAt = &now
	c.UpdatedAt = now
	return nil
}

// Hide marks content as hidden without changing status.
func (c *Content) Hide() error {
	// Guard: Cannot modify deleted content
	if c.Status == StatusDeleted {
		return &AlreadyDeletedError{ContentID: c.ID}
	}

	if c.IsHidden {
		return nil // Idempotent - already hidden
	}

	c.IsHidden = true
	c.UpdatedAt = time.Now()
	return nil
}

// Unhide marks content as visible without changing status.
func (c *Content) Unhide() error {
	// Guard: Cannot modify deleted content
	if c.Status == StatusDeleted {
		return &AlreadyDeletedError{ContentID: c.ID}
	}

	if !c.IsHidden {
		return nil // Idempotent - already visible
	}

	c.IsHidden = false
	c.UpdatedAt = time.Now()
	return nil
}

// IsVisible returns true if content is visible (not hidden and not deleted).
func (c *Content) IsVisible() bool {
	return !c.IsHidden && c.Status != StatusDeleted
}

// ============================================================================
// SHARE CONTRACT V1: Repost Methods
// ============================================================================

// IsRepost returns true if this content is a repost of another content.
func (c *Content) IsRepost() bool {
	return c.OriginalAuthorID != nil
}

// MarkAsRepostWithStatus marks this content as a repost with explicit deletion status.
// This creates a reference to the original content without copying it.
//
// SHARE VALIDATION V1: Accepts explicit isDeleted flag for honest UI rendering.
// Service layer must validate original content before calling this method.
func (c *Content) MarkAsRepostWithStatus(originalContentID uuid.UUID, originalAuthorID uuid.UUID, title string, imageURL string, isDeleted bool) error {
	// Guard: Cannot modify deleted content
	if c.Status == StatusDeleted {
		return &AlreadyDeletedError{ContentID: c.ID}
	}

	originalAuthor := originalAuthorID
	c.OriginalAuthorID = &originalAuthor
	// Legacy share_reference storage has been removed; repost state now
	// derives solely from original_author_id plus the canonical occurrence row.
	_ = originalContentID
	_ = title
	_ = imageURL
	_ = isDeleted
	c.UpdatedAt = time.Now()
	return nil
}

// GetOriginalAuthorID returns the original author ID if this is a repost, nil otherwise.
func (c *Content) GetOriginalAuthorID() *uuid.UUID {
	return c.OriginalAuthorID
}
