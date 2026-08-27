//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contentProjectionFixture struct {
	appDB    *db.DB
	traced   *db.DB
	tracer   *queryCountingTracer
	resolver *contentProjectionBatchResolver
	cleanup  func()
}

func newContentProjectionFixture(t *testing.T) *contentProjectionFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	ctx := context.Background()

	baseCfg := *tdb.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer

	tracedPool, err := pgxpool.NewWithConfig(ctx, &baseCfg)
	require.NoError(t, err)

	fx := &contentProjectionFixture{
		appDB:    db.NewFromPool(tdb.Pool()),
		traced:   db.NewFromPool(tracedPool),
		tracer:   tracer,
		resolver: newContentProjectionBatchResolver(db.NewFromPool(tracedPool)),
		cleanup: func() {
			tracedPool.Close()
			cleanup()
		},
	}

	t.Cleanup(fx.cleanup)
	return fx
}

func (f *contentProjectionFixture) seedUser(
	t *testing.T,
	accountStatus string,
	deletedAt *time.Time,
	username string,
	avatarURL *string,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, deleted_at, created_at, updated_at, role)
		VALUES ($1, $2, $3, NOW(), true, $4, $5, $6, $6, 'user')
	`, id, id.String(), id.String()+"@test.local", accountStatus, deletedAt, now)
	require.NoError(t, err)

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, uuid.New(), id, username, avatarURL)
	require.NoError(t, err)

	return id
}

func (f *contentProjectionFixture) seedBlock(t *testing.T, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func (f *contentProjectionFixture) seedFollow(t *testing.T, followerID, followingID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_follows (follower_id, following_id, created_at)
		VALUES ($1, $2, NOW())
	`, followerID, followingID)
	require.NoError(t, err)
}

func (f *contentProjectionFixture) seedContent(
	t *testing.T,
	authorID uuid.UUID,
	visibility entity.Visibility,
	status entity.Status,
	isHidden bool,
	deletedAt *time.Time,
	caption *string,
	createdAt time.Time,
	shareReference json.RawMessage,
	originalAuthorID *uuid.UUID,
) uuid.UUID {
	t.Helper()

	contentID := uuid.New()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO contents (
			id, author_id, status, caption,
			visibility, is_hidden, original_author_id,
			created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)
	`, contentID, authorID, string(status), caption, string(visibility), isHidden, originalAuthorID, createdAt, deletedAt)
	require.NoError(t, err)

	if len(shareReference) > 0 {
		var raw struct {
			TargetType string `json:"targetType"`
			TargetID   string `json:"targetId"`
		}
		require.NoError(t, json.Unmarshal(shareReference, &raw))

		targetType, err := entity.ParseShareTargetType(raw.TargetType)
		require.NoError(t, err)
		targetID, err := uuid.Parse(raw.TargetID)
		require.NoError(t, err)

		var profileSourceID, contentSourceID, forSaleSourceID, auctionSourceID *uuid.UUID
		switch targetType {
		case entity.ShareTargetTypeProfile:
			profileSourceID = &targetID
		case entity.ShareTargetTypeContent:
			contentSourceID = &targetID
		case entity.ShareTargetTypeForSale:
			forSaleSourceID = &targetID
		case entity.ShareTargetTypeAuction:
			auctionSourceID = &targetID
		default:
			t.Fatalf("unsupported share target type %q", targetType)
		}

		_, err = f.appDB.Pool().Exec(context.Background(), `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation,
				profile_source_id, content_source_id, for_sale_source_id, auction_source_id,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, contentID, authorID, string(entity.ContentResourceOccurrenceOperationShareToFeed), profileSourceID, contentSourceID, forSaleSourceID, auctionSourceID, createdAt)
		require.NoError(t, err)
	}

	return contentID
}

func (f *contentProjectionFixture) seedMedia(
	t *testing.T,
	contentID uuid.UUID,
	position int,
	mediaURL string,
	mediaType string,
) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO content_media (id, content_id, media_url, media_type, position, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, uuid.New(), contentID, mediaURL, mediaType, position)
	require.NoError(t, err)
}

