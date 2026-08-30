package response_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/commerce/response"
	"github.com/labuda/backend/pkg/db"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type stubForSale struct {
	status forSaleEntity.ForSaleStatus
}

type stubAuction struct {
	status auctionEntity.Status
}

type stubFPSGetter struct {
	fps map[uuid.UUID]stubForSale
}

func (g *stubFPSGetter) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*forSaleEntity.ForSale, error) {
	f, ok := g.fps[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &forSaleEntity.ForSale{
		Status: f.status,
	}, nil
}

type stubAuctionGetter struct {
	auctions map[uuid.UUID]stubAuction
}

func (g *stubAuctionGetter) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*auctionEntity.Auction, error) {
	a, ok := g.auctions[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &auctionEntity.Auction{
		Status: a.status,
	}, nil
}

// ---------------------------------------------------------------------------
// For Sale tests
// ---------------------------------------------------------------------------

func TestValidateReference_ForSale_Active_Passes(t *testing.T) {
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusActive}}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.NoError(t, err)
}

func TestValidateReference_ForSale_Draft_Fails(t *testing.T) {
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusDraft}}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_ForSale_Sold_Fails(t *testing.T) {
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusSold}}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_ForSale_Withdrawn_Fails(t *testing.T) {
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusWithdrawn}}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_ForSale_NotFound_Fails(t *testing.T) {
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, uuid.New())
	require.ErrorIs(t, err, response.ErrResourceNotFound)
}

// ---------------------------------------------------------------------------
// Auction tests
// ---------------------------------------------------------------------------

func TestValidateReference_Auction_Scheduled_Passes(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusScheduled}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.NoError(t, err)
}

func TestValidateReference_Auction_Active_Passes(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusActive}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.NoError(t, err)
}

func TestValidateReference_Auction_Draft_Fails(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusDraft}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_Auction_Ended_Fails(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusEnded}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_Auction_Cancelled_Fails(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusCancelled}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_Auction_WaitingSettlement_Fails(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusWaitingSettlement}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_Auction_ExpiredBNR_Fails(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusExpiredBNR}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable)
}

func TestValidateReference_Auction_NotFound_Fails(t *testing.T) {
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, uuid.New())
	require.ErrorIs(t, err, response.ErrResourceNotFound)
}

// ---------------------------------------------------------------------------
// Unsupported type
// ---------------------------------------------------------------------------

func TestValidateReference_UnsupportedType_Fails(t *testing.T) {
	v := response.NewValidator(&stubFPSGetter{}, &stubAuctionGetter{})
	err := v.ValidateReference(context.Background(), nil, response.ResourceType("profile"), uuid.New())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported commerce resource type")
}

// ---------------------------------------------------------------------------
// Cross-caller: any user can reference any displayable resource
// ---------------------------------------------------------------------------

func TestValidateReference_ForSale_AnyUser_CanReference(t *testing.T) {
	// The validator does not care WHO is referencing — any user can reference
	// any displayable resource. Ownership is not checked.
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusActive}}},
		&stubAuctionGetter{},
	)

	// Different user referencing — should pass
	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.NoError(t, err)
}

func TestValidateReference_Auction_AnyUser_CanReference(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusActive}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// COUNTERFACTUAL PROOF: Explicit business-truth locks
// ---------------------------------------------------------------------------
//
// These tests explicitly lock the canonical reference/display business truth:
// - ANY user may reference ANY displayable commerce resource
// - Ownership is NOT checked
// - Seller capability is NOT checked
// - Only existence + displayability matter

// TestCounterfactual_ActorBReferencesActorAsActiveForSale locks:
// Actor B references Actor A's active ForSale → ALLOWED.
func TestCounterfactual_ActorBReferencesActorAsActiveForSale(t *testing.T) {
	actorA_FPS := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{actorA_FPS: {status: forSaleEntity.ForSaleStatusActive}}},
		&stubAuctionGetter{},
	)

	// Actor B (no identity passed to validator) references Actor A's resource.
	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, actorA_FPS)
	require.NoError(t, err, "Actor B MUST be allowed to reference Actor A's active ForSale")
}

// TestCounterfactual_ActorBReferencesActorAsScheduledAuction locks:
// Actor B references Actor A's scheduled Auction → ALLOWED.
func TestCounterfactual_ActorBReferencesActorAsScheduledAuction(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusScheduled}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.NoError(t, err, "Actor B MUST be allowed to reference Actor A's scheduled Auction")
}

// TestCounterfactual_ActorBReferencesActorAsActiveAuction locks:
// Actor B references Actor A's active Auction → ALLOWED.
func TestCounterfactual_ActorBReferencesActorAsActiveAuction(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusActive}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.NoError(t, err, "Actor B MUST be allowed to reference Actor A's active Auction")
}

// TestCounterfactual_NoSellerCapability_CanReference locks:
// An actor without any seller capability can reference a displayable resource.
// The validator does not receive or check caller identity at all.
func TestCounterfactual_NoSellerCapability_CanReference(t *testing.T) {
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusActive}}},
		&stubAuctionGetter{},
	)

	// The validator API takes no callerID — it cannot check seller capability.
	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.NoError(t, err, "non-seller MUST be allowed to reference a displayable resource")
}

// TestCounterfactual_NonExistentForSale_Rejected locks:
// Referencing a non-existent ForSale → REJECTED with ErrResourceNotFound.
func TestCounterfactual_NonExistentForSale_Rejected(t *testing.T) {
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, uuid.New())
	require.ErrorIs(t, err, response.ErrResourceNotFound, "non-existent ForSale MUST be rejected")
}

// TestCounterfactual_NonExistentAuction_Rejected locks:
// Referencing a non-existent Auction → REJECTED with ErrResourceNotFound.
func TestCounterfactual_NonExistentAuction_Rejected(t *testing.T) {
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, uuid.New())
	require.ErrorIs(t, err, response.ErrResourceNotFound, "non-existent Auction MUST be rejected")
}

// TestCounterfactual_NonDisplayableForSale_Rejected locks:
// Referencing a draft ForSale → REJECTED with ErrResourceNotDisplayable.
func TestCounterfactual_NonDisplayableForSale_Rejected(t *testing.T) {
	fpsID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{fps: map[uuid.UUID]stubForSale{fpsID: {status: forSaleEntity.ForSaleStatusDraft}}},
		&stubAuctionGetter{},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeForSale, fpsID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable, "draft ForSale MUST be rejected (not displayable)")
}

// TestCounterfactual_NonDisplayableAuction_Rejected locks:
// Referencing a draft Auction → REJECTED with ErrResourceNotDisplayable.
func TestCounterfactual_NonDisplayableAuction_Rejected(t *testing.T) {
	auctionID := uuid.New()
	v := response.NewValidator(
		&stubFPSGetter{},
		&stubAuctionGetter{auctions: map[uuid.UUID]stubAuction{auctionID: {status: auctionEntity.StatusDraft}}},
	)

	err := v.ValidateReference(context.Background(), nil, response.ResourceTypeAuction, auctionID)
	require.ErrorIs(t, err, response.ErrResourceNotDisplayable, "draft Auction MUST be rejected (not displayable)")
}
