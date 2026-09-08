// Package entity holds the canonical Promotion Contract rolling target queue.
package entity

import (
	"time"

	"github.com/google/uuid"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
)

// MaxTargetsPerContract is the canonical rolling queue capacity (§12).
const MaxTargetsPerContract = 10

// ContractTarget is one entry in a promotion contract's rolling target queue.
// The queue is position-ordered (0..9); delivery resolves the first operable
// entry. Promotion never duplicates Commerce lifecycle — eligibility is
// deferred to For Sale / Auction authority at delivery/qualification time.
type ContractTarget struct {
	ID         uuid.UUID
	ContractID uuid.UUID
	TargetType promoentity.TargetType
	TargetID   uuid.UUID
	Position   int
	AddedAt    time.Time
}

// IsValid reports whether t is a well-formed queue entry.
func (t *ContractTarget) IsValid() bool {
	if t.ContractID == uuid.Nil || t.TargetID == uuid.Nil {
		return false
	}
	if !t.TargetType.IsValid() {
		return false
	}
	if t.Position < 0 || t.Position >= MaxTargetsPerContract {
		return false
	}
	return true
}