func (f *contentProjectionFixture) seedProduct(
	t *testing.T,
	sellerID uuid.UUID,
	title string,
	description string,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
	`, id, sellerID, title, description, json.RawMessage(`[]`), "Kohaku", string(fpsEntity.PreparationTimeImmediate), now)
	require.NoError(t, err)

	return id
}

func (f *contentProjectionFixture) seedForSale(
	t *testing.T,
	sellerID uuid.UUID,
	status fpsEntity.ForSaleStatus,
	publishedAt *time.Time,
) uuid.UUID {
	t.Helper()

	productID := f.seedProduct(t, sellerID, "fixture listing product", "fixture listing product")
	id := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9, $9)
	`, id, productID, sellerID, int64(100000), false, string(status), publishedAt, 1, now)
	require.NoError(t, err)

	return id
}

func (f *contentProjectionFixture) seedAuction(
	t *testing.T,
	sellerID uuid.UUID,
	status auctionEntity.Status,
) uuid.UUID {
	t.Helper()

	productID := f.seedProduct(t, sellerID, "fixture auction product", "fixture auction product")
	id := uuid.New()
	now := time.Now().UTC()
	startAt := now.Add(-1 * time.Hour)
	endAt := now.Add(1 * time.Hour)
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO auctions (
			id, seller_id, product_id, order_id, settlement_deadline,
			start_price, bid_increment, buy_now_price, start_at, end_at, current_bid,
			current_winner_id, status, created_at,
			updated_at, anti_snipe_extension_seconds
		)
		VALUES ($1, $2, $3, NULL, NULL, $4, $5, NULL, $6, $7, NULL, NULL, $8, $9, $9, $10)
	`, id, sellerID, productID, int64(100000), int64(5000), startAt, endAt, string(status), now, int64(0))
	require.NoError(t, err)

	return id
}

func (f *contentProjectionFixture) resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return f.resolver.ResolveContents(ctx, viewerID, occurrences)
}

func newContentOccurrence(messageID, contentID uuid.UUID) *chatEntity.ChatMessageResourceOccurrence {
	return chatEntity.NewChatMessageResourceOccurrence(
		messageID,
		chatEntity.ResourceOccurrenceOperationShareToChat,
		chatEntity.ResourceOccurrenceResourceTypeContent,
		contentID,
		json.RawMessage(`{}`),
	)
}

func requireLiveContentProjection(t *testing.T, proj *chatApp.ResourceProjection) chatApp.ContentLivePayload {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateLive, proj.State)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeContent, proj.Identity.ResourceType)
	require.NotNil(t, proj.Payload)
	require.Nil(t, proj.CommerceActions)

	payload, ok := proj.Payload.(chatApp.ContentLivePayload)
	require.True(t, ok, "expected ContentLivePayload, got %T", proj.Payload)
	require.NotNil(t, payload.Author.Lifecycle)
	require.NotNil(t, payload.Media)
	return payload
}

func requireNestedResourceIndicator(
	t *testing.T,
	got *chatApp.NestedResourceIndicator,
	wantType chatEntity.ResourceOccurrenceResourceType,
	wantID uuid.UUID,
) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, wantType, got.ResourceType)
	require.Equal(t, wantID, got.ResourceID)
}

func requireTombstoneContentProjection(t *testing.T, proj *chatApp.ResourceProjection) {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateTombstone, proj.State)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeContent, proj.Identity.ResourceType)
	require.Nil(t, proj.Payload)
	require.Nil(t, proj.CommerceActions)
	assert.True(t, proj.ViewerCapabilities.BlockedByTombstone)
	assert.False(t, proj.ViewerCapabilities.CanView)
	assert.False(t, proj.ViewerCapabilities.CanInteract)
	assert.Equal(t, uuid.Nil, proj.Identity.ResourceID)
}

func TestContentProjectionResolver_MixedStatesAndPayloadContract(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerAvatar := "https://cdn.example.test/viewer.png"
	viewerID := fx.seedUser(t, "active", nil, "viewer", &viewerAvatar)

	publicAvatar := "https://cdn.example.test/public.png"
	publicAuthorID := fx.seedUser(t, "active", nil, "public_author", &publicAvatar)

	followedAuthorAvatar := "https://cdn.example.test/followed.png"
	followedAuthorID := fx.seedUser(t, "active", nil, "followed_author", &followedAuthorAvatar)
	fx.seedFollow(t, viewerID, followedAuthorID)

	notFollowedAuthorID := fx.seedUser(t, "active", nil, "not_followed", nil)
	privateSelfID := viewerID

	blockedByViewerID := fx.seedUser(t, "active", nil, "blocked_by_viewer", nil)
	fx.seedBlock(t, viewerID, blockedByViewerID)

	blocksViewerID := fx.seedUser(t, "active", nil, "blocks_viewer", nil)
	fx.seedBlock(t, blocksViewerID, viewerID)

	hiddenAuthorID := fx.seedUser(t, "active", nil, "hidden_author", nil)
	deletedStatusAuthorID := fx.seedUser(t, "active", nil, "deleted_status_author", nil)
	deletedAtTime := time.Now().UTC()
	deletedAtAuthorID := fx.seedUser(t, "active", &deletedAtTime, "deleted_at_author", nil)
	suspendedAuthorID := fx.seedUser(t, "suspended", nil, "suspended_author", nil)
	bannedAuthorID := fx.seedUser(t, "banned", nil, "banned_author", nil)
	removedAtTime := time.Now().UTC()
	removedAuthorID := fx.seedUser(t, "active", &removedAtTime, "removed_author", nil)

	shareRefAuthorID := fx.seedUser(t, "active", nil, "share_ref_author", nil)
	shareRefOriginalAuthorID := fx.seedUser(t, "active", nil, "share_ref_original_author", nil)
	shareRefTargetID := uuid.New()
	shareReference := json.RawMessage(fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"shared","imageUrl":"https://cdn.example.test/original.jpg","isAvailable":true}}`, shareRefTargetID))

	base := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	_, err := fx.appDB.Pool().Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption,
			visibility, is_hidden, original_author_id,
			created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, NULL)
	`, shareRefTargetID, shareRefOriginalAuthorID, string(entity.StatusActive), "shared target", string(entity.VisibilityPublic), false, nil, base.Add(14*time.Minute))
	require.NoError(t, err)
	liveCaption := "public caption"
	followedCaption := "followers caption"
	privateCaption := "private caption"
	shareCaption := "share caption"

	publicContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, &liveCaption, base, nil, nil)
	selfContentID := fx.seedContent(t, viewerID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(1*time.Minute), nil, nil)
	followedContentID := fx.seedContent(t, followedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, &followedCaption, base.Add(2*time.Minute), nil, nil)
	notFollowedContentID := fx.seedContent(t, notFollowedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, base.Add(3*time.Minute), nil, nil)
	followedSelfContentID := fx.seedContent(t, viewerID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, base.Add(4*time.Minute), nil, nil)
	privateSelfContentID := fx.seedContent(t, privateSelfID, entity.VisibilityPrivate, entity.StatusActive, false, nil, &privateCaption, base.Add(5*time.Minute), nil, nil)
	privateOtherContentID := fx.seedContent(t, notFollowedAuthorID, entity.VisibilityPrivate, entity.StatusActive, false, nil, nil, base.Add(6*time.Minute), nil, nil)
	blockedByViewerContentID := fx.seedContent(t, blockedByViewerID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(7*time.Minute), nil, nil)
	blocksViewerContentID := fx.seedContent(t, blocksViewerID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(8*time.Minute), nil, nil)
	hiddenContentID := fx.seedContent(t, hiddenAuthorID, entity.VisibilityPublic, entity.StatusActive, true, nil, nil, base.Add(9*time.Minute), nil, nil)
	deletedStatusContentID := fx.seedContent(t, deletedStatusAuthorID, entity.VisibilityPublic, entity.StatusDeleted, false, nil, nil, base.Add(10*time.Minute), nil, nil)
	deletedAtContentID := fx.seedContent(t, deletedAtAuthorID, entity.VisibilityPublic, entity.StatusActive, false, &deletedAtTime, nil, base.Add(11*time.Minute), nil, nil)
	suspendedContentID := fx.seedContent(t, suspendedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(12*time.Minute), nil, nil)
	bannedContentID := fx.seedContent(t, bannedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(13*time.Minute), nil, nil)
	removedContentID := fx.seedContent(t, removedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(14*time.Minute), nil, nil)
	shareRefContentID := fx.seedContent(t, shareRefAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, &shareCaption, base.Add(15*time.Minute), shareReference, &shareRefOriginalAuthorID)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), publicContentID),
		uuid.New(): newContentOccurrence(uuid.New(), selfContentID),
		uuid.New(): newContentOccurrence(uuid.New(), followedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), notFollowedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), followedSelfContentID),
		uuid.New(): newContentOccurrence(uuid.New(), privateSelfContentID),
		uuid.New(): newContentOccurrence(uuid.New(), privateOtherContentID),
		uuid.New(): newContentOccurrence(uuid.New(), blockedByViewerContentID),
		uuid.New(): newContentOccurrence(uuid.New(), blocksViewerContentID),
		uuid.New(): newContentOccurrence(uuid.New(), hiddenContentID),
		uuid.New(): newContentOccurrence(uuid.New(), deletedStatusContentID),
		uuid.New(): newContentOccurrence(uuid.New(), deletedAtContentID),
		uuid.New(): newContentOccurrence(uuid.New(), suspendedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), bannedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), removedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), shareRefContentID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, len(occurrences))

	for msgID, occ := range occurrences {
		proj, ok := projections[msgID]
		require.True(t, ok, "missing projection for message %s", msgID)
		require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeContent, occ.ResourceType())

		switch occ.SourceID() {
		case publicContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Equal(t, "public caption", *payload.Caption)
			require.Len(t, payload.Media, 0)
			require.Equal(t, "active", payload.Lifecycle)
			require.Equal(t, base.Format(time.RFC3339), payload.CreatedAt)
			require.Equal(t, "public_author", payload.Author.Username)
			require.NotNil(t, payload.Author.AvatarURL)
			require.Equal(t, publicAvatar, *payload.Author.AvatarURL)
		case selfContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.Caption)
			require.Equal(t, "active", payload.Lifecycle)
			require.Equal(t, viewerID, payload.Author.ID)
			require.Equal(t, "viewer", payload.Author.Username)
		case followedContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Equal(t, "followers caption", *payload.Caption)
			require.Equal(t, "active", payload.Lifecycle)
			require.Equal(t, followedAuthorID, payload.Author.ID)
			require.Equal(t, "followed_author", payload.Author.Username)
		case notFollowedContentID:
			requireTombstoneContentProjection(t, proj)
		case followedSelfContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Equal(t, viewerID, payload.Author.ID)
			require.Equal(t, "viewer", payload.Author.Username)
		case privateSelfContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Equal(t, "private caption", *payload.Caption)
			require.Equal(t, viewerID, payload.Author.ID)
		case privateOtherContentID:
			requireTombstoneContentProjection(t, proj)
		case blockedByViewerContentID, blocksViewerContentID, hiddenContentID, deletedStatusContentID, deletedAtContentID, suspendedContentID, bannedContentID, removedContentID:
			requireTombstoneContentProjection(t, proj)
		case shareRefContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Equal(t, "share caption", *payload.Caption)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, shareRefTargetID)
			require.Equal(t, shareRefAuthorID, payload.Author.ID)
		default:
			t.Fatalf("unexpected content id %s", occ.SourceID())
		}
	}
}

func TestContentProjectionResolver_NestedShareReferences_DepthOneAndAccessRules(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerAvatar := "https://cdn.example.test/viewer.png"
	viewerID := fx.seedUser(t, "active", nil, "viewer", &viewerAvatar)

	primaryAuthorID := fx.seedUser(t, "active", nil, "primary_author", nil)
	base := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)

	nestedLeafMissingID := uuid.New()
	_, err := fx.appDB.Pool().Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption,
			visibility, is_hidden, original_author_id,
			created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, NULL)
	`, nestedLeafMissingID, primaryAuthorID, string(entity.StatusActive), "nested leaf target", string(entity.VisibilityPublic), false, nil, base)
	require.NoError(t, err)
	nestedLeafShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"content","targetId":"%s","preview":{"title":"stale nested leaf","imageUrl":"https://cdn.example.test/stale-nested.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		nestedLeafMissingID,
	))

	nestedContentAuthorID := fx.seedUser(t, "active", nil, "nested_content_author", nil)
	nestedContentTargetID := fx.seedContent(
		t,
		nestedContentAuthorID,
		entity.VisibilityPublic,
		entity.StatusActive,
		false,
		nil,
		nil,
		base.Add(1*time.Minute),
		nestedLeafShareRef,
		nil,
	)

	profileTargetID := fx.seedUser(t, "active", nil, "profile_target", nil)
	blockedProfileTargetID := fx.seedUser(t, "active", nil, "blocked_profile_target", nil)
	fx.seedBlock(t, viewerID, blockedProfileTargetID)

	saleOwnerID := fx.seedUser(t, "active", nil, "sale_owner", nil)
	publicPublishedAt := base.Add(2 * time.Minute)
	publicSaleID := fx.seedForSale(t, saleOwnerID, fpsEntity.ForSaleStatusActive, &publicPublishedAt)

	privateSaleOwnerID := fx.seedUser(t, "active", nil, "private_sale_owner", nil)
	privateSaleID := fx.seedForSale(t, privateSaleOwnerID, fpsEntity.ForSaleStatusDraft, nil)

	auctionOwnerID := fx.seedUser(t, "active", nil, "auction_owner", nil)
	auctionID := fx.seedAuction(t, auctionOwnerID, auctionEntity.StatusActive)

	missingTargetID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusDeleted, false, nil, nil, base.Add(9*time.Minute), nil, nil)
	publicSaleShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"for_sale","targetId":"%s","preview":{"title":"Public Sale","imageUrl":"https://cdn.example.test/public-sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		publicSaleID,
	))
	privateSaleShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"for_sale","targetId":"%s","preview":{"title":"Private Sale Preview","imageUrl":"https://cdn.example.test/private-sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		privateSaleID,
	))
	profileShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"profile","targetId":"%s","preview":{"title":"Profile Target","imageUrl":"https://cdn.example.test/profile.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		profileTargetID,
	))
	blockedProfileShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"profile","targetId":"%s","preview":{"title":"Blocked Profile","imageUrl":"https://cdn.example.test/blocked-profile.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		blockedProfileTargetID,
	))
	auctionShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"auction","targetId":"%s","preview":{"title":"Auction Target","imageUrl":"https://cdn.example.test/auction.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		auctionID,
	))
	contentShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"content","targetId":"%s","preview":{"title":"Nested Content","imageUrl":"https://cdn.example.test/content.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		nestedContentTargetID,
	))
	missingShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"content","targetId":"%s","preview":{"title":"Missing Target","imageUrl":"https://cdn.example.test/missing.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		missingTargetID,
	))

	nestedPrimaryContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(3*time.Minute), contentShareRef, nil)
	profileAllowedContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(4*time.Minute), profileShareRef, nil)
	profileBlockedContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(5*time.Minute), blockedProfileShareRef, nil)
	publicSaleContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(6*time.Minute), publicSaleShareRef, nil)
	privateSaleContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(7*time.Minute), privateSaleShareRef, nil)
	auctionContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(8*time.Minute), auctionShareRef, nil)
	missingTargetContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(10*time.Minute), missingShareRef, nil)

	deletedAt := base.Add(11 * time.Minute)
	tombstonePrimaryContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, &deletedAt, nil, base.Add(11*time.Minute), publicSaleShareRef, nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), nestedPrimaryContentID),
		uuid.New(): newContentOccurrence(uuid.New(), profileAllowedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), profileBlockedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), publicSaleContentID),
		uuid.New(): newContentOccurrence(uuid.New(), privateSaleContentID),
		uuid.New(): newContentOccurrence(uuid.New(), auctionContentID),
		uuid.New(): newContentOccurrence(uuid.New(), missingTargetContentID),
		uuid.New(): newContentOccurrence(uuid.New(), tombstonePrimaryContentID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, len(occurrences))

	for msgID, occ := range occurrences {
		proj := projections[msgID]
		switch occ.SourceID() {
		case nestedPrimaryContentID:
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, nestedContentTargetID)
		case profileAllowedContentID:
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeProfile, profileTargetID)
		case profileBlockedContentID, privateSaleContentID, missingTargetContentID:
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		case publicSaleContentID:
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeForSale, publicSaleID)
		case auctionContentID:
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
		case tombstonePrimaryContentID:
			requireTombstoneContentProjection(t, proj)
		default:
			t.Fatalf("unexpected content id %s", occ.SourceID())
		}
	}
}

