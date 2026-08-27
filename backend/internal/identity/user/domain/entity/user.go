package entity

import (
	"time"

	"github.com/google/uuid"
)

// User represents the core identity of a user in the system.
// This maps to the users table.
type User struct {
	ID              uuid.UUID
	FirebaseUID     string
	Email           *string
	PhoneNumber     *string
	EmailVerified   bool
	PhoneVerified   bool
	AccountStatus   string // active, suspended, banned
	IsIDVerified    bool
	IsFarmVerified  bool
	EmailVerifiedAt *time.Time
	PhoneVerifiedAt *time.Time
	IDVerifiedAt    *time.Time
	FarmVerifiedAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// IsActive returns true if the user account is active and not deleted.
func (u *User) IsActive() bool {
	return u.AccountStatus == "active" && u.DeletedAt == nil
}

// GetDeletedAt implements the guard.UserLike interface for auth gate checks.
func (u *User) GetDeletedAt() *time.Time {
	return u.DeletedAt
}

// GetAccountStatus implements the guard.UserLike interface for auth gate checks.
func (u *User) GetAccountStatus() string {
	return u.AccountStatus
}

// UserPublicInfo represents publicly viewable user information.
// Used for displaying user profiles to other users.
//
// E5.1 — AccountStatus + IsDeleted carry raw lifecycle truth INSIDE the
// service boundary so viewercontext.CoarsenLifecycle can coarsen them at the
// single canonical mapping site. These fields are NEVER serialized to the
// wire; only the coarsened publiccard.UserCard.Lifecycle string crosses the
// public-card boundary per docs/contracts/public-card-boundary.md §4.2.
type UserPublicInfo struct {
	UserID        uuid.UUID
	Username      string
	Bio           *string
	AvatarURL     *string
	CoverPhotoURL *string
	Location      *string

	// Social stats (public)
	FollowersCount int
	FollowingCount int

	// Verification status (public)
	IsIDVerified    bool
	IsFarmVerified  bool
	IsEmailVerified bool

	// Role information (public)
	IsSeller bool
	Roles    []string

	CreatedAt string

	// Seller state
	HasSellerProfile bool

	// Raw lifecycle truth — service-internal, coarsened at emit.
	AccountStatus string
	IsDeleted     bool
}

// MyProfileResponse represents the complete user profile for the authenticated user.
type MyProfileResponse struct {
	User    *User
	Profile *UserProfile
	Roles   []string
}

// UserRole represents a user role assignment.
type UserRole struct {
	UserID uuid.UUID
	Role   string
}


