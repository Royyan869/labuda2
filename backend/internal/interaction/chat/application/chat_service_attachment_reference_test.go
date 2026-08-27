package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
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

type mockAttachmentChecker struct {
	getByIDFn func(ctx context.Context, tx db.Tx, id uuid.UUID) (interface{}, error)
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

func (m *mockAttachmentChecker) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (interface{}, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, tx, id)
	}
	return struct{}{}, nil
}

var _ db.Tx = (*mockAttachmentTx)(nil)
var _ socialRepo.SocialRepository = (*mockAttachmentSocialRepo)(nil)
var _ ForSaleChecker = (*mockAttachmentChecker)(nil)
var _ AuctionChecker = (*mockAttachmentChecker)(nil)

func TestValidateAttachmentReferences_AllowsCanonicalForSaleAuctionPostRequestReferences(t *testing.T) {
	senderID := uuid.MustParse("00000000-0000-0000-0000-000000000040")
	forSaleID := uuid.MustParse("00000000-0000-0000-0000-000000000041")
	auctionID := uuid.MustParse("00000000-0000-0000-0000-000000000042")
	contentID := uuid.MustParse("00000000-0000-0000-0000-000000000043")

	service := &Service{
		forSaleChecker: &mockAttachmentChecker{
			getByIDFn: func(context.Context, db.Tx, uuid.UUID) (interface{}, error) {
				return struct{}{}, nil
			},
		},
		auctionChecker: &mockAttachmentChecker{
			getByIDFn: func(context.Context, db.Tx, uuid.UUID) (interface{}, error) {
				return struct{}{}, nil
			},
		},
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

	cases := []struct {
		name       string
		attachment map[string]interface{}
	}{
		{
			name: "for_sale",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "for_sale",
					"target_id":   forSaleID.String(),
					"preview": map[string]interface{}{
						"title": "Fixed-price sale",
					},
				},
			},
		},
		{
			name: "auction",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "auction",
					"target_id":   auctionID.String(),
					"preview": map[string]interface{}{
						"title": "Auction",
					},
				},
			},
		},
		{
			name: "post",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "post",
					"target_id":   contentID.String(),
					"preview": map[string]interface{}{
						"title": "Post",
					},
				},
			},
		},
		{
			name: "request",
			attachment: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "request",
					"target_id":   contentID.String(),
					"preview": map[string]interface{}{
						"title": "Request",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.validateAttachmentReferences(
				context.Background(),
				tx,
				senderID,
				tc.attachment,
			)
			if err != nil {
				t.Fatalf("expected %s reference to validate, got %v", tc.name, err)
			}
		})
	}
}

func TestValidateAttachmentReferences_AllowsPublicProfileReference(t *testing.T) {
	senderID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	profileID := uuid.MustParse("00000000-0000-0000-0000-000000000011")

	service := &Service{
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		},
	}

	tx := &mockAttachmentTx{
		queryRowFn: func(query string, args ...any) pgx.Row {
			if !contains(query, "FROM users") {
				return &mockAttachmentRow{}
			}
			return &mockAttachmentRow{
				scanFn: func(dest ...any) error {
					if len(dest) == 0 {
						return nil
					}
					if idPtr, ok := dest[0].(*uuid.UUID); ok {
						*idPtr = profileID
					}
					return nil
				},
			}
		},
	}

	err := service.validateAttachmentReferences(
		context.Background(),
		tx,
		senderID,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "profile",
				"target_id":   profileID.String(),
			},
		},
	)

	if err != nil {
		t.Fatalf("expected public profile reference to validate, got %v", err)
	}
}

func TestValidateAttachmentReferences_RejectsBlockedProfileReference(t *testing.T) {
	senderID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	profileID := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	usersQueried := false

	service := &Service{
		socialRepo: &mockAttachmentSocialRepo{
			existsBlockFn: func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
				return true, nil
			},
		},
	}

	tx := &mockAttachmentTx{
		queryRowFn: func(query string, args ...any) pgx.Row {
			usersQueried = usersQueried || contains(query, "FROM users")
			return &mockAttachmentRow{}
		},
	}

	err := service.validateAttachmentReferences(
		context.Background(),
		tx,
		senderID,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "profile",
				"target_id":   profileID.String(),
			},
		},
	)

	if !errors.Is(err, chatRepo.ErrAttachmentProfileNotFound) {
		t.Fatalf("expected ErrAttachmentProfileNotFound, got %v", err)
	}
	if usersQueried {
		t.Fatal("expected blocked profile to fail before profile lookup")
	}
}

func TestValidateAttachmentReferences_RejectsMissingProfileReference(t *testing.T) {
	senderID := uuid.MustParse("00000000-0000-0000-0000-000000000030")
	profileID := uuid.MustParse("00000000-0000-0000-0000-000000000031")

	service := &Service{
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
					return pgx.ErrNoRows
				},
			}
		},
	}

	err := service.validateAttachmentReferences(
		context.Background(),
		tx,
		senderID,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "profile",
				"target_id":   profileID.String(),
			},
		},
	)

	if !errors.Is(err, chatRepo.ErrAttachmentProfileNotFound) {
		t.Fatalf("expected ErrAttachmentProfileNotFound, got %v", err)
	}
}

func TestValidateAttachmentReferences_RejectsDeletedProfileReference(t *testing.T) {
	senderID := uuid.MustParse("00000000-0000-0000-0000-000000000032")
	profileID := uuid.MustParse("00000000-0000-0000-0000-000000000033")

	service := &Service{
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
					return pgx.ErrNoRows
				},
			}
		},
	}

	err := service.validateAttachmentReferences(
		context.Background(),
		tx,
		senderID,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "profile",
				"target_id":   profileID.String(),
			},
		},
	)

	if !errors.Is(err, chatRepo.ErrAttachmentProfileNotFound) {
		t.Fatalf("expected ErrAttachmentProfileNotFound, got %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}