func TestContentProjectionResolver_NestedShareReferences_DoNotRecursePastDepthOne(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	primaryAuthorID := fx.seedUser(t, "active", nil, "primary_author", nil)
	nestedAuthorID := fx.seedUser(t, "active", nil, "nested_author", nil)
	base := time.Date(2026, time.August, 8, 13, 0, 0, 0, time.UTC)

	targetWithoutNestedID := fx.seedContent(t, nestedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(1*time.Minute), nil, nil)
	missingDeepTargetID := uuid.New()
	_, err := fx.appDB.Pool().Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption,
			visibility, is_hidden, original_author_id,
			created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, NULL)
	`, missingDeepTargetID, nestedAuthorID, string(entity.StatusActive), "deep target leaf", string(entity.VisibilityPublic), false, nil, base.Add(1*time.Minute))
	require.NoError(t, err)
	targetWithNestedShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"content","targetId":"%s","preview":{"title":"deep target","imageUrl":"https://cdn.example.test/deep-target.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		missingDeepTargetID,
	))
	targetWithNestedID := fx.seedContent(t, nestedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(2*time.Minute), targetWithNestedShareRef, nil)

	primaryWithoutNestedShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"content","targetId":"%s","preview":{"title":"shallow primary","imageUrl":"https://cdn.example.test/shallow-primary.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		targetWithoutNestedID,
	))
	primaryWithNestedShareRef := json.RawMessage(fmt.Sprintf(
		`{"targetType":"content","targetId":"%s","preview":{"title":"deep primary","imageUrl":"https://cdn.example.test/deep-primary.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
		targetWithNestedID,
	))

	primaryWithoutNestedID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(3*time.Minute), primaryWithoutNestedShareRef, nil)
	primaryWithNestedID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(4*time.Minute), primaryWithNestedShareRef, nil)

	shallowOccurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), primaryWithoutNestedID),
	}
	deepOccurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), primaryWithNestedID),
	}

	fx.tracer.reset()
	shallowProjections, err := fx.resolve(ctx, viewerID, shallowOccurrences)
	require.NoError(t, err)
	shallowCount := fx.tracer.value()
	require.Len(t, shallowProjections, len(shallowOccurrences))
	for _, proj := range shallowProjections {
		payload := requireLiveContentProjection(t, proj)
		requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, targetWithoutNestedID)
	}

	fx.tracer.reset()
	deepProjections, err := fx.resolve(ctx, viewerID, deepOccurrences)
	require.NoError(t, err)
	deepCount := fx.tracer.value()
	require.Len(t, deepProjections, len(deepOccurrences))
	for _, proj := range deepProjections {
		payload := requireLiveContentProjection(t, proj)
		requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, targetWithNestedID)
	}

	require.Equal(t, shallowCount, deepCount)
	t.Logf("query counts: shallow=%d deep=%d", shallowCount, deepCount)
}

