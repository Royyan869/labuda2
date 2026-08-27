package entity

import (
	"time"

	"github.com/google/uuid"
)

// ShippingProof represents a shipping proof uploaded by a seller.
//
// HONEST PURPOSE: This is DISPUTE EVIDENCE, not a fulfillment requirement.
// The order state machine transitions from paid→shipped based on the seller's
// MarkShipped() call, not based on ShippingProof existence.
//
// ShippingProof is only used if a dispute arises - it provides evidence that
// the seller actually shipped the item. It does NOT block or enable the shipped
// transition, and it does NOT affect the auto-complete timer.
//
// This design supports the seller-managed shipping model where sellers are
// trusted to mark orders as shipped, with disputes as the escalation path
// if buyers don't receive their items.
type ShippingProof struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	SellerID  uuid.UUID
	MediaURL  string
	CreatedAt time.Time
}

// NewShippingProof creates a new shipping proof.
func NewShippingProof(orderID, sellerID uuid.UUID, mediaURL string) *ShippingProof {
	return &ShippingProof{
		ID:        uuid.New(),
		OrderID:   orderID,
		SellerID:  sellerID,
		MediaURL:  mediaURL,
		CreatedAt: time.Now(),
	}
}

// GenerateOrderNumber generates a human-readable order number.
// Format: ORD-YYYYMMDD-XXXXXXXX (8 character random hex)
func GenerateOrderNumber() string {
	// Format: ORD-20260309-AB12CD34
	return "ORD-" + time.Now().Format("20060102") + "-" + uuid.New().String()[:8]
}


