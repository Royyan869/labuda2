package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	socialRepo "github.com/labuda/backend/internal/social/graph"
	"github.com/labuda/backend/pkg/db"
)

type mockAttachmentTx struct {
	queryRowFn func(query string, args ...any) pgx.Row
}

func (m *mockAttachmentTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockAttachmentTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockAttachmentTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(query, args...)
	}
	return &mockAttachmentRow{}
}

func (m *mockAttachmentTx) Commit(context.Context) error   { return nil }
func (m *mockAttachmentTx) Rollback(context.Context) error { return nil }

type mockAttachmentRow struct {
	scanFn func(dest ...any) error
}

func (m *mockAttachmentRow) Scan(dest ...any) error {
	if m.scanFn != nil {
		return m.scanFn(dest...)
	}
	return nil
}

type mockAttachmentSocialRepo struct {
	existsBlockFn func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
}

func (m *mockAttachmentSocialRepo) InsertFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) DeleteFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) DeleteFollowBothDirections(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) ExistsFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockAttachmentSocialRepo) ListFollowers(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockAttachmentSocialRepo) ListFollowing(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockAttachmentSocialRepo) InsertBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) DeleteBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) ExistsBlock(
	ctx context.Context,
	tx interface{},
	userA, userB uuid.UUID,
) (bool, error) {
	if m.existsBlockFn != nil {
		return m.existsBlockFn(ctx, tx, userA, userB)
	}
	return false, nil
}

func (m *mockAttachmentSocialRepo) AcquireFollowLock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) IsBlockedBy(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockAttachmentSocialRepo) InsertMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) DeleteMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockAttachmentSocialRepo) ExistsMute(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockAttachmentSocialRepo) ListMuted(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockAttachmentSocialRepo) ListBlocked(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

// mockCommerceRefValidator implements commerceResponse.Validator for tests.
type mockCommerceRefValidator struct {
	pass bool // true = resource exists + displayable; false = not found
}

func (m *mockCommerceRefValidator) ValidateReference(_ context.Context, _ db.Tx, _ commerceResponse.ResourceType, _ uuid.UUID) error {
	if m.pass {
		return nil
	}
	return commerceResponse.ErrResourceNotFound
}

var _ db.Tx = (*mockAttachmentTx)(nil)
var _ socialRepo.SocialRepository = (*mockAttachmentSocialRepo)(nil)
var _ commerceResponse.Validator = (*mockCommerceRefValidator)(nil)

// =====================================================================
// Allows valid references (any user can reference any displayable resource)
// =====================================================================

func TestValidateAttachmentReferences_AllowsCanonicalForSaleAuctionPostRequestReferences(t *testing.T) {
	senderID := uuid.MustParse("00000000-0000-0000-0000-000000000040")
	forSaleID := uuid.MustParse("00000000-0000-0000-0000-000000000041")
	auctionID := uuid.MustParse("00000000-0000-0000-0000-000000000042")
	contentID := uuid.MustParse("00000000-0000-0000-0000-000000000043")

	// Validator passes — resource exists and is displayable.
	mockValidator := &mockCommerceRefValidator{pass: true}

	service := &Service{
		commerceRefValidator: mockValidator,
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	tx := &mockAttachmentTx{
		queryRowFn: func(query string, args ...any) pgx.Row {
			return &mockAttachmentRow{
				scanFn: func(dest ...any) error {
					if len(dest) > 0 {
						if idPtr, ok := dest[0].(*uuid.UUID); ok {
							switch len(args) {
							case 1:
								*idPtr = args[0].(uuid.UUID)
							default:
								*idPtr = contentID
							}
						}
					}
					return nil
				},
			}
		},
	}

	tests := []struct {
		name       string
		attachment map[string]interface{}
	}{
		{
			name: "for_sale reference",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "for_sale",
					"target_id":   forSaleID.String(),
				},
			},
		},
		{
			name: "auction reference",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "auction",
					"target_id":   auctionID.String(),
				},
			},
		},
		{
			name: "post reference",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "post",
					"target_id":   contentID.String(),
				},
			},
		},
		{
			name: "request reference",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "request",
					"target_id":   contentID.String(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateAttachmentReferences(
				context.Background(),
				tx,
				senderID,
				tt.attachment,
			)
			if err != nil {
				t.Fatalf("expected no error for %s, got: %v", tt.name, err)
			}
		})
	}
}