func TestContentProjectionResolver_MediaOrderAndEmptyMedia(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	authorID := fx.seedUser(t, "active", nil, "media_author", nil)

	orderedCaption := "ordered media"
	orderedCreatedAt := time.Date(2026, time.August, 8, 11, 0, 0, 0, time.UTC)
	orderedContentID := fx.seedContent(t, authorID, entity.VisibilityPublic, entity.StatusActive, false, nil, &orderedCaption, orderedCreatedAt, nil, nil)
	fx.seedMedia(t, orderedContentID, 3, "https://cdn.example.test/media-3.jpg", "video")
	fx.seedMedia(t, orderedContentID, 1, "https://cdn.example.test/media-1.jpg", "image")
	fx.seedMedia(t, orderedContentID, 2, "https://cdn.example.test/media-2.jpg", "image")

	emptyContentID := fx.seedContent(t, authorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, orderedCreatedAt.Add(1*time.Minute), nil, nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), orderedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), emptyContentID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, len(occurrences))

	for _, proj := range projections {
		payload := requireLiveContentProjection(t, proj)
		switch proj.Identity.ResourceID {
		case orderedContentID:
			require.Equal(t, 3, len(payload.Media))
			require.Equal(t, "https://cdn.example.test/media-1.jpg", payload.Media[0].URL)
			require.Equal(t, "https://cdn.example.test/media-2.jpg", payload.Media[1].URL)
			require.Equal(t, "https://cdn.example.test/media-3.jpg", payload.Media[2].URL)
		case emptyContentID:
			require.NotNil(t, payload.Media)
			require.Empty(t, payload.Media)
		default:
			t.Fatalf("unexpected projection identity %s", proj.Identity.ResourceID)
		}
	}
}

