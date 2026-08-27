package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

// =============================================================================
// EXTERNAL PRODUCT REVIEW NOTIFICATION CONTRACT TESTS
//
// These tests pin the event-emission contract for admin review decisions.
// They verify:
//   - externalProductReviewEventType() maps actions to canonical event names.
//   - emitExternalProductReviewEventTx() calls InsertEvent with correct args.
//   - Nil outboxEmitter is safe (no panic, no error).
//   - Payload carries owner_user_id, external_product_id, title, review_status.
//   - Reason is included when present; omitted when absent.
// =============================================================================

// ---------------------------------------------------------------------------
// EVENT TYPE MAPPING — pure function, no mocks needed
// ---------------------------------------------------------------------------

// TestExternalProductReviewEventType_AllActions verifies the canonical event
// name for each admin review action.
func TestExternalProductReviewEventType_AllActions(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"approve", "external_product.review.approved"},
		{"reject", "external_product.review.rejected"},
		{"request_changes", "external_product.review.request_changes"},
		{"hide", "external_product.review.hidden"},
	}
	for _, tc := range cases {
		got := externalProductReviewEventType(tc.action)
		if got != tc.want {
			t.Errorf("externalProductReviewEventType(%q) = %q, want %q", tc.action, got, tc.want)
		}
	}
}

// TestExternalProductReviewEventType_UnknownAction verifies unknown actions
// return an empty string (no event emitted).
func TestExternalProductReviewEventType_UnknownAction(t *testing.T) {
	got := externalProductReviewEventType("unknown_action")
	if got != "" {
		t.Errorf("externalProductReviewEventType(unknown) = %q, want empty string", got)
	}
}

// TestExternalProductReviewEventType_RequestChangesIsDistinctFromReject verifies
// that request_changes emits a distinct event from reject (not aliased).
func TestExternalProductReviewEventType_RequestChangesIsDistinctFromReject(t *testing.T) {
	rejectEvent := externalProductReviewEventType("reject")
	rcEvent := externalProductReviewEventType("request_changes")

	if rejectEvent == rcEvent {
		t.Errorf("request_changes event %q must differ from reject event %q", rcEvent, rejectEvent)
	}
	if !strings.Contains(rcEvent, "request_changes") {
		t.Errorf("request_changes event should contain 'request_changes', got %q", rcEvent)
	}
}

// ---------------------------------------------------------------------------
// OUTBOX EMITTER SPY
// ---------------------------------------------------------------------------

// spyOutboxEmitter records the call args for InsertEvent.
type spyOutboxEmitter struct {
	called    bool
	eventType string
	entityID  uuid.UUID
	payload   map[string]interface{}
	returnErr error
}

func (s *spyOutboxEmitter) InsertEvent(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityID uuid.UUID,
	payload []byte,
) error {
	s.called = true
	s.eventType = eventType
	s.entityID = entityID
	_ = json.Unmarshal(payload, &s.payload)
	return s.returnErr
}

// ---------------------------------------------------------------------------
// emitExternalProductReviewEventTx UNIT TESTS
// ---------------------------------------------------------------------------

func newTestExternalProduct() *entity.ExternalProduct {
	return &entity.ExternalProduct{
		ID:           uuid.New(),
		OwnerUserID:  uuid.New(),
		Title:        "My Test Koi Auction",
		ReviewStatus: entity.ExternalProductReviewStatusApproved,
	}
}

