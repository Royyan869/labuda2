package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	"github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// isActiveAccountChecker allows all callers for test isolation.
type isActiveAccountChecker struct{}

func (isActiveAccountChecker) EnsureActive(context.Context, uuid.UUID) error  { return nil }
func (isActiveAccountChecker) GetStatus(context.Context, uuid.UUID) (string, error) { return "active", nil }
func (isActiveAccountChecker) IsBanned(context.Context, uuid.UUID) (bool, error)     { return false, nil }

// stubCommerceValidator records calls and returns a configurable error.
type stubCommerceValidator struct {
	validateCalls []stubValidateCall
	returnErr     error
}

type stubValidateCall struct {
	resourceType commerceResponse.ResourceType
	resourceID   uuid.UUID
}

func (v *stubCommerceValidator) ValidateReference(_ context.Context, _ db.Tx, resourceType commerceResponse.ResourceType, resourceID uuid.UUID) error {
	v.validateCalls = append(v.validateCalls, stubValidateCall{
		resourceType: resourceType,
		resourceID:   resourceID,
	})
	return v.returnErr
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// countingContentRepo embeds the real interface and adds call counters.
// Methods not overridden panic if called — this test only exercises Internal Share.
type countingContentRepo struct {
	contentrepo.ContentRepository
	createCalls int
	occurrences map[uuid.UUID]*entity.ContentResourceOccurrence
}

// Ensure countingContentRepo is used by the test (suppress unused import warning).
var _ contentrepo.ContentRepository = (*countingContentRepo)(nil)

func newCountingContentRepo() *countingContentRepo {
	return &countingContentRepo{}
}

func (r *countingContentRepo) Create(_ context.Context, _ interface{}, _ *entity.Content) error {
	r.createCalls++
	return nil
}

func (r *countingContentRepo) CreateResourceOccurrence(_ context.Context, _ interface{}, occ *entity.ContentResourceOccurrence) error {
	if r.occurrences == nil {
		r.occurrences = map[uuid.UUID]*entity.ContentResourceOccurrence{}
	}
	r.occurrences[occ.ContentID] = occ
	return nil
}

func (r *countingContentRepo) InsertTags(_ context.Context, _ interface{}, _ uuid.UUID, _ []string) error {
	return nil
}

// newInternalShareTestService creates a ContentService wired with the given
// validator and a no-op content repo. The internalShareAuthority is set to
// the service itself (same as production wiring).
func newInternalShareTestService(validator *stubCommerceValidator) (*ContentService, *countingContentRepo) {
	repo := newCountingContentRepo()
	svc := &ContentService{
		contentRepo:          repo,
		accountStatusChecker: isActiveAccountChecker{},
		commerceRefValidator: validator,
	}
	svc.internalShareAuthority = svc
	return svc, repo
}

// ---------------------------------------------------------------------------
// TASK C — Counterfactual Proof Tests
//
// These tests prove that the Internal Share path routes ForSale/Auction
// through the canonical commerceResponse.Validator before creating an
// occurrence. The business truth is:
//
//   - Actor B may reference Actor A's displayable ForSale/Auction
//   - Non-existent resources are rejected
//   - Non-displayable resources are rejected
//   - The canonical validator is the sole authority
// ---------------------------------------------------------------------------

// TestInternalShare_ForSale_Active_Allowed proves:
// Actor B references Actor A's active ForSale via Internal Share → ALLOWED.
func TestInternalShare_ForSale_Active_Allowed(t *testing.T) {
	validator := &stubCommerceValidator{returnErr: nil}
	svc, repo := newInternalShareTestService(validator)

	forSaleID := uuid.New()
	actorB := uuid.New()

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    actorB,
		TargetType: entity.ShareTargetTypeForSale,
		TargetID:   forSaleID.String(),
		Caption:    "Check out this listing!",
	})

	if err != nil {
		t.Fatalf("Internal Share to active ForSale should succeed: %v", err)
	}
	if content == nil {
		t.Fatal("expected content to be created")
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", repo.createCalls)
	}
	if len(validator.validateCalls) != 1 {
		t.Fatalf("expected 1 validator call, got %d", len(validator.validateCalls))
	}
	// Verify validator was called with the correct resource type and ID
	call := validator.validateCalls[0]
	if call.resourceType != commerceResponse.ResourceTypeForSale {
		t.Fatalf("validator called with wrong resource type: %s, want %s", call.resourceType, commerceResponse.ResourceTypeForSale)
	}
	if call.resourceID != forSaleID {
		t.Fatalf("validator called with wrong resource ID: %s, want %s", call.resourceID, forSaleID)
	}
}