func TestContentProjectionResolver_DedupesRepeatedContentAcrossMessages(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	authorID := fx.seedUser(t, "active", nil, "dedupe_author", nil)
	contentID := fx.seedContent(t, authorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, time.Now().UTC(), nil, nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), contentID),
		uuid.New(): newContentOccurrence(uuid.New(), contentID),
		uuid.New(): newContentOccurrence(uuid.New(), contentID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, len(occurrences))

	for _, proj := range projections {
		payload := requireLiveContentProjection(t, proj)
		require.Equal(t, contentID, proj.Identity.ResourceID)
		require.Equal(t, "dedupe_author", payload.Author.Username)
	}
}

func TestContentProjectionResolver_MixedBatch_NoCrossLeakage(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	publicAuthorID := fx.seedUser(t, "active", nil, "public", nil)
	followedAuthorID := fx.seedUser(t, "active", nil, "followed", nil)
	fx.seedFollow(t, viewerID, followedAuthorID)
	privateAuthorID := fx.seedUser(t, "active", nil, "private", nil)
	hiddenAuthorID := fx.seedUser(t, "active", nil, "hidden", nil)
	blockedAuthorID := fx.seedUser(t, "active", nil, "blocked", nil)
	fx.seedBlock(t, viewerID, blockedAuthorID)

	publicContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, time.Now().UTC(), nil, nil)
	followedContentID := fx.seedContent(t, followedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(1*time.Minute), nil, nil)
	privateContentID := fx.seedContent(t, privateAuthorID, entity.VisibilityPrivate, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(2*time.Minute), nil, nil)
	hiddenContentID := fx.seedContent(t, hiddenAuthorID, entity.VisibilityPublic, entity.StatusActive, true, nil, nil, time.Now().UTC().Add(3*time.Minute), nil, nil)
	blockedContentID := fx.seedContent(t, blockedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(4*time.Minute), nil, nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), publicContentID),
		uuid.New(): newContentOccurrence(uuid.New(), followedContentID),
		uuid.New(): newContentOccurrence(uuid.New(), privateContentID),
		uuid.New(): newContentOccurrence(uuid.New(), hiddenContentID),
		uuid.New(): newContentOccurrence(uuid.New(), blockedContentID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, len(occurrences))

	for msgID, proj := range projections {
		sourceID := occurrences[msgID].SourceID()
		switch sourceID {
		case publicContentID:
			requireLiveContentProjection(t, proj)
		case followedContentID:
			requireLiveContentProjection(t, proj)
		case privateContentID, hiddenContentID, blockedContentID:
			requireTombstoneContentProjection(t, proj)
		default:
			t.Fatalf("unexpected content %s", proj.Identity.ResourceID)
		}
	}
}