func TestValidateAttachmentReferences_CommerceRefValidatorMissing_RejectsForSale(t *testing.T) {
	forSaleID := uuid.MustParse("00000000-0000-0000-0000-000000000081")

	// Service with NO commerceRefValidator wired — must fail closed.
	service := &Service{
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "for_sale",
			"data": map[string]interface{}{
				"for_sale_id": forSaleID.String(),
			},
		},
	)

	if err == nil {
		t.Fatal("expected error when commerceRefValidator is nil (fail-closed), got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected 'not configured' error, got: %v", err)
	}
}

func TestValidateAttachmentReferences_CommerceRefValidatorMissing_RejectsAuction(t *testing.T) {
	auctionID := uuid.MustParse("00000000-0000-0000-0000-000000000082")

	service := &Service{
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "auction",
			"data": map[string]interface{}{
				"auction_id": auctionID.String(),
			},
		},
	)

	if err == nil {
		t.Fatal("expected error when commerceRefValidator is nil (fail-closed), got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected 'not configured' error, got: %v", err)
	}
}

func TestValidateAttachmentReferences_CommerceRefValidatorMissing_RejectsReferenceType(t *testing.T) {
	forSaleID := uuid.MustParse("00000000-0000-0000-0000-000000000083")

	service := &Service{
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "for_sale",
				"target_id":   forSaleID.String(),
			},
		},
	)

	if err == nil {
		t.Fatal("expected error when commerceRefValidator is nil (fail-closed), got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected 'not configured' error, got: %v", err)
	}
}

// =====================================================================
// ANY USER CAN REFERENCE: displayable resources pass regardless of caller
// =====================================================================

func TestValidateAttachmentReferences_AnyUserCanReference_ForSale(t *testing.T) {
	forSaleID := uuid.MustParse("00000000-0000-0000-0000-0000000000A1")

	// Validator passes — resource exists and is displayable.
	mockValidator := &mockCommerceRefValidator{pass: true}
	service := &Service{
		commerceRefValidator: mockValidator,
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	// ANY user can reference — ownership is NOT checked.
	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "for_sale",
			"data": map[string]interface{}{
				"for_sale_id": forSaleID.String(),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected no error for any-user for_sale reference, got: %v", err)
	}
}

func TestValidateAttachmentReferences_AnyUserCanReference_Auction(t *testing.T) {
	auctionID := uuid.MustParse("00000000-0000-0000-0000-0000000000A2")

	mockValidator := &mockCommerceRefValidator{pass: true}
	service := &Service{
		commerceRefValidator: mockValidator,
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "auction",
			"data": map[string]interface{}{
				"auction_id": auctionID.String(),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected no error for any-user auction reference, got: %v", err)
	}
}

func TestValidateAttachmentReferences_RejectsNotFound_ForSale(t *testing.T) {
	forSaleID := uuid.MustParse("00000000-0000-0000-0000-0000000000B1")

	// Validator rejects: resource not found.
	mockValidator := &mockCommerceRefValidator{pass: false}
	service := &Service{
		commerceRefValidator: mockValidator,
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "for_sale",
			"data": map[string]interface{}{
				"for_sale_id": forSaleID.String(),
			},
		},
	)
	if err == nil {
		t.Fatal("expected rejection for not-found for_sale")
	}
}

func TestValidateAttachmentReferences_RejectsNotFound_Auction(t *testing.T) {
	auctionID := uuid.MustParse("00000000-0000-0000-0000-0000000000B2")

	mockValidator := &mockCommerceRefValidator{pass: false}
	service := &Service{
		commerceRefValidator: mockValidator,
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	senderID := uuid.New()
	err := service.validateAttachmentReferences(
		context.Background(),
		&mockAttachmentTx{},
		senderID,
		map[string]interface{}{
			"type": "auction",
			"data": map[string]interface{}{
				"auction_id": auctionID.String(),
			},
		},
	)
	if err == nil {
		t.Fatal("expected rejection for not-found auction")
	}
}
