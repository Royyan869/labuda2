package models

import (
	"time"

	"github.com/google/uuid"
)

// UserDB represents the users table.
// This is the core identity table that maps Firebase Auth UIDs to PostgreSQL UUIDs.
//
// SECURITY: Firebase UID is the ONLY unique identity key. Email is stored for reference
// but is NOT unique - multiple Firebase accounts may have the same email address.
// This prevents identity takeover via email reuse.
// GORM tags removed - migration now handled via SQL
type UserDB struct {
	ID              uuid.UUID  `json:"id"`
	FirebaseUID     string     `json:"firebase_uid"` // ONLY identity key
	Email           *string    `json:"email"`        // NOT unique - duplicates allowed
	PhoneNumber     *string    `json:"phone_number,omitempty"`
	EmailVerified   bool       `json:"email_verified"`
	PhoneVerified   bool       `json:"phone_verified"`
	AccountStatus   string     `json:"account_status"`   // active, suspended, banned
	IsIDVerified    bool       `json:"is_id_verified"`   // KYC verification status
	IsFarmVerified  bool       `json:"is_farm_verified"` // Farm verification status
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	PhoneVerifiedAt *time.Time `json:"phone_verified_at,omitempty"`
	IDVerifiedAt    *time.Time `json:"id_verified_at,omitempty"`
	FarmVerifiedAt  *time.Time `json:"farm_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"` // soft delete
}

// TableName specifies the table name for UserDB.
func (UserDB) TableName() string {
	return "users"
}

// IsActive returns true if the user account is active and not deleted.
func (u *UserDB) IsActive() bool {
	return u.AccountStatus == "active" && u.DeletedAt == nil
}