func TestContentProjectionResolver_QueryCount_BoundedByDistinctContents(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	sharedAuthorID := fx.seedUser(t, "active", nil, "shared", nil)
	sharedContentID := fx.seedContent(t, sharedAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, time.Now().UTC(), nil, nil)

	one := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), sharedContentID),
	}

	twentySame := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		twentySame[uuid.New()] = newContentOccurrence(uuid.New(), sharedContentID)
	}

	twentyDistinct := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		authorID := fx.seedUser(t, "active", nil, fmt.Sprintf("author-%02d", i), nil)
		contentID := fx.seedContent(t, authorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(time.Duration(i)*time.Minute), nil, nil)
		twentyDistinct[uuid.New()] = newContentOccurrence(uuid.New(), contentID)
	}

	twentyFollowersOnly := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		authorID := fx.seedUser(t, "active", nil, fmt.Sprintf("followers-%02d", i), nil)
		fx.seedFollow(t, viewerID, authorID)
		contentID := fx.seedContent(t, authorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(time.Duration(i+100)*time.Minute), nil, nil)
		twentyFollowersOnly[uuid.New()] = newContentOccurrence(uuid.New(), contentID)
	}

	mixed := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), sharedContentID),
	}
	for i := 0; i < 4; i++ {
		authorID := fx.seedUser(t, "active", nil, fmt.Sprintf("mixed-%02d", i), nil)
		visibility := entity.VisibilityPublic
		if i%2 == 0 {
			visibility = entity.VisibilityFollowersOnly
			fx.seedFollow(t, viewerID, authorID)
		}
		contentID := fx.seedContent(t, authorID, visibility, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(time.Duration(i+200)*time.Minute), nil, nil)
		mixed[uuid.New()] = newContentOccurrence(uuid.New(), contentID)
	}

	fx.tracer.reset()
	_, err := fx.resolve(ctx, viewerID, one)
	require.NoError(t, err)
	countOne := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, twentySame)
	require.NoError(t, err)
	countSame := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, twentyDistinct)
	require.NoError(t, err)
	countDistinct := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, twentyFollowersOnly)
	require.NoError(t, err)
	countFollowers := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, mixed)
	require.NoError(t, err)
	countMixed := fx.tracer.value()

	t.Logf("query counts: one=%d same=%d distinct=%d followers_only=%d mixed=%d", countOne, countSame, countDistinct, countFollowers, countMixed)
	require.Equal(t, countOne, countSame)
	require.Equal(t, countOne, countDistinct)
	require.Equal(t, countFollowers, countMixed)
	require.GreaterOrEqual(t, countFollowers, countOne)
}

