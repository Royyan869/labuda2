// Package entity defines domain types for auth session management.
package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RefreshSessionStatus represents the lifecycle state of a server-side refresh session.
//
// State machine:
//
//	active ──→ consumed  (normal rotation: old token exchanged for new one)
//	active ──→ revoked   (explicit logout or logout-all)
//	active ──→ reused    (rotation attack detected: already-consumed token replayed)
//
// All terminal states (consumed, revoked, reused) are immutable once set.
type RefreshSessionStatus string

const (
	// RefreshSessionStatusActive is the only state that permits a refresh exchange.
	RefreshSessionStatusActive RefreshSessionStatus = "active"

	// RefreshSessionStatusConsumed means this token was successfully used in a rotation.
	// The replacement token has jti = replaced_by_jti in the same family.
	RefreshSessionStatusConsumed RefreshSessionStatus = "consumed"

	// RefreshSessionStatusRevoked means this token was invalidated by an explicit
	// logout, logout-all, password change, or account suspension.
	RefreshSessionStatusRevoked RefreshSessionStatus = "revoked"

	// RefreshSessionStatusReused means a previously-consumed token was replayed.
	// This signals a possible theft. All sessions in the family are revoked.
	RefreshSessionStatusReused RefreshSessionStatus = "reused"
)

// IsValid returns true for all defined status constants.
func (s RefreshSessionStatus) IsValid() bool {
	switch s {
	case RefreshSessionStatusActive,
		RefreshSessionStatusConsumed,
		RefreshSessionStatusRevoked,
		RefreshSessionStatusReused:
		return true
	}
	return false
}

// IsTerminal returns true when no further state transitions are permitted.
func (s RefreshSessionStatus) IsTerminal() bool {
	return s != RefreshSessionStatusActive
}

// RefreshSession is the domain model for a server-side refresh token record.
//
// Security invariants (enforced at DB + repository layer):
//   - TokenHash must be SHA-256 hex of the raw refresh token JWT.
//   - Raw refresh token strings are NEVER stored here.
//   - JTI must be unique across all sessions.
//   - TokenHash must be unique across all sessions.
//   - FamilyID groups all rotation-linked sessions for one logical login.
type RefreshSession struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	FamilyID        uuid.UUID
	JTI             uuid.UUID
	TokenHash       string // SHA-256 hex of raw refresh JWT; never the raw token
	Status          RefreshSessionStatus
	IssuedAt        time.Time
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	RevokedAt       *time.Time
	ReuseDetectedAt *time.Time
	ReplacedByJTI   *uuid.UUID
	FCMTokenID      *uuid.UUID // nullable link to fcm_tokens.id (push device context)
	DeviceID        *string
	DeviceName      *string
	Platform        *string
	AppVersion      *string
	IPHash          *string // hash of client IP; never raw IP
	UserAgent       *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewRefreshSession constructs a new active RefreshSession with validated fields.
// tokenHash must be the SHA-256 hex of the raw refresh token (use HashRefreshToken).
// expires must be strictly in the future.
func NewRefreshSession(
	userID uuid.UUID,
	familyID uuid.UUID,
	jti uuid.UUID,
	tokenHash string,
	expires time.Time,
) (*RefreshSession, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("refresh session: userID must not be nil")
	}
	if familyID == uuid.Nil {
		return nil, fmt.Errorf("refresh session: familyID must not be nil")
	}
	if jti == uuid.Nil {
		return nil, fmt.Errorf("refresh session: jti must not be nil")
	}
	if tokenHash == "" {
		return nil, fmt.Errorf("refresh session: tokenHash must not be empty")
	}
	if len(tokenHash) != 64 {
		return nil, fmt.Errorf("refresh session: tokenHash must be 64-char SHA-256 hex, got len %d", len(tokenHash))
	}
	now := time.Now()
	if !expires.After(now) {
		return nil, fmt.Errorf("refresh session: expires must be in the future")
	}

	return &RefreshSession{
		ID:        uuid.New(),
		UserID:    userID,
		FamilyID:  familyID,
		JTI:       jti,
		TokenHash: tokenHash,
		Status:    RefreshSessionStatusActive,
		IssuedAt:  now,
		ExpiresAt: expires,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}


