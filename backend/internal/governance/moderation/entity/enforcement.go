// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Enforcement entity — mutable execution lifecycle
//
// SLICE 5: This entity is the canonical Enforcement authority.
// Enforcement is a mutable execution record for a Decision.
// Enforcement ≠ Decision. Decision is immutable; Enforcement is mutable.
//
// Canonical lifecycle:
//   pending → processing → succeeded
//                       → failed → retry → processing → ...
//
// Canonical boundary:
//   Decision (immutable governance record)
//     ↓
//   Enforcement (mutable execution lifecycle)
//     ↓
//   outbox event (delivery)
//     ↓
//   target-domain executor
//     ↓
//   Enforcement result write-back

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EnforcementStatus represents the canonical Enforcement lifecycle.
// LOCKED: Only four values from the enforcement_status_enum.
type EnforcementStatus string

const (
	EnforcementStatusPending    EnforcementStatus = "pending"
	EnforcementStatusProcessing EnforcementStatus = "processing"
	EnforcementStatusSucceeded  EnforcementStatus = "succeeded"
	EnforcementStatusFailed     EnforcementStatus = "failed"
)

// IsValid returns true if the status is in the canonical set.
func (s EnforcementStatus) IsValid() bool {
	return s == EnforcementStatusPending ||
		s == EnforcementStatusProcessing ||
		s == EnforcementStatusSucceeded ||
		s == EnforcementStatusFailed
}

// String returns the string representation.
func (s EnforcementStatus) String() string {
	return string(s)
}

// ModerationTargetType represents the canonical moderation target types.
// LOCKED: Matches moderation_target_type_enum (content, comment, for_sale, auction, user).
type ModerationTargetType string

const (
	ModerationTargetTypeContent ModerationTargetType = "content"
	ModerationTargetTypeComment ModerationTargetType = "comment"
	ModerationTargetTypeForSale ModerationTargetType = "for_sale"
	ModerationTargetTypeAuction ModerationTargetType = "auction"
	ModerationTargetTypeUser    ModerationTargetType = "user"
)

// IsValid returns true if the target type is in the canonical set.
func (t ModerationTargetType) IsValid() bool {
	return t == ModerationTargetTypeContent ||
		t == ModerationTargetTypeComment ||
		t == ModerationTargetTypeForSale ||
		t == ModerationTargetTypeAuction ||
		t == ModerationTargetTypeUser
}

// String returns the string representation.
func (t ModerationTargetType) String() string {
	return string(t)
}

// Enforcement represents a mutable execution record for a Decision.
//
// Business rules:
//   - Enforcement belongs to exactly one Decision (decision_id NOT NULL, FK)
//   - One Decision may produce multiple Enforcements (different targets)
//   - UNIQUE(decision_id, target_type, target_id) — one enforcement per target per decision
//   - Enforcement lifecycle: pending → processing → succeeded/failed
//   - Failed Enforcement can be retried (next_attempt_at, attempt_count)
//   - Enforcement status is the authority for execution state
//   - Decision outcome does NOT change based on Enforcement result
//
// Schema: enforcements table (migration 000055)
// Constraints: enforcements_decision_target_unique, enforcements_attempt_count_nonneg
type Enforcement struct {
	ID             uuid.UUID
	DecisionID     uuid.UUID
	TargetType     ModerationTargetType
	TargetID       uuid.UUID
	Status         EnforcementStatus
	AttemptCount   int
	RequestedAt    time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	LastError      *string
	NextAttemptAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewEnforcement creates a new pending Enforcement.
func NewEnforcement(
	decisionID uuid.UUID,
	targetType ModerationTargetType,
	targetID uuid.UUID,
) (*Enforcement, error) {
	if !targetType.IsValid() {
		return nil, &ErrInvalidEnforcementTargetType{TargetType: targetType}
	}

	now := time.Now().UTC()
	return &Enforcement{
		ID:            uuid.New(),
		DecisionID:    decisionID,
		TargetType:    targetType,
		TargetID:      targetID,
		Status:        EnforcementStatusPending,
		AttemptCount:  0,
		RequestedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// IsTerminal returns true if the Enforcement is in a terminal state.
func (e *Enforcement) IsTerminal() bool {
	return e.Status == EnforcementStatusSucceeded || e.Status == EnforcementStatusFailed
}

// CanProcess returns true if the Enforcement can transition to processing.
func (e *Enforcement) CanProcess() bool {
	return e.Status == EnforcementStatusPending || e.Status == EnforcementStatusFailed
}

// ErrInvalidEnforcementTargetType is returned when an invalid target type is provided.
type ErrInvalidEnforcementTargetType struct {
	TargetType ModerationTargetType
}

func (e *ErrInvalidEnforcementTargetType) Error() string {
	return fmt.Sprintf("invalid enforcement target type: %s (valid: content, comment, for_sale, auction, user)", e.TargetType)
}