// TestInternalShare_Auction_Scheduled_Allowed proves:
// Actor B references Actor A's scheduled Auction via Internal Share → ALLOWED.
func TestInternalShare_Auction_Scheduled_Allowed(t *testing.T) {
	validator := &stubCommerceValidator{returnErr: nil}
	svc, repo := newInternalShareTestService(validator)

	auctionID := uuid.New()
	actorB := uuid.New()

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    actorB,
		TargetType: entity.ShareTargetTypeAuction,
		TargetID:   auctionID.String(),
		Caption:    "Bid on this!",
	})

	if err != nil {
		t.Fatalf("Internal Share to scheduled Auction should succeed: %v", err)
	}
	if content == nil {
		t.Fatal("expected content to be created")
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", repo.createCalls)
	}
	if len(validator.validateCalls) != 1 {
		t.Fatalf("expected 1 validator call, got %d", len(validator.validateCalls))
	}
	call := validator.validateCalls[0]
	if call.resourceType != commerceResponse.ResourceTypeAuction {
		t.Fatalf("validator called with wrong resource type: %s, want %s", call.resourceType, commerceResponse.ResourceTypeAuction)
	}
	if call.resourceID != auctionID {
		t.Fatalf("validator called with wrong resource ID: %s, want %s", call.resourceID, auctionID)
	}
}

// TestInternalShare_ForSale_NotFound_Rejected proves:
// Internal Share to non-existent ForSale → REJECTED.
func TestInternalShare_ForSale_NotFound_Rejected(t *testing.T) {
	validator := &stubCommerceValidator{
		returnErr: fmt.Errorf("for_sale not found: %s", uuid.New()),
	}
	svc, repo := newInternalShareTestService(validator)

	forSaleID := uuid.New()

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    uuid.New(),
		TargetType: entity.ShareTargetTypeForSale,
		TargetID:   forSaleID.String(),
		Caption:    "Share listing",
	})

	if err == nil {
		t.Fatal("Internal Share to non-existent ForSale should be rejected")
	}
	if content != nil {
		t.Fatal("expected no content created on rejection")
	}
	if repo.createCalls != 0 {
		t.Fatalf("expected 0 create calls on rejection, got %d", repo.createCalls)
	}
	if len(validator.validateCalls) != 1 {
		t.Fatalf("expected validator to be called even for non-existent resource, got %d calls", len(validator.validateCalls))
	}
}

// TestInternalShare_Auction_NotFound_Rejected proves:
// Internal Share to non-existent Auction → REJECTED.
func TestInternalShare_Auction_NotFound_Rejected(t *testing.T) {
	validator := &stubCommerceValidator{
		returnErr: fmt.Errorf("auction not found: %s", uuid.New()),
	}
	svc, repo := newInternalShareTestService(validator)

	auctionID := uuid.New()

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    uuid.New(),
		TargetType: entity.ShareTargetTypeAuction,
		TargetID:   auctionID.String(),
		Caption:    "Share auction",
	})

	if err == nil {
		t.Fatal("Internal Share to non-existent Auction should be rejected")
	}
	if content != nil {
		t.Fatal("expected no content created on rejection")
	}
	if repo.createCalls != 0 {
		t.Fatalf("expected 0 create calls on rejection, got %d", repo.createCalls)
	}
	if len(validator.validateCalls) != 1 {
		t.Fatalf("expected validator to be called even for non-existent resource, got %d calls", len(validator.validateCalls))
	}
}