// TestEmitExternalProductReviewEventTx_ApprovedPayload verifies InsertEvent is
// called with the approved event type and a payload containing all required fields.
func TestEmitExternalProductReviewEventTx_ApprovedPayload(t *testing.T) {
	spy := &spyOutboxEmitter{}
	svc := &PromotionService{outboxEmitter: spy}

	product := newTestExternalProduct()
	adminID := uuid.New()

	err := svc.emitExternalProductReviewEventTx(
		context.Background(), nil,
		"external_product.review.approved",
		product, nil, adminID,
	)

	if err != nil {
		t.Fatalf("emitExternalProductReviewEventTx() error = %v, want nil", err)
	}
	if !spy.called {
		t.Fatal("InsertEvent was not called")
	}
	if spy.eventType != "external_product.review.approved" {
		t.Errorf("eventType = %q, want %q", spy.eventType, "external_product.review.approved")
	}
	if spy.entityID != product.ID {
		t.Errorf("entityID = %v, want %v", spy.entityID, product.ID)
	}
	if got := spy.payload["external_product_id"].(string); got != product.ID.String() {
		t.Errorf("payload.external_product_id = %q, want %q", got, product.ID.String())
	}
	if got := spy.payload["owner_user_id"].(string); got != product.OwnerUserID.String() {
		t.Errorf("payload.owner_user_id = %q, want %q", got, product.OwnerUserID.String())
	}
	if got := spy.payload["title"].(string); got != product.Title {
		t.Errorf("payload.title = %q, want %q", got, product.Title)
	}
	if got := spy.payload["review_status"].(string); got != string(product.ReviewStatus) {
		t.Errorf("payload.review_status = %q, want %q", got, product.ReviewStatus)
	}
	if _, hasReason := spy.payload["reason"]; hasReason {
		t.Error("payload.reason should be absent when reason is nil")
	}
}

