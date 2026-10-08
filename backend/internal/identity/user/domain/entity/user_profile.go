package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile represents the user's profile information.
// This maps to the user_profiles table.
type UserProfile struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Username       *string
	Bio            *string
	AvatarURL      *string
	CoverPhotoURL  *string
	DateOfBirth    *time.Time
	Gender         *string
	Location       *string
	City           *string
	Province       *string
	PreferredLang  *string
	LastActiveAt   *time.Time
	FollowersCount int
	FollowingCount int
	IsVerified     bool

	// JSON fields
	SocialMedia map[string]interface{}
	Privacy     map[string]interface{}

	// Cover photo write timestamp (schema: cover_photo_updated_at).
	CoverPhotoUpdatedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SocialMedia represents the optional social media handles a user may expose
// on their profile. Presence of a value is the visibility authority: a nil
// handle means the account is not displayed. There is no separate visibility
// toggle.
type SocialMedia struct {
	InstagramHandle *string
	FacebookHandle  *string
	TwitterHandle   *string
	TiktokHandle    *string
}

// ToMap converts the handles to the persisted jsonb shape. Nil or empty
// handles are omitted so the stored object only carries present accounts
// (presence = visible).
func (s *SocialMedia) ToMap() map[string]interface{} {
	if s == nil {
		return nil
	}
	m := map[string]interface{}{}
	if s.InstagramHandle != nil && *s.InstagramHandle != "" {
		m["instagram_handle"] = *s.InstagramHandle
	}
	if s.FacebookHandle != nil && *s.FacebookHandle != "" {
		m["facebook_handle"] = *s.FacebookHandle
	}
	if s.TwitterHandle != nil && *s.TwitterHandle != "" {
		m["twitter_handle"] = *s.TwitterHandle
	}
	if s.TiktokHandle != nil && *s.TiktokHandle != "" {
		m["tiktok_handle"] = *s.TiktokHandle
	}
	return m
}

// HasAny reports whether at least one handle is present.
func (s *SocialMedia) HasAny() bool {
	if s == nil {
		return false
	}
	return (s.InstagramHandle != nil && *s.InstagramHandle != "") ||
		(s.FacebookHandle != nil && *s.FacebookHandle != "") ||
		(s.TwitterHandle != nil && *s.TwitterHandle != "") ||
		(s.TiktokHandle != nil && *s.TiktokHandle != "")
}

// UpdateProfileInput contains fields that can be updated on a user profile.
type UpdateProfileInput struct {
	Bio           *string
	Location      *string
	City          *string
	Province      *string
	AvatarURL     *string
	CoverPhotoURL *string
	Username      *string
	Gender        *string
	SocialMedia   *SocialMedia
}