type contentProjectionFailingDB struct {
	base        *db.DB
	failOnQuery int
	err         error
}

func (d *contentProjectionFailingDB) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	return d.base.WithTx(ctx, func(tx db.Tx) error {
		return fn(&contentProjectionFailingTx{
			Tx:          tx,
			failOnQuery: d.failOnQuery,
			err:         d.err,
		})
	})
}

type contentProjectionFailingTx struct {
	db.Tx
	failOnQuery int
	queryCount  int
	err         error
}

func (t *contentProjectionFailingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.queryCount++
	if t.queryCount == t.failOnQuery {
		return nil, t.err
	}
	return t.Tx.Query(ctx, sql, args...)
}

var _ contentProjectionDB = (*contentProjectionFailingDB)(nil)

func TestContentProjectionResolver_QueryFailures_Propagate(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	targetAuthorID := fx.seedUser(t, "active", nil, "target", nil)
	followedAuthorID := fx.seedUser(t, "active", nil, "followed", nil)
	fx.seedFollow(t, viewerID, followedAuthorID)
	targetContentID := fx.seedContent(t, targetAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, time.Now().UTC(), nil, nil)
	followedContentID := fx.seedContent(t, followedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, time.Now().UTC().Add(1*time.Minute), nil, nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), targetContentID),
		uuid.New(): newContentOccurrence(uuid.New(), followedContentID),
	}

	t.Run("content source query failure", func(t *testing.T) {
		wantErr := errors.New("source query boom")
		resolver := newContentProjectionBatchResolver(&contentProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 1,
			err:         wantErr,
		})

		projections, err := resolver.ResolveContents(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "content source batch query failed")
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("author query failure", func(t *testing.T) {
		wantErr := errors.New("author query boom")
		resolver := newContentProjectionBatchResolver(&contentProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 3,
			err:         wantErr,
		})

		projections, err := resolver.ResolveContents(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "content author batch query failed")
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("block query failure", func(t *testing.T) {
		wantErr := errors.New("block query boom")
		resolver := newContentProjectionBatchResolver(&contentProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 4,
			err:         wantErr,
		})

		projections, err := resolver.ResolveContents(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "content block batch query failed")
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("follow query failure", func(t *testing.T) {
		wantErr := errors.New("follow query boom")
		resolver := newContentProjectionBatchResolver(&contentProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 5,
			err:         wantErr,
		})

		projections, err := resolver.ResolveContents(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "content follow batch query failed")
		require.ErrorIs(t, err, wantErr)
	})
}

func TestContentProjectionResolver_MissingSourceRow_IsIntegrityError(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, "viewer", nil)
	missingContentID := uuid.New()
	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), missingContentID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.Error(t, err)
	require.Nil(t, projections)
	assert.Contains(t, err.Error(), "content source row missing")
}
