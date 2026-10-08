package application

import (
	"context"

	"github.com/google/uuid"
)

// ShippingQuoteProjection is the viewer-scoped availability/actionability of a
// shipping quote as projected into a conversation message.
//
// The values are produced by the Shipping commerce authority (quote lifecycle +
// buyer eligibility). Conversation surfaces only RENDER them; they must never
// compute "is this the current/actionable quote" themselves, and must never
// derive buyer/seller identity.
type ShippingQuoteProjection struct {
	IsCurrent        bool
	ViewerActionable bool
}

// ShippingQuoteProjectionResolver resolves viewer-scoped shipping quote
// projections for a BATCH of quotes (keyed by quote id) in one call.
//
// This is the conversation-side port. It is defined here (Chat owns the shape
// it consumes) and implemented in wiring by the Shipping commerce authority, so
// Chat does not import Shipping internals and never becomes Commerce authority.
type ShippingQuoteProjectionResolver interface {
	ResolveShippingQuoteProjections(
		ctx context.Context,
		viewerID uuid.UUID,
		quoteIDs []uuid.UUID,
	) (map[uuid.UUID]ShippingQuoteProjection, error)
}
