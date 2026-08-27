// DOMAIN: SOCIAL
// NOTE: Content engagement through likes

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TargetType represents the type of entity that can be liked.
//
// - content: All content items (posts and requests)
// - comment: Comments on content
type TargetType string

const (
	TargetTypeContent TargetType = "content"
	TargetTypeComment TargetType = "comment"
)

// Like represents a like on content by a user.
// Likes are immutable - once created, they cannot be modified.
// To remove a like, delete the record.
type Like struct {
	ContentID uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
}

// ErrTargetNotFound is returned when target doesn't exist.
type ErrTargetNotFound struct {
	TargetID   uuid.UUID
	TargetType TargetType
}

func (e *ErrTargetNotFound) Error() string {
	return fmt.Sprintf("target not found: target_type=%s, target_id=%s", e.TargetType, e.TargetID)
}

// ErrContentNotFound is returned when content doesn't exist.
type ErrContentNotFound struct {
	ContentID uuid.UUID
}

func (e *ErrContentNotFound) Error() string {
	return fmt.Sprintf("content not found: content_id=%s", e.ContentID)
}

// ErrContentDeleted is returned when trying to like deleted content.
type ErrContentDeleted struct {
	ContentID uuid.UUID
}

func (e *ErrContentDeleted) Error() string {
	return fmt.Sprintf("content is deleted: content_id=%s", e.ContentID)
}

// ErrInvalidTargetType is returned when an invalid target type is provided.
type ErrInvalidTargetType struct {
	TargetType TargetType
}

func (e *ErrInvalidTargetType) Error() string {
	return fmt.Sprintf("invalid target type: %s", e.TargetType)
}

// ErrLikeStatsInaccessible is returned when like stats are requested for a
// target that is not visible to the viewer (deleted/hidden content, blocked
// relationship, or nonexistent target). It lets the handler answer 404 without
// leaking engagement metadata.
var ErrLikeStatsInaccessible = fmt.Errorf("like stats not accessible")

// IsValidTargetType checks if a target type is valid for likes.
// Valid types: content, comment
func IsValidTargetType(t TargetType) bool {
	switch t {
	case TargetTypeContent, TargetTypeComment:
		return true
	default:
		return false
	}
}
