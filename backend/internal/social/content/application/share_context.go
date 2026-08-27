package application

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
)

// ShareAttributionContext is the canonical backend attribution context.
// It is backend-only and does not alter the public wire shape.
type ShareAttributionContext struct {
	ActorID          uuid.UUID
	TargetOwnerID    *uuid.UUID
	OriginalAuthorID *uuid.UUID
	DisplayName      string
	Username         string
	SellerFarmName   string
	LifecycleState   string
	VisibilityState  string
}

// ShareSnapshotContext is the canonical backend snapshot context.
// It remains available for projection code that needs a zero-value snapshot.
type ShareSnapshotContext struct {
	TargetType entity.ShareTargetType
	TargetID   string

	Title    string
	Subtitle string
	Image    string

	Availability bool
	Lifecycle    string

	OwnerID             *uuid.UUID
	OwnerDisplayName    string
	OwnerUsername       string
	OwnerSellerFarmName string
	OwnerLifecycleState string

	PriceSummary string
	BidSummary   string

	IsSold    bool
	IsClosed  bool
	IsDeleted bool
}

// BuildContentAttributionContext derives the backend-only attribution context
// from the persisted content row.
func BuildContentAttributionContext(content *entity.Content) ShareAttributionContext {
	if content == nil {
		return ShareAttributionContext{}
	}

	ctx := ShareAttributionContext{
		ActorID:         content.AuthorID,
		LifecycleState:  content.Status.PublicLifecycle(),
		VisibilityState: "public",
	}

	if content.IsHidden {
		ctx.VisibilityState = "private"
	}

	if content.IsRepost() && content.OriginalAuthorID != nil {
		original := *content.OriginalAuthorID
		ctx.OriginalAuthorID = &original
		ctx.TargetOwnerID = &original
	}

	return ctx
}