// TestInternalShare_ForSale_NonDisplayable_Rejected proves:
// Internal Share to non-displayable (draft/withdrawn/sold) ForSale → REJECTED.
func TestInternalShare_ForSale_NonDisplayable_Rejected(t *testing.T) {
	validator := &stubCommerceValidator{
		returnErr: fmt.Errorf("cannot share for_sale in status \"draft\""),
	}
	svc, repo := newInternalShareTestService(validator)

	forSaleID := uuid.New()

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    uuid.New(),
		TargetType: entity.ShareTargetTypeForSale,
		TargetID:   forSaleID.String(),
		Caption:    "Share draft listing",
	})

	if err == nil {
		t.Fatal("Internal Share to non-displayable ForSale should be rejected")
	}
	if content != nil {
		t.Fatal("expected no content created on rejection")
	}
	if repo.createCalls != 0 {
		t.Fatalf("expected 0 create calls on rejection, got %d", repo.createCalls)
	}
}

// TestInternalShare_Auction_NonDisplayable_Rejected proves:
// Internal Share to non-displayable (draft/ended/cancelled) Auction → REJECTED.
func TestInternalShare_Auction_NonDisplayable_Rejected(t *testing.T) {
	validator := &stubCommerceValidator{
		returnErr: fmt.Errorf("cannot share auction in status \"draft\""),
	}
	svc, repo := newInternalShareTestService(validator)

	auctionID := uuid.New()

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    uuid.New(),
		TargetType: entity.ShareTargetTypeAuction,
		TargetID:   auctionID.String(),
		Caption:    "Share draft auction",
	})

	if err == nil {
		t.Fatal("Internal Share to non-displayable Auction should be rejected")
	}
	if content != nil {
		t.Fatal("expected no content created on rejection")
	}
	if repo.createCalls != 0 {
		t.Fatalf("expected 0 create calls on rejection, got %d", repo.createCalls)
	}
}

// TestInternalShare_ForSale_ForeignResource_Allowed proves:
// Actor B references Actor A's ForSale → ALLOWED (no ownership check).
func TestInternalShare_ForSale_ForeignResource_Allowed(t *testing.T) {
	validator := &stubCommerceValidator{returnErr: nil}
	svc, repo := newInternalShareTestService(validator)

	forSaleID := uuid.New()
	actorB := uuid.New() // NOT the owner

	content, err := svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    actorB,
		TargetType: entity.ShareTargetTypeForSale,
		TargetID:   forSaleID.String(),
		Caption:    "Sharing someone else's listing",
	})

	if err != nil {
		t.Fatalf("Internal Share to foreign ForSale should succeed (no ownership check): %v", err)
	}
	if content == nil {
		t.Fatal("expected content to be created")
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", repo.createCalls)
	}
	// Verify the validator was called — it should NOT check ownership
	if len(validator.validateCalls) != 1 {
		t.Fatalf("expected validator to be called exactly once, got %d", len(validator.validateCalls))
	}
}

// TestInternalShare_NoOwnershipCheck proves:
// The validator does NOT receive actor identity. This test ensures that the
// Internal Share path passes no caller ID to the commerce validator, making
// ownership-based rejection architecturally impossible.
func TestInternalShare_NoOwnershipCheck(t *testing.T) {
	validator := &stubCommerceValidator{returnErr: nil}
	svc, _ := newInternalShareTestService(validator)

	// Actor who is NOT the owner shares the ForSale
	svc.CreateInternalShare(context.Background(), nil, &CreateInternalShareRequest{
		ActorID:    uuid.New(),
		TargetType: entity.ShareTargetTypeForSale,
		TargetID:   uuid.New().String(),
		Caption:    "Not my listing but sharing it",
	})

	// The validator was called with only resourceType + resourceID — no actor identity.
	// This proves ownership check is architecturally impossible through this path.
	if len(validator.validateCalls) != 1 {
		t.Fatalf("expected 1 validator call, got %d", len(validator.validateCalls))
	}
}
