//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/identity/auth"
	contentapp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

type occurrenceAccountChecker struct{}

func (occurrenceAccountChecker) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (occurrenceAccountChecker) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}
func (occurrenceAccountChecker) IsBanned(context.Context, uuid.UUID) (bool, error) { return false, nil }

type occurrenceRoleChecker struct {
	hasCapability bool
}

func (r occurrenceRoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (r occurrenceRoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return r.hasCapability, nil
}
func (r occurrenceRoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

func newOccurrenceService(hasCapability bool) *contentapp.ContentService {
	return contentapp.NewContentService(
		contentrepo.NewContentRepository(),
		nil,
		occurrenceRoleChecker{hasCapability: hasCapability},
		occurrenceAccountChecker{},
		nil,
	)
}

type occurrenceExec interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func seedOccurrenceUser(t *testing.T, ctx context.Context, tx occurrenceExec, status string) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
	`, userID, userID.String(), fmt.Sprintf("%s@test.invalid", userID), status)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, uuid.New(), userID, "u-"+strings.ReplaceAll(userID.String(), "-", "")[:20])
	require.NoError(t, err)
	return userID
}

func seedOccurrenceForSale(t *testing.T, ctx context.Context, tx occurrenceExec, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()

	productID := uuid.New()
	saleID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, productID, sellerID, "Product "+saleID.String()[:8], "desc", `[]`, "kohaku", "immediate")
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6)
	`, saleID, productID, sellerID, int64(250000), status, 1)
	require.NoError(t, err)
	return saleID
}

func seedOccurrenceAuction(t *testing.T, ctx context.Context, tx occurrenceExec, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()

	productID := uuid.New()
	auctionID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, productID, sellerID, "Auction product "+auctionID.String()[:8], "desc", `[]`, "kohaku", "immediate")
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO auctions (
			id, seller_id, product_id,
			start_price, bid_increment, start_at, end_at, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', $6, NOW(), NOW())
	`, auctionID, sellerID, productID, int64(100000), int64(10000), status)
	require.NoError(t, err)
	return auctionID
}

func createOrdinaryContent(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	svc *contentapp.ContentService,
	authorID uuid.UUID,
	caption string,
) *contententity.Content {
	t.Helper()

	content, err := svc.CreateContent(
		ctx,
		tx,
		authorID,
		caption,
		contententity.VisibilityPublic,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	return content
}

func assertCanonicalOccurrenceState(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
	wantActorID uuid.UUID,
	wantResourceType contententity.ContentResourceOccurrenceResourceType,
	wantResourceID uuid.UUID,
) {
	t.Helper()

	var shareRefExists bool
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'contents'
			  AND column_name = 'share_reference'
		)
	`).Scan(&shareRefExists))
	require.False(t, shareRefExists)

	occ, err := contentrepo.NewContentRepository().GetResourceOccurrenceByContentID(ctx, tx, contentID)
	require.NoError(t, err)
	require.Equal(t, wantActorID, occ.ActorID)
	require.Equal(t, wantResourceType, occ.ResourceType())
	require.Equal(t, wantResourceID, occ.SourceID())
}

func TestCreateContent_OrdinaryContentWithoutOccurrence_Succeeds(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := newOccurrenceService(true)
	authorID := seedOccurrenceUser(t, ctx, tdb.Pool(), "active")

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		content := createOrdinaryContent(t, ctx, tx, svc, authorID, "plain content")
		require.NotNil(t, content)
		require.False(t, content.IsRepost())
		return nil
	})
	require.NoError(t, err)
}

