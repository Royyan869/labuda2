//go:build integration

package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestContentResourceOccurrence_PostgresConstraints(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	actorID := seedOccurrenceUser(t, ctx, tdb.Pool(), "active")
	svc := newOccurrenceService(true)

	t.Run("CW16 exactly one typed source enforced by DB", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			targetAuthor := seedOccurrenceUser(t, ctx, tx, "active")
			targetContent := createOrdinaryContent(t, ctx, tx, svc, targetAuthor, "target")

			contentID := uuid.New()
			_, err := tx.Exec(ctx, `
				INSERT INTO contents (id, author_id, status, caption, visibility, created_at, updated_at)
				VALUES ($1, $2, 'active', 'db exact one', 'public', NOW(), NOW())
			`, contentID, actorID)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `
				INSERT INTO content_resource_occurrences (
					content_id, actor_id, operation, profile_source_id, content_source_id,
					for_sale_source_id, auction_source_id, created_at
				)
				VALUES ($1, $2, 'share_to_feed', $3, $4, NULL, NULL, NOW())
			`, contentID, actorID, actorID, targetContent.ID)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
	})

	t.Run("CW17 operation/resource compatibility enforced by DB", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			contentID := uuid.New()
			_, err := tx.Exec(ctx, `
				INSERT INTO contents (id, author_id, status, caption, visibility, created_at, updated_at)
				VALUES ($1, $2, 'active', 'db compat occurrence', 'public', NOW(), NOW())
			`, contentID, actorID)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `
				INSERT INTO content_resource_occurrences (
					content_id, actor_id, operation, profile_source_id, content_source_id,
					for_sale_source_id, auction_source_id, created_at
				)
				VALUES ($1, $2, 'direct_commerce_insert_content', $3, NULL, NULL, NULL, NOW())
			`, contentID, actorID, actorID)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
	})

	t.Run("CW18 anti-self-reference enforced", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			contentID := uuid.New()
			_, err := tx.Exec(ctx, `
				INSERT INTO contents (id, author_id, status, caption, visibility, created_at, updated_at)
				VALUES ($1, $2, 'active', 'db self-ref', 'public', NOW(), NOW())
			`, contentID, actorID)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `
				INSERT INTO content_resource_occurrences (
					content_id, actor_id, operation, profile_source_id, content_source_id,
					for_sale_source_id, auction_source_id, created_at
				)
				VALUES ($1, $2, 'share_to_feed', NULL, $1, NULL, NULL, NOW())
			`, contentID, actorID)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
	})

	t.Run("CW19 occurrence UPDATE rejected by DB", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			contentID := uuid.New()
			_, err := tx.Exec(ctx, `
				INSERT INTO contents (id, author_id, status, caption, visibility, created_at, updated_at)
				VALUES ($1, $2, 'active', 'db update', 'public', NOW(), NOW())
			`, contentID, actorID)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `
				INSERT INTO content_resource_occurrences (
					content_id, actor_id, operation, profile_source_id, content_source_id,
					for_sale_source_id, auction_source_id, created_at
				)
				VALUES ($1, $2, 'share_to_feed', $3, NULL, NULL, NULL, NOW())
			`, contentID, actorID, actorID)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `
				UPDATE content_resource_occurrences
				SET actor_id = $2
				WHERE content_id = $1
			`, contentID, actorID)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
	})

	t.Run("cascade from owning content deletes occurrence", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			content, err := svc.CreateContentWithResourceOccurrence(
				ctx,
				tx,
				actorID,
				"db cascade",
				contententity.VisibilityPublic,
				nil,
				nil,
				&contententity.ContentResourceOccurrenceIdentity{
					Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
					ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
					ResourceID:   actorID,
				},
				nil,
			)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `DELETE FROM contents WHERE id = $1`, content.ID)
			require.NoError(t, err)

			var occurrenceCount int
			require.NoError(t, tx.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM content_resource_occurrences
				WHERE content_id = $1
			`, content.ID).Scan(&occurrenceCount))
			require.Equal(t, 0, occurrenceCount)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("source row delete is restricted for profile source", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			sourceUser := seedOccurrenceUser(t, ctx, tx, "active")
			content, err := svc.CreateContentWithResourceOccurrence(
				ctx,
				tx,
				actorID,
				"profile source restriction",
				contententity.VisibilityPublic,
				nil,
				nil,
				&contententity.ContentResourceOccurrenceIdentity{
					Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
					ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
					ResourceID:   sourceUser,
				},
				nil,
			)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, sourceUser)
			require.Error(t, err)
			_ = content
			return err
		})
		require.Error(t, err)
	})

	t.Run("source row delete is restricted for content source", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			sourceAuthor := seedOccurrenceUser(t, ctx, tx, "active")
			sourceContent := createOrdinaryContent(t, ctx, tx, svc, sourceAuthor, "source content")
			content, err := svc.CreateContentWithResourceOccurrence(
				ctx,
				tx,
				actorID,
				"content source restriction",
				contententity.VisibilityPublic,
				nil,
				nil,
				&contententity.ContentResourceOccurrenceIdentity{
					Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
					ResourceType: contententity.ContentResourceOccurrenceResourceTypeContent,
					ResourceID:   sourceContent.ID,
				},
				nil,
			)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `DELETE FROM contents WHERE id = $1`, sourceContent.ID)
			require.Error(t, err)
			_ = content
			return err
		})
		require.Error(t, err)
	})

	t.Run("source row delete is restricted for fixed-price sale source", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			sellerID := seedOccurrenceUser(t, ctx, tx, "active")
			saleID := seedOccurrenceForSale(t, ctx, tx, sellerID, "active")
			_, err := svc.CreateContentWithResourceOccurrence(
				ctx,
				tx,
				actorID,
				"sale source restriction",
				contententity.VisibilityPublic,
				nil,
				nil,
				&contententity.ContentResourceOccurrenceIdentity{
					Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
					ResourceType: contententity.ContentResourceOccurrenceResourceTypeForSale,
					ResourceID:   saleID,
				},
				nil,
			)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `DELETE FROM for_sales WHERE id = $1`, saleID)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
	})

	t.Run("source row delete is restricted for auction source", func(t *testing.T) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			sellerID := seedOccurrenceUser(t, ctx, tx, "active")
			auctionID := seedOccurrenceAuction(t, ctx, tx, sellerID, "active")
			_, err := svc.CreateContentWithResourceOccurrence(
				ctx,
				tx,
				actorID,
				"auction source restriction",
				contententity.VisibilityPublic,
				nil,
				nil,
				&contententity.ContentResourceOccurrenceIdentity{
					Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
					ResourceType: contententity.ContentResourceOccurrenceResourceTypeAuction,
					ResourceID:   auctionID,
				},
				nil,
			)
			require.NoError(t, err)

			_, err = tx.Exec(ctx, `DELETE FROM auctions WHERE id = $1`, auctionID)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
	})

	t.Run("CW21 Content + occurrence atomic on failure", func(t *testing.T) {
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

			require.NoError(t, contentrepo.NewContentRepository().CreateResourceOccurrence(ctx, tx, occ))
			err := contentrepo.NewContentRepository().CreateResourceOccurrence(ctx, tx, occ)
			require.Error(t, err)
			return err
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, contentrepo.ErrDuplicateContentResourceOccurrence) || containsDuplicate(err.Error()))

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
	})
}

func containsDuplicate(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "duplicate")
}
