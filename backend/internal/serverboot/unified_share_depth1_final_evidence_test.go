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
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

func uniqueSeedLabel(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.NewString())
}

func TestUnifiedShareDepth1_QueryCountsAndSuppressionEvidence(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("viewer"), nil)
	primaryAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("primary_author"), nil)
	base := time.Date(2026, time.August, 8, 14, 0, 0, 0, time.UTC)

	// Q1/Q2: one vs twenty live contents without nested share_reference.
	plainContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base, nil, nil)
	one := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newContentOccurrence(uuid.New(), plainContentID),
	}
	twentySame := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		twentySame[uuid.New()] = newContentOccurrence(uuid.New(), plainContentID)
	}

	// Q3/Q4: same vs distinct nested Profiles.
	profileTargetID := fx.seedUser(t, "active", nil, uniqueSeedLabel("profile_target"), nil)
	sameProfileContents := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	distinctProfileContents := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		shareRef := json.RawMessage(fmt.Sprintf(
			`{"targetType":"profile","targetId":"%s","preview":{"title":"profile-%d","imageUrl":"https://cdn.example.test/profile-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			profileTargetID, i, i,
		))
		contentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+1)*time.Minute), shareRef, nil)
		sameProfileContents[uuid.New()] = newContentOccurrence(uuid.New(), contentID)

		distinctProfileID := fx.seedUser(t, "active", nil, uniqueSeedLabel(fmt.Sprintf("profile_target_%02d", i)), nil)
		distinctShareRef := json.RawMessage(fmt.Sprintf(
			`{"targetType":"profile","targetId":"%s","preview":{"title":"profile-distinct-%d","imageUrl":"https://cdn.example.test/profile-distinct-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			distinctProfileID, i, i,
		))
		distinctContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+21)*time.Minute), distinctShareRef, nil)
		distinctProfileContents[uuid.New()] = newContentOccurrence(uuid.New(), distinctContentID)
	}

	// Q5/Q6: same vs distinct nested Contents.
	nestedContentAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("nested_content_author"), nil)
	nestedContentTargetID := fx.seedContent(t, nestedContentAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(100*time.Minute), nil, nil)
	sameContentContents := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	distinctContentContents := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		shareRef := json.RawMessage(fmt.Sprintf(
			`{"targetType":"content","targetId":"%s","preview":{"title":"content-%d","imageUrl":"https://cdn.example.test/content-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			nestedContentTargetID, i, i,
		))
		contentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+200)*time.Minute), shareRef, nil)
		sameContentContents[uuid.New()] = newContentOccurrence(uuid.New(), contentID)

		distinctNestedContentAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel(fmt.Sprintf("distinct_nested_content_author_%02d", i)), nil)
		distinctNestedContentID := fx.seedContent(t, distinctNestedContentAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+300)*time.Minute), nil, nil)
		distinctShareRef := json.RawMessage(fmt.Sprintf(
			`{"targetType":"content","targetId":"%s","preview":{"title":"content-distinct-%d","imageUrl":"https://cdn.example.test/content-distinct-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			distinctNestedContentID, i, i,
		))
		distinctContentID := fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+320)*time.Minute), distinctShareRef, nil)
		distinctContentContents[uuid.New()] = newContentOccurrence(uuid.New(), distinctContentID)
	}

	// Q7/Q8: mixed all four nested resource types.
	saleOwnerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("sale_owner"), nil)
	publicPublishedAt := base.Add(400 * time.Minute)
	publicSaleID := fx.seedForSale(t, saleOwnerID, fpsEntity.ForSaleStatusActive, &publicPublishedAt)
	draftSaleOwnerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("draft_sale_owner"), nil)
	draftPublishedAt := base.Add(401 * time.Minute)
	draftSaleID := fx.seedForSale(t, draftSaleOwnerID, fpsEntity.ForSaleStatusDraft, &draftPublishedAt)
	auctionOwnerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("auction_owner"), nil)
	activeAuctionID := fx.seedAuction(t, auctionOwnerID, auctionEntity.StatusActive)
	draftAuctionOwnerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("draft_auction_owner"), nil)
	draftAuctionID := fx.seedAuction(t, draftAuctionOwnerID, auctionEntity.StatusDraft)

	mixedNested := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{}
	repeatedNested := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{}
	for i := 0; i < 5; i++ {
		mixedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+500)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"profile","targetId":"%s","preview":{"title":"mix-profile-%d","imageUrl":"https://cdn.example.test/mix-profile-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			profileTargetID, i, i,
		)), nil))
		mixedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+510)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"content","targetId":"%s","preview":{"title":"mix-content-%d","imageUrl":"https://cdn.example.test/mix-content-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			nestedContentTargetID, i, i,
		)), nil))
		mixedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+520)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"for_sale","targetId":"%s","preview":{"title":"mix-sale-%d","imageUrl":"https://cdn.example.test/mix-sale-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			publicSaleID, i, i,
		)), nil))
		mixedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+530)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"auction","targetId":"%s","preview":{"title":"mix-auction-%d","imageUrl":"https://cdn.example.test/mix-auction-%d.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			activeAuctionID, i, i,
		)), nil))

		repeatedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+600)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"profile","targetId":"%s","preview":{"title":"repeat-profile","imageUrl":"https://cdn.example.test/repeat-profile.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			profileTargetID,
		)), nil))
		repeatedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+610)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"content","targetId":"%s","preview":{"title":"repeat-content","imageUrl":"https://cdn.example.test/repeat-content.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			nestedContentTargetID,
		)), nil))
		repeatedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+620)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"for_sale","targetId":"%s","preview":{"title":"repeat-sale","imageUrl":"https://cdn.example.test/repeat-sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			draftSaleID,
		)), nil))
		repeatedNested[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, primaryAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(i+630)*time.Minute), json.RawMessage(fmt.Sprintf(
			`{"targetType":"auction","targetId":"%s","preview":{"title":"repeat-auction","imageUrl":"https://cdn.example.test/repeat-auction.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}`,
			draftAuctionID,
		)), nil))
	}

	measure := func(name string, occ map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) int64 {
		t.Helper()
		fx.tracer.reset()
		_, err := fx.resolve(ctx, viewerID, occ)
		require.NoError(t, err, name)
		return fx.tracer.value()
	}

	countQ1 := measure("Q1", one)
	countQ2 := measure("Q2", twentySame)
	countQ3 := measure("Q3", sameProfileContents)
	countQ4 := measure("Q4", distinctProfileContents)
	countQ5 := measure("Q5", sameContentContents)
	countQ6 := measure("Q6", distinctContentContents)
	countQ7 := measure("Q7", mixedNested)
	countQ8 := measure("Q8", repeatedNested)

	t.Logf("Q1 1 LIVE Content, no nested = %d", countQ1)
	t.Logf("Q2 20 LIVE Contents, no nested = %d", countQ2)
	t.Logf("Q3 20 Contents -> SAME nested Profile = %d", countQ3)
	t.Logf("Q4 20 Contents -> 20 DISTINCT nested Profiles = %d", countQ4)
	t.Logf("Q5 20 Contents -> SAME nested Content = %d", countQ5)
	t.Logf("Q6 20 Contents -> 20 DISTINCT nested Contents = %d", countQ6)
	t.Logf("Q7 mixed nested Profile + Content + FPS + Auction = %d", countQ7)
	t.Logf("Q8 many repeated references across all four types = %d", countQ8)

	require.Equal(t, countQ1, countQ2)
	require.Equal(t, countQ3, countQ4)
	require.Equal(t, countQ5, countQ6)
}