func TestCreateContentWithResourceOccurrence_Matrix(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()

	cases := []struct {
		name            string
		operation       contententity.ContentResourceOccurrenceOperation
		resourceType    contententity.ContentResourceOccurrenceResourceType
		hasCapability   bool
		setup           func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID)
		wantErrIs       error
		wantErrContains string
	}{
		{
			name:          "CW2 share_to_feed profile succeeds",
			operation:     contententity.ContentResourceOccurrenceOperationShareToFeed,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeProfile,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				targetID := seedOccurrenceUser(t, ctx, tx, "active")
				return targetID, actorID
			},
		},
		{
			name:          "CW3 share_to_feed content succeeds",
			operation:     contententity.ContentResourceOccurrenceOperationShareToFeed,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeContent,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				targetAuthor := seedOccurrenceUser(t, ctx, tx, "active")
				target := createOrdinaryContent(t, ctx, tx, svc, targetAuthor, "target content")
				return target.ID, actorID
			},
		},
		{
			name:          "CW4 share_to_feed fps succeeds",
			operation:     contententity.ContentResourceOccurrenceOperationShareToFeed,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeForSale,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				sellerID := seedOccurrenceUser(t, ctx, tx, "active")
				saleID := seedOccurrenceForSale(t, ctx, tx, sellerID, "active")
				return saleID, actorID
			},
		},
		{
			name:          "CW5 share_to_feed auction succeeds",
			operation:     contententity.ContentResourceOccurrenceOperationShareToFeed,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeAuction,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				sellerID := seedOccurrenceUser(t, ctx, tx, "active")
				auctionID := seedOccurrenceAuction(t, ctx, tx, sellerID, "active")
				return auctionID, actorID
			},
		},
		{
			name:          "CW6 direct fps succeeds for owner with market authority",
			operation:     contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeForSale,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				ownerID := seedOccurrenceUser(t, ctx, tx, "active")
				saleID := seedOccurrenceForSale(t, ctx, tx, ownerID, "active")
				return saleID, ownerID
			},
		},
		{
			name:          "CW7 direct auction succeeds for owner with market authority",
			operation:     contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeAuction,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				ownerID := seedOccurrenceUser(t, ctx, tx, "active")
				auctionID := seedOccurrenceAuction(t, ctx, tx, ownerID, "active")
				return auctionID, ownerID
			},
		},
		{
			name:            "CW8 direct profile rejected",
			operation:       contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:    contententity.ContentResourceOccurrenceResourceTypeProfile,
			hasCapability:   true,
			wantErrContains: "invalid resource type for direct commerce insert: profile",
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				targetID := seedOccurrenceUser(t, ctx, tx, "active")
				return targetID, actorID
			},
		},
		{
			name:            "CW9 direct content rejected",
			operation:       contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:    contententity.ContentResourceOccurrenceResourceTypeContent,
			hasCapability:   true,
			wantErrContains: "invalid resource type for direct commerce insert: content",
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				targetAuthor := seedOccurrenceUser(t, ctx, tx, "active")
				target := createOrdinaryContent(t, ctx, tx, svc, targetAuthor, "target content")
				return target.ID, actorID
			},
		},
		{
			name:          "CW10 direct fps other seller rejected",
			operation:     contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeForSale,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				ownerID := seedOccurrenceUser(t, ctx, tx, "active")
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				saleID := seedOccurrenceForSale(t, ctx, tx, ownerID, "active")
				return saleID, actorID
			},
		},
		{
			name:          "CW11 direct auction other seller rejected",
			operation:     contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeAuction,
			hasCapability: true,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				ownerID := seedOccurrenceUser(t, ctx, tx, "active")
				actorID := seedOccurrenceUser(t, ctx, tx, "active")
				auctionID := seedOccurrenceAuction(t, ctx, tx, ownerID, "active")
				return auctionID, actorID
			},
		},
		{
			name:          "CW12 direct market-authority failure rejected",
			operation:     contententity.ContentResourceOccurrenceOperationDirectCommerceInsertContent,
			resourceType:  contententity.ContentResourceOccurrenceResourceTypeForSale,
			hasCapability: false,
			wantErrIs:     auth.ErrMarketAuthorityRequired,
			setup: func(t *testing.T, ctx context.Context, tx db.Tx, svc *contentapp.ContentService) (uuid.UUID, uuid.UUID) {
				ownerID := seedOccurrenceUser(t, ctx, tx, "active")
				saleID := seedOccurrenceForSale(t, ctx, tx, ownerID, "active")
				return saleID, ownerID
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc := newOccurrenceService(tc.hasCapability)
			err := tdb.WithTx(ctx, func(tx db.Tx) error {
				resourceID, actorID := tc.setup(t, ctx, tx, svc)
				content, createErr := svc.CreateContentWithResourceOccurrence(
					ctx,
					tx,
					actorID,
					"canonical content",
					contententity.VisibilityPublic,
					nil,
					nil,
					&contententity.ContentResourceOccurrenceIdentity{
						Operation:    tc.operation,
						ResourceType: tc.resourceType,
						ResourceID:   resourceID,
					},
					nil,
				)

				if tc.wantErrIs != nil || tc.wantErrContains != "" {
					require.Error(t, createErr)
					if tc.wantErrIs != nil {
						require.ErrorIs(t, createErr, tc.wantErrIs)
					}
					if tc.wantErrContains != "" {
						require.Contains(t, createErr.Error(), tc.wantErrContains)
					}
					return nil
				}

				require.NoError(t, createErr)
				require.NotNil(t, content)
				assertCanonicalOccurrenceState(t, ctx, tx, content.ID, actorID, tc.resourceType, resourceID)
				return nil
			})
			require.NoError(t, err)
		})
	}
}

func TestContentResourceOccurrence_AtomicRollbackOnDuplicateInsert(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := newOccurrenceService(true)
	repo := contentrepo.NewContentRepository()
	actorID := seedOccurrenceUser(t, ctx, tdb.Pool(), "active")
	var contentID uuid.UUID

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		content := createOrdinaryContent(t, ctx, tx, svc, actorID, "atomic duplicate")
		contentID = content.ID
		occ := contententity.NewContentResourceOccurrence(
			content.ID,
			actorID,
			&contententity.ContentResourceOccurrenceIdentity{
				Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
				ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
				ResourceID:   actorID,
			},
		)

		require.NoError(t, repo.CreateResourceOccurrence(ctx, tx, occ))
		return repo.CreateResourceOccurrence(ctx, tx, occ)
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, contentrepo.ErrDuplicateContentResourceOccurrence) || strings.Contains(err.Error(), "duplicate"))

	var contentCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM contents
		WHERE id = $1
	`, contentID).Scan(&contentCount))
	require.Equal(t, 0, contentCount)

	var occurrenceCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM content_resource_occurrences
		WHERE content_id = $1
	`, contentID).Scan(&occurrenceCount))
	require.Equal(t, 0, occurrenceCount)
}
