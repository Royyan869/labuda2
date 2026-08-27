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

// SocialMedia represents social media handles.
type SocialMedia struct {
	InstagramHandle *string
	FacebookHandle  *string
	TwitterHandle   *string
	TiktokHandle    *string
	YoutubeHandle   *string
	WebsiteURL      *string
}

// PrivacySettings represents user privacy settings.
type PrivacySettings struct {
	ShowLocation         bool
	ShowPhoneNumber      bool
	ShowEmail            bool
	AllowMessagesFrom    string
	AllowTagging         bool
	ShowActivityStatus   bool
	ShowTransactionCount bool
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