// TestEmitExternalProductReviewEventTx_RejectedPayloadWithReason verifies the
// reason field is present when a rejection reason is provided.
func TestEmitExternalProductReviewEventTx_RejectedPayloadWithReason(t *testing.T) {
	spy := &spyOutboxEmitter{}
	svc := &PromotionService{outboxEmitter: spy}

	product := newTestExternalProduct()
	product.ReviewStatus = entity.ExternalProductReviewStatusRejected
	adminID := uuid.New()
	reason := "Konten tidak sesuai kebijakan platform"

	err := svc.emitExternalProductReviewEventTx(
		context.Background(), nil,
		"external_product.review.rejected",
		product, &reason, adminID,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := spy.payload["reason"].(string); got != reason {
		t.Errorf("payload.reason = %q, want %q", got, reason)
	}
}

// TestEmitExternalProductReviewEventTx_EmptyReasonOmitted verifies an empty
// reason string is NOT included in the payload (omitempty behaviour).
func TestEmitExternalProductReviewEventTx_EmptyReasonOmitted(t *testing.T) {
	spy := &spyOutboxEmitter{}
	svc := &PromotionService{outboxEmitter: spy}

	product := newTestExternalProduct()
	adminID := uuid.New()
	emptyReason := ""

	_ = svc.emitExternalProductReviewEventTx(
		context.Background(), nil,
		"external_product.review.approved",
		product, &emptyReason, adminID,
	)

	if _, hasReason := spy.payload["reason"]; hasReason {
		t.Error("payload.reason should be absent when reason is empty string")
	}
}

// TestEmitExternalProductReviewEventTx_NilEmitterIsSafe verifies that a nil
// outboxEmitter returns nil without panicking.
func TestEmitExternalProductReviewEventTx_NilEmitterIsSafe(t *testing.T) {
	svc := &PromotionService{outboxEmitter: nil}
	product := newTestExternalProduct()

	err := svc.emitExternalProductReviewEventTx(
		context.Background(), nil,
		"external_product.review.approved",
		product, nil, uuid.New(),
	)

	if err != nil {
		t.Errorf("nil emitter should return nil, got %v", err)
	}
}

// TestEmitExternalProductReviewEventTx_PayloadContainsAllRequiredFields is a
// comprehensive field-coverage assertion.
func TestEmitExternalProductReviewEventTx_PayloadContainsAllRequiredFields(t *testing.T) {
	spy := &spyOutboxEmitter{}
	svc := &PromotionService{outboxEmitter: spy}

	product := newTestExternalProduct()
	reason := "test-reason"
	adminID := uuid.New()

	_ = svc.emitExternalProductReviewEventTx(
		context.Background(), nil,
		"external_product.review.rejected",
		product, &reason, adminID,
	)

	required := []string{"owner_user_id", "external_product_id", "title", "review_status", "reviewed_by", "reason"}
	for _, field := range required {
		if _, ok := spy.payload[field]; !ok {
			t.Errorf("payload missing required field %q", field)
		}
	}
}

// TestSetOutboxEmitter_NilIsSafe verifies that SetOutboxEmitter(nil) does not
// panic and leaves the service in the expected nil-emitter state.
func TestSetOutboxEmitter_NilIsSafe(t *testing.T) {
	svc := &PromotionService{}
	svc.SetOutboxEmitter(nil)

	if svc.outboxEmitter != nil {
		t.Error("outboxEmitter should be nil after SetOutboxEmitter(nil)")
	}
}

// TestExternalProductReviewNotifications_OwnerIsOnlyRecipient verifies that
// only the product owner_user_id is in the payload (no second recipient).
func TestExternalProductReviewNotifications_OwnerIsOnlyRecipient(t *testing.T) {
	spy := &spyOutboxEmitter{}
	svc := &PromotionService{outboxEmitter: spy}

	product := newTestExternalProduct()
	adminID := uuid.New()

	_ = svc.emitExternalProductReviewEventTx(
		context.Background(), nil,
		"external_product.review.approved",
		product, nil, adminID,
	)

	if _, hasBuyer := spy.payload["buyer_id"]; hasBuyer {
		t.Error("payload must not contain buyer_id — only owner receives this notification")
	}
	if got := spy.payload["owner_user_id"].(string); got != product.OwnerUserID.String() {
		t.Errorf("owner_user_id = %q, want %q", got, product.OwnerUserID.String())
	}
}

// ---------------------------------------------------------------------------
// INTEGRATION WITH applyAdminReview VIA MOCK REPO
// ---------------------------------------------------------------------------

// reviewNotificationMockRepo is a minimal implementation of PromotionRepository
// that supports the applyAdminReview path.  All unrelated methods panic so
// misuse is immediately visible.
//
// Compile-time interface satisfaction check:
var _ promotionRepo.PromotionRepository = (*reviewNotificationMockRepo)(nil)

type reviewNotificationMockRepo struct {
	product *entity.ExternalProduct
}

// -- Methods used by applyAdminReview (must be functional) --

func (r *reviewNotificationMockRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ExternalProduct, error) {
	return r.product, nil
}
func (r *reviewNotificationMockRepo) GetDBTime(_ context.Context, _ db.Tx) (time.Time, error) {
	return time.Now().UTC(), nil
}
func (r *reviewNotificationMockRepo) UpdateByID(_ context.Context, _ db.Tx, _ *entity.ExternalProduct) error {
	return nil
}
func (r *reviewNotificationMockRepo) AppendReviewHistory(_ context.Context, _ db.Tx, _ *entity.ExternalProductReviewHistory) error {
	return nil
}

// -- Package methods (panic if called) --

func (r *reviewNotificationMockRepo) CreatePackage(_ context.Context, _ db.Tx, _ *entity.PromotionPackage) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetPackageByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionPackage, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListPackages(_ context.Context, _ db.Tx, _ bool) ([]*entity.PromotionPackage, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) UpdatePackage(_ context.Context, _ db.Tx, _ *entity.PromotionPackage) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListCampaignsAdmin(_ context.Context, _ db.Tx, _ promotionRepo.AdminCampaignFilter) ([]*promotionRepo.AdminCampaignRow, int, error) {
	panic("not implemented")
}

// -- Ownership methods (panic if called) --

func (r *reviewNotificationMockRepo) CreateOwnership(_ context.Context, _ db.Tx, _ *entity.PromotionOwnership) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetOwnershipByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionOwnership, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetOwnershipForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionOwnership, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetOwnershipWithInstances(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionOwnership, []*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) UpdateOwnership(_ context.Context, _ db.Tx, _ *entity.PromotionOwnership) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) AddConsumedDurationToOwnership(_ context.Context, _ db.Tx, _ uuid.UUID, _ int) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListOwnershipsByUser(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.OwnershipStatus) ([]*entity.PromotionOwnership, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListOwnershipsByUserPaginated(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.OwnershipStatus, _, _ int) ([]*entity.PromotionOwnership, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) FindActiveOwnershipByUserAndPackage(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.PromotionOwnership, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListExpiredOwnerships(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionOwnership, error) {
	panic("not implemented")
}

// -- Instance methods (panic if called) --

func (r *reviewNotificationMockRepo) CreateInstance(_ context.Context, _ db.Tx, _ *entity.PromotionInstance) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetInstanceByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetInstanceForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) UpdateInstance(_ context.Context, _ db.Tx, _ *entity.PromotionInstance) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListInstancesByUser(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.InstanceStatus) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListInstancesByOwnership(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetActiveInstanceByOwnership(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetActiveInstanceByOwnershipForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetActiveInstancesByTarget(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetPausedInstancesByTarget(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetActiveInstancesForDiscovery(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetAllActiveInstances(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetAllPausedInstances(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionInstance, error) {
	panic("not implemented")
}

// -- Validation methods (panic if called) --

func (r *reviewNotificationMockRepo) HasActivePromotionForTarget(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) (bool, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetActiveInstanceByTargetForUpdate(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not implemented")
}

// -- External product methods (panic if called, except those used by applyAdminReview above) --

func (r *reviewNotificationMockRepo) CreateDraft(_ context.Context, _ db.Tx, _ *entity.ExternalProduct) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) UpdateOwned(_ context.Context, _ db.Tx, _, _ uuid.UUID, _ entity.ExternalProductUpdateInput) (*entity.ExternalProduct, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) SubmitOwned(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ResubmitOwned(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) GetOwnedByID(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListOwned(_ context.Context, _ db.Tx, _ uuid.UUID, _ promotionRepo.ExternalProductListFilters) ([]*entity.ExternalProduct, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListForReview(_ context.Context, _ db.Tx, _ promotionRepo.ExternalProductAdminListFilters) ([]*entity.ExternalProduct, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListReviewHistory(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.ExternalProductReviewHistory, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) AddMedia(_ context.Context, _ db.Tx, _ *entity.ExternalProductMedia) error {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) ListMedia(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.ExternalProductMedia, error) {
	panic("not implemented")
}
func (r *reviewNotificationMockRepo) SoftDeleteMedia(_ context.Context, _ db.Tx, _, _, _ uuid.UUID) error {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// INTEGRATION TESTS — applyAdminReview emits correct events
// ---------------------------------------------------------------------------

// TestApproveExternalProduct_EmitsApprovedEvent verifies ApproveExternalProduct
// emits the correct event type and payload.
func TestApproveExternalProduct_EmitsApprovedEvent(t *testing.T) {
	ownerID := uuid.New()
	productID := uuid.New()

	product := &entity.ExternalProduct{
		ID:           productID,
		OwnerUserID:  ownerID,
		Title:        "Koi Kohaku Premium",
		ReviewStatus: entity.ExternalProductReviewStatusPendingReview,
	}

	spy := &spyOutboxEmitter{}
	svc := &PromotionService{
		repo:          &reviewNotificationMockRepo{product: product},
		outboxEmitter: spy,
	}

	input := AdminExternalProductReviewInput{
		AdminID:           uuid.New(),
		ExternalProductID: productID,
	}
	_, err := svc.ApproveExternalProduct(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("ApproveExternalProduct() error = %v", err)
	}
	if !spy.called {
		t.Fatal("InsertEvent was not called after approval")
	}
	if spy.eventType != "external_product.review.approved" {
		t.Errorf("eventType = %q, want %q", spy.eventType, "external_product.review.approved")
	}
	if spy.payload["owner_user_id"].(string) != ownerID.String() {
		t.Errorf("payload.owner_user_id mismatch")
	}
}

// TestRejectExternalProduct_EmitsRejectedEventWithReason verifies RejectExternalProduct
// emits the rejected event with the reason in the payload.
func TestRejectExternalProduct_EmitsRejectedEventWithReason(t *testing.T) {
	ownerID := uuid.New()
	productID := uuid.New()
	reason := "URL tidak valid"

	product := &entity.ExternalProduct{
		ID:           productID,
		OwnerUserID:  ownerID,
		Title:        "Koi Tancho",
		ReviewStatus: entity.ExternalProductReviewStatusPendingReview,
	}

	spy := &spyOutboxEmitter{}
	svc := &PromotionService{
		repo:          &reviewNotificationMockRepo{product: product},
		outboxEmitter: spy,
	}

	input := AdminExternalProductReviewInput{
		AdminID:           uuid.New(),
		ExternalProductID: productID,
		Reason:            &reason,
	}
	_, err := svc.RejectExternalProduct(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("RejectExternalProduct() error = %v", err)
	}
	if spy.eventType != "external_product.review.rejected" {
		t.Errorf("eventType = %q, want %q", spy.eventType, "external_product.review.rejected")
	}
	if got := spy.payload["reason"].(string); got != reason {
		t.Errorf("payload.reason = %q, want %q", got, reason)
	}
}

// TestRequestChangesExternalProduct_EmitsRequestChangesEvent verifies the
// request_changes action emits its own distinct event type.
func TestRequestChangesExternalProduct_EmitsRequestChangesEvent(t *testing.T) {
	productID := uuid.New()
	reason := "Tambahkan deskripsi produk"

	product := &entity.ExternalProduct{
		ID:           productID,
		OwnerUserID:  uuid.New(),
		Title:        "Tancho Showa",
		ReviewStatus: entity.ExternalProductReviewStatusPendingReview,
	}

	spy := &spyOutboxEmitter{}
	svc := &PromotionService{
		repo:          &reviewNotificationMockRepo{product: product},
		outboxEmitter: spy,
	}

	input := AdminExternalProductReviewInput{
		AdminID:           uuid.New(),
		ExternalProductID: productID,
		Reason:            &reason,
	}
	_, err := svc.RequestChangesExternalProduct(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("RequestChangesExternalProduct() error = %v", err)
	}
	if spy.eventType != "external_product.review.request_changes" {
		t.Errorf("eventType = %q, want %q", spy.eventType, "external_product.review.request_changes")
	}
}

// TestHideExternalProduct_EmitsHiddenEvent verifies HideExternalProduct emits
// the hidden event type even when the product is already approved.
func TestHideExternalProduct_EmitsHiddenEvent(t *testing.T) {
	productID := uuid.New()
	reason := "Konten melanggar kebijakan"

	product := &entity.ExternalProduct{
		ID:           productID,
		OwnerUserID:  uuid.New(),
		Title:        "Kohaku Asagi",
		ReviewStatus: entity.ExternalProductReviewStatusApproved,
	}

	spy := &spyOutboxEmitter{}
	svc := &PromotionService{
		repo:          &reviewNotificationMockRepo{product: product},
		outboxEmitter: spy,
	}

	input := AdminExternalProductReviewInput{
		AdminID:           uuid.New(),
		ExternalProductID: productID,
		Reason:            &reason,
	}
	_, err := svc.HideExternalProduct(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("HideExternalProduct() error = %v", err)
	}
	if spy.eventType != "external_product.review.hidden" {
		t.Errorf("eventType = %q, want %q", spy.eventType, "external_product.review.hidden")
	}
}

// TestApproveExternalProduct_InvalidTransition_NoEvent verifies that an invalid
// state transition (approve an already-approved product) does not emit any event.
func TestApproveExternalProduct_InvalidTransition_NoEvent(t *testing.T) {
	productID := uuid.New()

	// Already approved — CanApprove() returns false
	product := &entity.ExternalProduct{
		ID:           productID,
		OwnerUserID:  uuid.New(),
		Title:        "Already Approved",
		ReviewStatus: entity.ExternalProductReviewStatusApproved,
	}

	spy := &spyOutboxEmitter{}
	svc := &PromotionService{
		repo:          &reviewNotificationMockRepo{product: product},
		outboxEmitter: spy,
	}

	input := AdminExternalProductReviewInput{
		AdminID:           uuid.New(),
		ExternalProductID: productID,
	}
	_, err := svc.ApproveExternalProduct(context.Background(), nil, input)
	if err == nil {
		t.Fatal("expected invalid-transition error, got nil")
	}
	if spy.called {
		t.Error("InsertEvent must NOT be called on invalid transition")
	}
}
