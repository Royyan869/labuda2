package application

import (
	"context"

	"github.com/google/uuid"
	commerceshared "github.com/hishumi/backend/internal/commerce/shared"
	chatEntity "github.com/hishumi/backend/internal/interaction/chat/entity"
)

// ResourceProjectionResolver resolves chat message occurrences into viewer-
// aware canonical projection envelopes.
//
// SCOPE #3 CONVERGENCE: the chat module used to own a private projection
// envelope (ResourceProjection + payload zoo + validation + marshalling).
// That duplicate authority is deleted; every surface now shares
// commerceshared.ResourceProjection. Only this port interface remains in the
// chat application package, because it speaks the chat occurrence graph.
type ResourceProjectionResolver interface {
	ResolveResourceProjections(
		ctx context.Context,
		viewerID uuid.UUID,
		occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	) (map[uuid.UUID]*commerceshared.ResourceProjection, error)
}

// The aliases below re-export the CANONICAL shared types under their former
// chat-module names. They are Go type aliases — literally the same type, one
// validation/marshal implementation, zero duplication — so chat call sites
// keep compiling while the wire stays canonical. The chat payload shapes that
// DISAGREED with canonical (fps flat seller + image_url, auction pointer
// fields) have NO alias on purpose: the compiler forces every construction
// site onto the canonical shape.
type (
	ResourceProjection           = commerceshared.ResourceProjection
	ProjectionState              = commerceshared.ProjectionState
	ProjectionViewerCapabilities = commerceshared.ProjectionViewerCapabilities
	NestedResourceIndicator      = commerceshared.NestedResourceIndicator
	ProfileLivePayload           = commerceshared.ProfileLivePayload
	ContentLivePayload           = commerceshared.ContentLivePayload
	ForSaleLivePrice             = commerceshared.LivePrice
)

const (
	ProjectionStateLive      = commerceshared.ProjectionStateLive
	ProjectionStateTombstone = commerceshared.ProjectionStateTombstone
)
