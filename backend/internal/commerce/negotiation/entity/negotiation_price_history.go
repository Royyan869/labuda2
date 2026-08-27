// DOMAIN: COMMERCE
// NOTE: Audit trail for negotiation price changes

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NegotiationPriceHistory represents a price change in a negotiation session.
// This provides an audit trail for all price updates.
type NegotiationPriceHistory struct {
	ID              uuid.UUID
	SessionID       uuid.UUID
	ProposalSequence int64
	OldPrice        *int64 // NULL for initial proposal
	NewPrice        int64  // Always set
	ChangedByUserID uuid.UUID
	ChangeReason    string
	CreatedAt       time.Time
}

// NewNegotiationPriceHistory creates a new price history entry.
func NewNegotiationPriceHistory(
	sessionID uuid.UUID,
	proposalSequence int64,
	oldPrice *int64,
	newPrice int64,
	changedByUserID uuid.UUID,
	changeReason string,
) *NegotiationPriceHistory {
	return &NegotiationPriceHistory{
		ID:              uuid.New(),
		SessionID:       sessionID,
		ProposalSequence: proposalSequence,
		OldPrice:        oldPrice,
		NewPrice:        newPrice,
		ChangedByUserID: changedByUserID,
		ChangeReason:    changeReason,
		CreatedAt:       time.Now(),
	}
}

// GetPriceChange returns the price difference (can be negative).
func (h *NegotiationPriceHistory) GetPriceChange() int64 {
	if h.OldPrice == nil {
		return h.NewPrice // Initial proposal
	}
	return h.NewPrice - *h.OldPrice
}

// IsPriceIncrease returns true if the new price is higher than the old price.
func (h *NegotiationPriceHistory) IsPriceIncrease() bool {
	if h.OldPrice == nil {
		return false // Initial proposal is not an increase
	}
	return h.NewPrice > *h.OldPrice
}

// IsPriceDecrease returns true if the new price is lower than the old price.
func (h *NegotiationPriceHistory) IsPriceDecrease() bool {
	if h.OldPrice == nil {
		return false // Initial proposal is not a decrease
	}
	return h.NewPrice < *h.OldPrice
}

// String returns a human-readable description of the price change.
func (h *NegotiationPriceHistory) String() string {
	if h.OldPrice == nil {
		return fmt.Sprintf("Initial proposal: %d", h.NewPrice)
	}

	change := h.GetPriceChange()
	direction := "increased"
	if change < 0 {
		direction = "decreased"
		change = -change
	}

	return fmt.Sprintf("Price %s: %d → %d (change: %d)",
		direction, *h.OldPrice, h.NewPrice, change)
}


