// Package commercegov is the canonical commerce violation + restriction
// authority for auction settlement failures (and future commerce defaults).
//
// It replaces the obsolete buyer_bnr_strikes machinery:
//   - commerce_violations  — immutable, append-only violation history
//   - commerce_restrictions — one active row per user, EXTEND stacking
//
// Business rules (locked):
//   - 1st violation  -> 7 days
//   - 2nd violation  -> 15 days
//   - 3rd+ violation -> 30 days
//   - EXTEND stacking: new restricted_until = max(now, current restricted_until) + duration
//   - Violation history is immutable (DB trigger enforced)
//   - No decay, no admin reset, no permanent ban
package commercegov

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// ViolationType is the canonical commerce violation taxonomy.
type ViolationType string

const (
	// ViolationBuyerShippingTimeout is a buyer who failed to resolve shipping
	// within auction.end_at + 24h.
	ViolationBuyerShippingTimeout ViolationType = "buyer_shipping_timeout"
	// ViolationSellerShippingDefault is a seller who failed to provide a
	// required private quote within auction.end_at + 24h.
	ViolationSellerShippingDefault ViolationType = "seller_shipping_default"
	// ViolationBuyerBNR is a buyer who failed to pay after shipping was
	// resolved (payment window shipping_resolved_at + 24h elapsed).
	ViolationBuyerBNR ViolationType = "buyer_bnr"
)

// IsValid reports whether t is a canonical violation type.
func (t ViolationType) IsValid() bool {
	switch t {
	case ViolationBuyerShippingTimeout, ViolationSellerShippingDefault, ViolationBuyerBNR:
		return true
	}
	return false
}

// SourceType is the canonical commerce surface type of a violation source.
type SourceType string

const (
	SourceTypeAuction SourceType = "auction"
	SourceTypeForSale SourceType = "for_sale"
)

// RestrictionDuration returns the canonical restriction duration for the
// violation number (1-based). 1st -> 7d, 2nd -> 15d, 3rd+ -> 30d.
func RestrictionDuration(violationNumber int) time.Duration {
	switch {
	case violationNumber <= 1:
		return 7 * 24 * time.Hour
	case violationNumber == 2:
		return 15 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

// ErrRepositoryNotConfigured is returned when the service is used without a
// wired repository (defensive; production wiring always provides one).
var ErrRepositoryNotConfigured = errors.New("commercegov: repository not configured")

// Repository persists violations and restrictions. Implemented by the
// infrastructure repository; defined here so application callers depend on an
// interface rather than the concrete package.
type Repository interface {
	InsertViolation(ctx context.Context, tx db.Tx, v *Violation) error
	GetRestrictionForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*Restriction, error)
	UpsertRestriction(ctx context.Context, tx db.Tx, r *Restriction) error
}

// Violation is a commerce violation history record.
type Violation struct {
	ID            uuid.UUID     `json:"id"`
	UserID        uuid.UUID     `json:"user_id"`
	ViolationType ViolationType `json:"violation_type"`
	SourceType    SourceType    `json:"source_type"`
	SourceID      uuid.UUID     `json:"source_id"`
	Reason        string        `json:"reason,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

// Restriction is the current active commerce restriction for a user.
type Restriction struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	ViolationCount   int       `json:"violation_count"`
	RestrictedUntil  time.Time `json:"restricted_until"`
	LastViolationID  uuid.UUID `json:"last_violation_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RecordInput is the input for recording a violation and applying/extending
// the offending user's restriction atomically.
type RecordInput struct {
	UserID        uuid.UUID
	ViolationType ViolationType
	SourceType    SourceType
	SourceID      uuid.UUID
	Reason        string
	Metadata      map[string]any
}

// RecordViolationAndRestrict appends an immutable violation and upserts the
// user's restriction with EXTEND stacking in a single transaction. Returns
// the created violation and the resulting restriction. The restriction
// deadline is computed from the NEW violation count.
func RecordViolationAndRestrict(
	ctx context.Context,
	tx db.Tx,
	repo Repository,
	input RecordInput,
) (*Violation, *Restriction, error) {
	if repo == nil {
		return nil, nil, ErrRepositoryNotConfigured
	}
	if !input.ViolationType.IsValid() {
		return nil, nil, fmt.Errorf("commercegov: invalid violation type %q", input.ViolationType)
	}

	violation := &Violation{
		ID:            uuid.New(),
		UserID:        input.UserID,
		ViolationType: input.ViolationType,
		SourceType:    input.SourceType,
		SourceID:      input.SourceID,
		Reason:        input.Reason,
		Metadata:      input.Metadata,
	}
	if err := repo.InsertViolation(ctx, tx, violation); err != nil {
		return nil, nil, fmt.Errorf("commercegov: insert violation: %w", err)
	}

	existing, err := repo.GetRestrictionForUpdate(ctx, tx, input.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("commercegov: load restriction: %w", err)
	}

	now := time.Now()
	restriction := &Restriction{
		ID:              uuid.New(),
		UserID:          input.UserID,
		ViolationCount:  1,
		RestrictedUntil: now.Add(RestrictionDuration(1)),
		LastViolationID: violation.ID,
	}
	if existing != nil {
		restriction.ID = existing.ID
		restriction.CreatedAt = existing.CreatedAt
		restriction.ViolationCount = existing.ViolationCount + 1

		duration := RestrictionDuration(restriction.ViolationCount)
		if existing.RestrictedUntil.After(now) {
			// Active restriction: EXTEND from the current expiry.
			restriction.RestrictedUntil = existing.RestrictedUntil.Add(duration)
		} else {
			restriction.RestrictedUntil = now.Add(duration)
		}
	}

	if err := repo.UpsertRestriction(ctx, tx, restriction); err != nil {
		return nil, nil, fmt.Errorf("commercegov: upsert restriction: %w", err)
	}

	return violation, restriction, nil
}

// IsUserRestricted returns whether the user has an active commerce restriction
// (restricted_until > now) and, when restricted, the restriction deadline.
func IsUserRestricted(
	ctx context.Context,
	tx db.Tx,
	repo Repository,
	userID uuid.UUID,
) (bool, *time.Time, error) {
	if repo == nil {
		return false, nil, ErrRepositoryNotConfigured
	}
	existing, err := repo.GetRestrictionForUpdate(ctx, tx, userID)
	if err != nil {
		return false, nil, err
	}
	if existing == nil {
		return false, nil, nil
	}
	now := time.Now()
	if existing.RestrictedUntil.After(now) {
		until := existing.RestrictedUntil
		return true, &until, nil
	}
	return false, nil, nil
}

// MarshalMetadata serializes metadata for JSON persistence.
func MarshalMetadata(m map[string]any) []byte {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}
