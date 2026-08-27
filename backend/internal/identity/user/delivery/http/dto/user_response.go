package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

// UserResponse represents the structured user response for /users/me and /users/sync
// This provides a consistent structure that the Flutter app can parse
type UserResponse struct {
	User    UserDTO    `json:"user"`
	Profile ProfileDTO `json:"profile"`
}

// UserDTO represents core user identity data
type UserDTO struct {
	ID            uuid.UUID `json:"id"`
	Email         *string   `json:"email"`
	PhoneNumber   *string   `json:"phone_number,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	AccountStatus string    `json:"account_status"` // active, suspended, banned
	Roles         []string  `json:"roles"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Verification flags (backend authority)
	IsIDVerified    *bool      `json:"is_id_verified,omitempty"`
	IsFarmVerified  *bool      `json:"is_farm_verified,omitempty"`
	PhoneVerifiedAt *time.Time `json:"phone_verified_at,omitempty"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	IDVerifiedAt    *time.Time `json:"id_verified_at,omitempty"`
	FarmVerifiedAt  *time.Time `json:"farm_verified_at,omitempty"`

	// Seller tier (NOT a role)
	SellerTier *string `json:"seller_tier,omitempty"`

	// Seller state (derived from seller_profiles and seller_subscriptions)
	// These fields provide honest seller state to Flutter - no more guessing from role
	HasSellerProfile         bool    `json:"has_seller_profile"`                   // Has created a seller profile (workspace identity)
	SellerSubscriptionStatus *string `json:"seller_subscription_status,omitempty"` // active, expired, none
	HasMarketAuthority       bool    `json:"has_market_authority"`                 // MARKET authority: has profile + active subscription

	// Warning counts (calculated from active warnings)
	ActiveWarningCount int `json:"active_warning_count"`
	SevereWarningCount int `json:"severe_warning_count"`
}

// ProfileDTO represents user profile information
type ProfileDTO struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	Username      string     `json:"username"`
	Bio           *string    `json:"bio,omitempty"`
	AvatarURL     *string    `json:"avatar_url,omitempty"`
	CoverPhotoURL *string    `json:"cover_photo_url,omitempty"`
	DateOfBirth   *time.Time `json:"date_of_birth,omitempty"`
	Gender        *string    `json:"gender,omitempty"`
	Location      *string    `json:"location,omitempty"`
	City          *string    `json:"city,omitempty"`
	Province      *string    `json:"province,omitempty"`
	PreferredLang string     `json:"preferred_lang,omitempty"`
	LastActiveAt  *time.Time `json:"last_active_at,omitempty"`

	// Social stats
	FollowersCount int `json:"followers_count"`
	FollowingCount int `json:"following_count"`

	// Social media handles
	SocialMedia *SocialMediaDTO `json:"social_media,omitempty"`

	// Privacy settings
	Privacy *PrivacySettingsDTO `json:"privacy,omitempty"`

	IsVerified *bool `json:"is_verified,omitempty"`
}

// SocialMediaDTO represents social media handles
type SocialMediaDTO struct {
	InstagramHandle *string `json:"instagram_handle,omitempty"`
	FacebookHandle  *string `json:"facebook_handle,omitempty"`
	TwitterHandle   *string `json:"twitter_handle,omitempty"`
	TiktokHandle    *string `json:"tiktok_handle,omitempty"`
	YoutubeHandle   *string `json:"youtube_handle,omitempty"`
	WebsiteURL      *string `json:"website_url,omitempty"`
}

// PrivacySettingsDTO represents privacy settings
type PrivacySettingsDTO struct {
	Visibility           string `json:"visibility"`
	ShowPhoneNumber      bool   `json:"show_phone_number"`
	ShowEmail            bool   `json:"show_email"`
	ShowLocation         bool   `json:"show_location"`
	AllowMessagesFrom    string `json:"allow_messages_from"`
	AllowTagging         bool   `json:"allow_tagging"`
	ShowActivityStatus   bool   `json:"show_activity_status"`
	ShowTransactionCount bool   `json:"show_transaction_count"`
}

// SyncUserResponse represents the response from /users/sync endpoint
type SyncUserResponse struct {
	User            UserDTO    `json:"user"`
	Profile         ProfileDTO `json:"profile"`
	Created         bool       `json:"created"`
	ProfileComplete bool       `json:"profile_complete"`
	Username        string     `json:"username"`
	AccessToken     string     `json:"access_token,omitempty"` // Platform JWT for API access
}

// PublicUserResponse represents a simplified public profile response.
// Used when viewing another user's profile (not your own).
//
// PUBLIC BOUNDARY:
//   - KYC verification flags (is_id_verified, is_farm_verified,
//     is_email_verified) are NOT exposed cross-user. They are trust-state
//     facts about the target and revealing them to an arbitrary viewer leaks
//     identity-verification telemetry that doctrine reserves for the
//     target's own self-profile and for SellerCard contexts where the
//     viewer is transacting.
//   - `is_seller` is derived from has-market-authority (active subscription
//   - seller profile present); it is the only publicly-safe trust flag
//     this surface emits.
//
// E5.1 — Identity is the canonical public-card seam per ADR-006 and
// docs/contracts/public-card-boundary.md §1.1. It carries the
// publiccard.UserCard with lifecycle populated via
// publiccard.NewWithLifecycle from viewercontext.CoarsenLifecycle. New
// consumers MUST read identity.* (and in particular identity.lifecycle for
// the public lifecycle state).
type PublicUserResponse struct {
	UserID         uuid.UUID            `json:"id"`
	Username       string               `json:"username"`
	Bio            *string              `json:"bio,omitempty"`
	AvatarURL      *string              `json:"avatar_url,omitempty"`
	CoverPhotoURL  *string              `json:"cover_photo_url,omitempty"`
	Location       *string              `json:"location,omitempty"`
	FollowersCount int                  `json:"followers_count"`
	FollowingCount int                  `json:"following_count"`
	IsSeller       bool                 `json:"is_seller"`
	Roles          []string             `json:"roles"`
	CreatedAt      time.Time            `json:"created_at"`
	Identity       *publiccard.UserCard `json:"identity"`

	// SellerTier is the public seller reputation badge.
	// Emitted ONLY when ENABLE_PUBLIC_SELLER_TIER_PROFILE=true AND both
	// user-identity lifecycle and seller-trust lifecycle are "active".
	// Values: "pro", "elite". Basic tier is never emitted (nil = no badge).
	SellerTier *string `json:"seller_tier,omitempty"`
}