func TestUnifiedShareDepth1_NestedFailurePropagationEvidence(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("viewer"), nil)
	authorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("author"), nil)
	blockedAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("blocked_author"), nil)
	fx.seedBlock(t, viewerID, blockedAuthorID)
	followedAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("followed_author"), nil)
	fx.seedFollow(t, viewerID, followedAuthorID)

	profileID := fx.seedUser(t, "active", nil, uniqueSeedLabel("profile_target"), nil)
	var err error

	publicCaption := "public target"
	publicContentID := fx.seedContent(t, authorID, entity.VisibilityPublic, entity.StatusActive, false, nil, &publicCaption, time.Now().UTC(), nil, nil)
	privateCaption := "private target"
	privateContentID := fx.seedContent(t, followedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, &privateCaption, time.Now().UTC(), nil, nil)

	productID := fx.seedProduct(t, authorID, "fixture fps product", "fixture fps product")
	publicPublishedAt := time.Now().UTC()
	publicSaleID := uuid.New()
	_, err = fx.appDB.Pool().Exec(context.Background(), `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, false, $5, $6, NULL, NULL, 1, $7, $7)
	`, publicSaleID, productID, authorID, int64(100000), string(fpsEntity.ForSaleStatusActive), publicPublishedAt, publicPublishedAt)
	require.NoError(t, err)

	draftProductID := fx.seedProduct(t, authorID, "fixture draft fps product", "fixture draft fps product")
	draftSaleID := uuid.New()
	_, err = fx.appDB.Pool().Exec(context.Background(), `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, false, $5, NULL, NULL, NULL, 1, $6, $6)
	`, draftSaleID, draftProductID, authorID, int64(100000), string(fpsEntity.ForSaleStatusDraft), time.Now().UTC())
	require.NoError(t, err)

	activeAuctionID := fx.seedAuction(t, authorID, auctionEntity.StatusActive)
	failingCall := func(failOnQuery int, fn func(tx db.Tx) error) error {
		return fx.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fn(&contentProjectionFailingTx{
				Tx:          tx,
				failOnQuery: failOnQuery,
				err:         errors.New("boom"),
			})
		})
	}

	t.Run("profile source query failure", func(t *testing.T) {
		err := failingCall(1, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedProfileIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				profileID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested profile source batch query failed")
	})

	t.Run("profile block query failure", func(t *testing.T) {
		err := failingCall(2, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedProfileIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				profileID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested profile block batch query failed")
	})

	t.Run("content source query failure", func(t *testing.T) {
		err := failingCall(1, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedContentIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				publicContentID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested content source batch query failed")
	})

	t.Run("content block query failure", func(t *testing.T) {
		err := failingCall(3, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedContentIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				publicContentID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested content block batch query failed")
	})

	t.Run("content follow query failure", func(t *testing.T) {
		err := failingCall(4, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedContentIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				privateContentID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested content follow batch query failed")
	})

	t.Run("fixed price sale source query failure", func(t *testing.T) {
		err := failingCall(1, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedForSaleIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				publicSaleID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested fixed price sale source batch query failed")
	})

	t.Run("fixed price sale block query failure", func(t *testing.T) {
		err := failingCall(2, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedForSaleIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				publicSaleID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested fixed price sale block batch query failed")
	})

	t.Run("auction source query failure", func(t *testing.T) {
		err := failingCall(1, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedAuctionIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				activeAuctionID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested auction source batch query failed")
	})

	t.Run("auction block query failure", func(t *testing.T) {
		err := failingCall(2, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedAuctionIndicators(ctx, tx, viewerID, map[uuid.UUID][]uuid.UUID{
				activeAuctionID: []uuid.UUID{uuid.New()},
			})
			return err
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested auction block batch query failed")
	})

	t.Run("malformed share target type errors", func(t *testing.T) {
		_, ok, err := parseNestedShareReference([]byte(`{"targetType":"broken_type","targetId":"` + uuid.NewString() + `"}`))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("nil share target uuid errors", func(t *testing.T) {
		_, ok, err := parseNestedShareReference([]byte(`{"targetType":"profile","targetId":"00000000-0000-0000-0000-000000000000"}`))
		require.Error(t, err)
		require.False(t, ok)
	})
}

func seedSellerAccessUser(t *testing.T, fx *contentProjectionFixture, accountStatus string, deletedAt *time.Time, subscriptionStatus string) uuid.UUID {
	t.Helper()

	sellerID := fx.seedUser(t, accountStatus, deletedAt, uniqueSeedLabel("seller"), nil)
	if subscriptionStatus == "" {
		return sellerID
	}

	now := time.Now().UTC()
	expiresAt := now.Add(365 * 24 * time.Hour)
	if subscriptionStatus != "active" {
		expiresAt = now.Add(-24 * time.Hour)
	}
	_, err := fx.appDB.Pool().Exec(context.Background(), `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at, duration_days,
			amount_paid, currency, payment_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 0, 'IDR', $7, $8, $8)
	`, uuid.New(), sellerID, subscriptionStatus, now.Add(-24*time.Hour), expiresAt, 365, uuid.New(), now)
	require.NoError(t, err)

	return sellerID
}

func sharePreview(target string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"title":"%s","imageUrl":"https://cdn.example.test/%s.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}`,
		target, target,
	))
}

func shareRef(targetType string, targetID uuid.UUID, previewTitle string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"targetType":"%s","targetId":"%s","preview":%s}`,
		targetType, targetID, sharePreview(previewTitle),
	))
}

func parseNestedShareReference(raw []byte) (map[string]any, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false, err
	}

	targetType, _ := payload["targetType"].(string)
	switch targetType {
	case "profile", "content", "for_sale", "auction":
	default:
		return payload, false, fmt.Errorf("unsupported nested share target type: %s", targetType)
	}

	targetID, _ := payload["targetId"].(string)
	parsedID, err := uuid.Parse(targetID)
	if err != nil || parsedID == uuid.Nil {
		return payload, false, fmt.Errorf("nested share target id must be a valid non-zero UUID")
	}

	return payload, true, nil
}

func TestUnifiedShareDepth1_DepthMatrixD1ToD30Evidence(t *testing.T) {
	fx := newContentProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, uniqueSeedLabel("viewer"), nil)
	base := time.Date(2026, time.August, 8, 16, 0, 0, 0, time.UTC)

	// D1-D3: profile targets.
	profileActiveID := fx.seedUser(t, "active", nil, uniqueSeedLabel("profile_active"), nil)
	profileBlockedID := fx.seedUser(t, "active", nil, uniqueSeedLabel("profile_blocked"), nil)
	fx.seedBlock(t, viewerID, profileBlockedID)
	suspendedProfileID := fx.seedUser(t, "suspended", nil, uniqueSeedLabel("profile_suspended"), nil)
	bannedProfileID := fx.seedUser(t, "banned", nil, uniqueSeedLabel("profile_banned"), nil)
	removedAt := base.Add(-time.Hour)
	removedProfileID := fx.seedUser(t, "active", &removedAt, uniqueSeedLabel("profile_removed"), nil)

	// D4-D9: content targets.
	publicAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_public"), nil)
	publicContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(1*time.Minute), nil, nil)

	privateAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_private"), nil)
	privateContentID := fx.seedContent(t, privateAuthorID, entity.VisibilityPrivate, entity.StatusActive, false, nil, nil, base.Add(2*time.Minute), nil, nil)

	followedAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_followed"), nil)
	fx.seedFollow(t, viewerID, followedAuthorID)
	followersOnlyContentID := fx.seedContent(t, followedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, base.Add(3*time.Minute), nil, nil)

	notFollowedAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_not_followed"), nil)
	notFollowedContentID := fx.seedContent(t, notFollowedAuthorID, entity.VisibilityFollowersOnly, entity.StatusActive, false, nil, nil, base.Add(4*time.Minute), nil, nil)

	blockedContentAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_blocked"), nil)
	fx.seedBlock(t, viewerID, blockedContentAuthorID)
	blockedContentID := fx.seedContent(t, blockedContentAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(5*time.Minute), nil, nil)

	hiddenContentAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_hidden"), nil)
	hiddenContentID := fx.seedContent(t, hiddenContentAuthorID, entity.VisibilityPublic, entity.StatusActive, true, nil, nil, base.Add(6*time.Minute), nil, nil)

	deletedStatusAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_deleted"), nil)
	deletedStatusContentID := fx.seedContent(t, deletedStatusAuthorID, entity.VisibilityPublic, entity.StatusDeleted, false, nil, nil, base.Add(7*time.Minute), nil, nil)

	deletedAtContentAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_deleted_at"), nil)
	deletedAtTime := base.Add(8 * time.Minute)
	deletedAtContentID := fx.seedContent(t, deletedAtContentAuthorID, entity.VisibilityPublic, entity.StatusActive, false, &deletedAtTime, nil, base.Add(8*time.Minute), nil, nil)

	nestedLeafAuthorID := fx.seedUser(t, "active", nil, uniqueSeedLabel("content_depth_leaf"), nil)
	nestedLeafID := fx.seedContent(t, nestedLeafAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(9*time.Minute), nil, nil)
	nestedMidShareRef := shareRef("content", nestedLeafID, "depth-mid")
	fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(10*time.Minute), nestedMidShareRef, nil)

	// D10-D14: fixed-price sales.
	fpsOwnerID := seedSellerAccessUser(t, fx, "active", nil, "active")
	fpsActivePublishedAt := base.Add(11 * time.Minute)
	fpsActiveID := fx.seedForSale(t, fpsOwnerID, fpsEntity.ForSaleStatusActive, &fpsActivePublishedAt)

	fpsDraftOwnerID := seedSellerAccessUser(t, fx, "active", nil, "active")
	fpsDraftID := fx.seedForSale(t, fpsDraftOwnerID, fpsEntity.ForSaleStatusDraft, nil)

	fpsRemovedOwnerID := seedSellerAccessUser(t, fx, "suspended", nil, "active")
	fpsRemovedPublishedAt := base.Add(12 * time.Minute)
	fpsRemovedID := fx.seedForSale(t, fpsRemovedOwnerID, fpsEntity.ForSaleStatusActive, &fpsRemovedPublishedAt)

	fpsTerminalOwnerID := seedSellerAccessUser(t, fx, "active", nil, "active")
	fpsTerminalPublishedAt := base.Add(13 * time.Minute)
	fpsTerminalID := fx.seedForSale(t, fpsTerminalOwnerID, fpsEntity.ForSaleStatusWithdrawn, &fpsTerminalPublishedAt)

	// D15-D19: auctions.
	auctionActiveOwnerID := seedSellerAccessUser(t, fx, "active", nil, "active")
	auctionActiveID := fx.seedAuction(t, auctionActiveOwnerID, auctionEntity.StatusActive)

	auctionDraftOwnerID := seedSellerAccessUser(t, fx, "active", nil, "active")
	auctionDraftID := fx.seedAuction(t, auctionDraftOwnerID, auctionEntity.StatusDraft)

	auctionRemovedOwnerID := seedSellerAccessUser(t, fx, "suspended", nil, "active")
	auctionRemovedID := fx.seedAuction(t, auctionRemovedOwnerID, auctionEntity.StatusActive)

	auctionTerminalOwnerID := seedSellerAccessUser(t, fx, "active", nil, "active")
	auctionTerminalID := fx.seedAuction(t, auctionTerminalOwnerID, auctionEntity.StatusEnded)

	// D20-D22: missing/malformed target contracts.
	missingTargetID := uuid.New()
	missingTargetContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(14*time.Minute), shareRef("content", missingTargetID, "missing"), nil)
	malformedTargetTypeContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(15*time.Minute), json.RawMessage(fmt.Sprintf(`{"targetType":"broken_type","targetId":"%s"}`, uuid.NewString())), nil)
	nilTargetUUIDContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(16*time.Minute), json.RawMessage(`{"targetType":"profile","targetId":"00000000-0000-0000-0000-000000000000"}`), nil)

	// D26-D29 shared targets.
	mixedProfileShareRef := shareRef("profile", profileActiveID, "profile-preview")
	mixedContentShareRef := shareRef("content", publicContentID, "content-preview")
	mixedSaleShareRef := shareRef("for_sale", fpsActiveID, "sale-preview")
	mixedAuctionShareRef := shareRef("auction", auctionActiveID, "auction-preview")
	mixedProfileContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(17*time.Minute), mixedProfileShareRef, nil)
	mixedContentContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(18*time.Minute), mixedContentShareRef, nil)
	mixedSaleContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(19*time.Minute), mixedSaleShareRef, nil)
	mixedAuctionContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(20*time.Minute), mixedAuctionShareRef, nil)

	repeatedSameTarget := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		repeatedSameTarget[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(21+i)*time.Minute), mixedProfileShareRef, nil))
	}

	depth2ShallowTargetID := fx.seedContent(t, nestedLeafAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(51*time.Minute), nil, nil)
	depth2NestedTargetShareRef := shareRef("content", depth2ShallowTargetID, "depth2-nested")
	depth2NestedTargetID := fx.seedContent(t, nestedLeafAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(52*time.Minute), depth2NestedTargetShareRef, nil)
	depth2PrimaryTargetShareRef := shareRef("content", depth2NestedTargetID, "depth2-primary")
	depth2PrimaryTargetID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(53*time.Minute), depth2PrimaryTargetShareRef, nil)

	t.Run("D1 accessible Profile -> indicator", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(60*time.Minute), shareRef("profile", profileActiveID, "profile-ok"), nil)),
		})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeProfile, profileActiveID)
		}
	})

	t.Run("D2 blocked Profile -> suppress", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(61*time.Minute), shareRef("profile", profileBlockedID, "profile-blocked"), nil)),
		})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D3 suspended/banned/deleted Profile -> suppress", func(t *testing.T) {
		cases := []uuid.UUID{suspendedProfileID, bannedProfileID, removedProfileID}
		for _, targetID := range cases {
			projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
				uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(62*time.Minute), shareRef("profile", targetID, "profile-inactive"), nil)),
			})
			require.NoError(t, err)
			for _, proj := range projections {
				payload := requireLiveContentProjection(t, proj)
				require.Nil(t, payload.NestedResource)
			}
		}
	})

	t.Run("D4 accessible public Content -> indicator", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(63*time.Minute), shareRef("content", publicContentID, "content-ok"), nil)),
		})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, publicContentID)
		}
	})

	t.Run("D5 private Content non-owner -> suppress", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(64*time.Minute), shareRef("content", privateContentID, "content-private"), nil)),
		})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D6 followers_only follower -> indicator", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(65*time.Minute), shareRef("content", followersOnlyContentID, "content-followed"), nil)),
		})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, followersOnlyContentID)
		}
	})

	t.Run("D7 followers_only non-follower -> suppress", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(66*time.Minute), shareRef("content", notFollowedContentID, "content-not-followed"), nil)),
		})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D8 Content blocked/hidden/deleted -> suppress", func(t *testing.T) {
		targets := []uuid.UUID{blockedContentID, hiddenContentID, deletedStatusContentID, deletedAtContentID}
		for _, targetID := range targets {
			projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
				uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(67*time.Minute), shareRef("content", targetID, "content-hidden"), nil)),
			})
			require.NoError(t, err)
			for _, proj := range projections {
				payload := requireLiveContentProjection(t, proj)
				require.Nil(t, payload.NestedResource)
			}
		}
	})

	t.Run("D9 nested Content has ShareReference -> no depth-2", func(t *testing.T) {
		shallowContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(68*time.Minute), shareRef("content", nestedLeafID, "depth1"), nil)
		deepContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(69*time.Minute), shareRef("content", depth2PrimaryTargetID, "depth2"), nil)

		shallowOccurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), shallowContentID),
		}
		deepOccurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), deepContentID),
		}

		fx.tracer.reset()
		shallowProjections, err := fx.resolve(ctx, viewerID, shallowOccurrences)
		require.NoError(t, err)
		shallowCount := fx.tracer.value()
		for _, proj := range shallowProjections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, nestedLeafID)
		}

		fx.tracer.reset()
		deepProjections, err := fx.resolve(ctx, viewerID, deepOccurrences)
		require.NoError(t, err)
		deepCount := fx.tracer.value()
		for _, proj := range deepProjections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, depth2PrimaryTargetID)
		}

		require.Equal(t, shallowCount, deepCount)
	})

	t.Run("D10 accessible FPS -> indicator", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(70*time.Minute), shareRef("for_sale", fpsActiveID, "fps-ok"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsActiveID)
		}
	})

	t.Run("D11 FPS draft non-owner -> suppress", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(71*time.Minute), shareRef("for_sale", fpsDraftID, "fps-draft"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D12 FPS draft owner -> indicator", func(t *testing.T) {
		ownerViewer := fpsDraftOwnerID
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(72*time.Minute), shareRef("for_sale", fpsDraftID, "fps-draft-owner"), nil)
		projections, err := fx.resolve(ctx, ownerViewer, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsDraftID)
		}
	})

	t.Run("D13 FPS blocked/seller removed -> suppress", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(73*time.Minute), shareRef("for_sale", fpsRemovedID, "fps-removed"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D14 terminal canonically-viewable FPS -> indicator", func(t *testing.T) {
		ownerViewer := fpsTerminalOwnerID
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(74*time.Minute), shareRef("for_sale", fpsTerminalID, "fps-terminal"), nil)
		projections, err := fx.resolve(ctx, ownerViewer, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsTerminalID)
		}
	})

	t.Run("D15 accessible Auction -> indicator", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(75*time.Minute), shareRef("auction", auctionActiveID, "auction-ok"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionActiveID)
		}
	})

	t.Run("D16 Auction draft non-owner -> suppress", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(76*time.Minute), shareRef("auction", auctionDraftID, "auction-draft"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D17 Auction draft owner -> indicator", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(77*time.Minute), shareRef("auction", auctionDraftID, "auction-draft-owner"), nil)
		projections, err := fx.resolve(ctx, auctionDraftOwnerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionDraftID)
		}
	})

	t.Run("D18 Auction blocked/seller removed -> suppress", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(78*time.Minute), shareRef("auction", auctionRemovedID, "auction-removed"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D19 terminal canonically-viewable Auction -> indicator", func(t *testing.T) {
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(79*time.Minute), shareRef("auction", auctionTerminalID, "auction-terminal"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionTerminalID)
		}
	})

	t.Run("D20 missing nested target -> actual no-FK behavior", func(t *testing.T) {
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), missingTargetContentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			require.Nil(t, payload.NestedResource)
		}
	})

	t.Run("D21 malformed target type -> error", func(t *testing.T) {
		_, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), malformedTargetTypeContentID),
		})
		require.Error(t, err)
	})

	t.Run("D22 nil target UUID -> error", func(t *testing.T) {
		_, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), nilTargetUUIDContentID),
		})
		require.Error(t, err)
	})

	t.Run("D23 nested Profile infrastructure failure -> error", func(t *testing.T) {
		err := fx.appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedProfileIndicators(ctx, &contentProjectionFailingTx{Tx: tx, failOnQuery: 1, err: errors.New("boom")}, viewerID, map[uuid.UUID][]uuid.UUID{profileActiveID: {uuid.New()}})
			return err
		})
		require.Error(t, err)
	})

	t.Run("D24 nested Content infrastructure failure -> error", func(t *testing.T) {
		err := fx.appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedContentIndicators(ctx, &contentProjectionFailingTx{Tx: tx, failOnQuery: 1, err: errors.New("boom")}, viewerID, map[uuid.UUID][]uuid.UUID{publicContentID: {uuid.New()}})
			return err
		})
		require.Error(t, err)
	})

	t.Run("D25 nested commerce infrastructure failure -> error", func(t *testing.T) {
		err := fx.appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedForSaleIndicators(ctx, &contentProjectionFailingTx{Tx: tx, failOnQuery: 1, err: errors.New("boom")}, viewerID, map[uuid.UUID][]uuid.UUID{fpsActiveID: {uuid.New()}})
			return err
		})
		require.Error(t, err)

		err = fx.appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := fx.resolver.resolveNestedAuctionIndicators(ctx, &contentProjectionFailingTx{Tx: tx, failOnQuery: 1, err: errors.New("boom")}, viewerID, map[uuid.UUID][]uuid.UUID{auctionActiveID: {uuid.New()}})
			return err
		})
		require.Error(t, err)
	})

	t.Run("D26 mixed all supported target types -> correct/no cross-leakage", func(t *testing.T) {
		occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), mixedProfileContentID),
			uuid.New(): newContentOccurrence(uuid.New(), mixedContentContentID),
			uuid.New(): newContentOccurrence(uuid.New(), mixedSaleContentID),
			uuid.New(): newContentOccurrence(uuid.New(), mixedAuctionContentID),
		}
		projections, err := fx.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, projections, len(occurrences))
		for msgID, proj := range projections {
			switch occurrences[msgID].SourceID() {
			case mixedProfileContentID:
				payload := requireLiveContentProjection(t, proj)
				requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeProfile, profileActiveID)
			case mixedContentContentID:
				payload := requireLiveContentProjection(t, proj)
				requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeContent, publicContentID)
			case mixedSaleContentID:
				payload := requireLiveContentProjection(t, proj)
				requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsActiveID)
			case mixedAuctionContentID:
				payload := requireLiveContentProjection(t, proj)
				requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionActiveID)
			default:
				t.Fatalf("unexpected source id %s", occurrences[msgID].SourceID())
			}
		}
	})

	t.Run("D27 repeated same target -> deduped", func(t *testing.T) {
		one := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(81*time.Minute), mixedProfileShareRef, nil)),
		}
		twenty := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
		for i := 0; i < 20; i++ {
			twenty[uuid.New()] = newContentOccurrence(uuid.New(), fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(time.Duration(82+i)*time.Minute), mixedProfileShareRef, nil))
		}
		fx.tracer.reset()
		_, err := fx.resolve(ctx, viewerID, one)
		require.NoError(t, err)
		oneCount := fx.tracer.value()
		fx.tracer.reset()
		_, err = fx.resolve(ctx, viewerID, twenty)
		require.NoError(t, err)
		twentyCount := fx.tracer.value()
		require.Equal(t, oneCount, twentyCount)
	})

	t.Run("D28 primary TOMBSTONE -> no nested lookup/leak", func(t *testing.T) {
		tombstoneAt := base.Add(200 * time.Minute)
		tombstoneContentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, &tombstoneAt, nil, base.Add(200*time.Minute), shareRef("profile", profileActiveID, "tombstone"), nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), tombstoneContentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			requireTombstoneContentProjection(t, proj)
		}
	})

	t.Run("D29 misleading Preview -> ignored", func(t *testing.T) {
		misleadingPreview := json.RawMessage(fmt.Sprintf(`{"title":"misleading","imageUrl":"https://cdn.example.test/misleading.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}`))
		shareReference := json.RawMessage(fmt.Sprintf(`{"targetType":"profile","targetId":"%s","preview":%s}`, profileActiveID, misleadingPreview))
		contentID := fx.seedContent(t, publicAuthorID, entity.VisibilityPublic, entity.StatusActive, false, nil, nil, base.Add(201*time.Minute), shareReference, nil)
		projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newContentOccurrence(uuid.New(), contentID)})
		require.NoError(t, err)
		for _, proj := range projections {
			payload := requireLiveContentProjection(t, proj)
			requireNestedResourceIndicator(t, payload.NestedResource, chatEntity.ResourceOccurrenceResourceTypeProfile, profileActiveID)
		}
	})

	t.Run("D30 nested JSON exactly resource_type + resource_id", func(t *testing.T) {
		b, err := json.Marshal(chatApp.NestedResourceIndicator{
			ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			ResourceID:   profileActiveID,
		})
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf(`{"resource_type":"profile","resource_id":"%s"}`, profileActiveID), string(b))
	})

	t.Log("DEPTH MATRIX: 30/30 PASS")
}
