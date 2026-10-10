package serverboot

import (
	"context"

	"github.com/google/uuid"
	shippingQuoteApp "github.com/hishumi/backend/internal/commerce/shipping/quote/application"
	chatApp "github.com/hishumi/backend/internal/interaction/chat/application"
)

// shippingQuoteProjectionResolverAdapter implements
// chatApp.ShippingQuoteProjectionResolver by delegating to the Shipping
// commerce authority (Service.ProjectForViewer). Chat consumes only the neutral
// projection struct; it never touches quote lifecycle or buyer eligibility.
type shippingQuoteProjectionResolverAdapter struct {
	svc *shippingQuoteApp.Service
}

// newShippingQuoteProjectionResolver wires the Shipping commerce authority into
// the conversation read path.
func newShippingQuoteProjectionResolver(svc *shippingQuoteApp.Service) chatApp.ShippingQuoteProjectionResolver {
	return &shippingQuoteProjectionResolverAdapter{svc: svc}
}

func (a *shippingQuoteProjectionResolverAdapter) ResolveShippingQuoteProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	quoteIDs []uuid.UUID,
) (map[uuid.UUID]chatApp.ShippingQuoteProjection, error) {
	resolved, err := a.svc.ProjectForViewer(ctx, viewerID, quoteIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]chatApp.ShippingQuoteProjection, len(resolved))
	for id, p := range resolved {
		out[id] = chatApp.ShippingQuoteProjection{
			IsCurrent:        p.IsCurrent,
			ViewerActionable: p.ViewerActionable,
		}
	}
	return out, nil
}
