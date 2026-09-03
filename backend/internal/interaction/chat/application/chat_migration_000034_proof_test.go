//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestMigration000034_RealPostgresProofs(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)

	sender := insertChatAuthorityUser(t, ctx, appDB, "occ-sender")
	other := insertChatAuthorityUser(t, ctx, appDB, "occ-other")
	room := insertChatAuthorityRoom(t, ctx, appDB, sender, other)

	insertMsg := func(t *testing.T) uuid.UUID {
		t.Helper()
		body := "test"
		id := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, idempotency_key, command_fingerprint, created_at)
			VALUES ($1,$2,$3,'text','test',$4,$5,NOW())
		`, id, room, sender, uuid.NewString(), chatEntity.ComputeCommandFingerprint(sender, chatEntity.MessageTypeText, &body, nil))
		require.NoError(t, err)
		return id
	}

	// A: zero sources -> CHECK violation
	t.Run("A_zero_sources_rejected", func(t *testing.T) {
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', '{}', NOW())`, msgID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "chat_occurrence_exactly_one_source")
	})

	// B: two sources -> CHECK violation
	t.Run("B_two_sources_rejected", func(t *testing.T) {
		pid := insertChatAuthorityUser(t, ctx, appDB, "occ-b-"+uuid.NewString()[:8])
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, content_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, $2, '{}', NOW())`, msgID, pid)
		require.Error(t, err)
		require.Contains(t, err.Error(), "chat_occurrence_exactly_one_source")
	})

	// C: Profile only -> success
	t.Run("C_profile_success", func(t *testing.T) {
		pid := insertChatAuthorityUser(t, ctx, appDB, "occ-c-"+uuid.NewString()[:8])
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{"username":"test"}'::jsonb, NOW())`, msgID, pid)
		require.NoError(t, err)
	})

	// D: Content only -> success
	t.Run("D_content_success", func(t *testing.T) {
		contentID := uuid.New()
		authorID := insertChatAuthorityUser(t, ctx, appDB, "occ-d-"+uuid.NewString()[:8])
		_, err := pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, caption, created_at, updated_at)
			VALUES ($1, $2, 'test content', NOW(), NOW())`, contentID, authorID)
		require.NoError(t, err)
		msgID := insertMsg(t)
		_, err = pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, content_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{"caption_excerpt":"test"}'::jsonb, NOW())`, msgID, contentID)
		require.NoError(t, err)
	})

	// E: FPS only -> success
	t.Run("E_fps_success", func(t *testing.T) {
		sellerID := insertChatAuthorityUser(t, ctx, appDB, "occ-e-"+uuid.NewString()[:8])
		productID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'test product', '', '', 'same_day', NOW(), NOW())`, productID, sellerID)
		require.NoError(t, err)
		fpsID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, created_at, updated_at)
			VALUES ($1, $2, $3, 10000, 'active', NOW(), NOW())`, fpsID, productID, sellerID)
		require.NoError(t, err)
		msgID := insertMsg(t)
		_, err = pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, for_sale_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{"title":"test"}'::jsonb, NOW())`, msgID, fpsID)
		require.NoError(t, err)
	})

	// F: Auction only -> success
	t.Run("F_auction_success", func(t *testing.T) {
		sellerID := insertChatAuthorityUser(t, ctx, appDB, "occ-f-"+uuid.NewString()[:8])
		productID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'test auction product', '', '', 'same_day', NOW(), NOW())`, productID, sellerID)
		require.NoError(t, err)
		auctionID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO auctions (id, product_id, seller_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at)
			VALUES ($1, $2, $3, 10000, 1000, NOW(), NOW() + INTERVAL '7 days', 'active', NOW(), NOW())`, auctionID, productID, sellerID)
		require.NoError(t, err)
		msgID := insertMsg(t)
		_, err = pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, auction_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{"title":"test"}'::jsonb, NOW())`, msgID, auctionID)
		require.NoError(t, err)
	})

	// G: direct + Profile -> rejected
	t.Run("G_direct_profile_rejected", func(t *testing.T) {
		pid := insertChatAuthorityUser(t, ctx, appDB, "occ-g-"+uuid.NewString()[:8])
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'direct_commerce_insert_chat', $2, '{}', NOW())`, msgID, pid)
		require.Error(t, err)
		require.Contains(t, err.Error(), "chat_occurrence_valid_operation")
	})

	// H: direct + Content -> rejected
	t.Run("H_direct_content_rejected", func(t *testing.T) {
		contentID := uuid.New()
		authorID := insertChatAuthorityUser(t, ctx, appDB, "occ-h-"+uuid.NewString()[:8])
		_, err := pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, caption, created_at, updated_at)
			VALUES ($1, $2, 'test', NOW(), NOW())`, contentID, authorID)
		require.NoError(t, err)
		msgID := insertMsg(t)
		_, err = pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, content_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'direct_commerce_insert_chat', $2, '{}', NOW())`, msgID, contentID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "chat_occurrence_valid_operation")
	})

	// I: duplicate message_id -> rejected
	t.Run("I_duplicate_rejected", func(t *testing.T) {
		pid := insertChatAuthorityUser(t, ctx, appDB, "occ-i-"+uuid.NewString()[:8])
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{}', NOW())`, msgID, pid)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{}', NOW())`, msgID, pid)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate key")
	})

	// J: source DELETE -> RESTRICT
	t.Run("J_source_delete_restrict", func(t *testing.T) {
		pid := insertChatAuthorityUser(t, ctx, appDB, "occ-j-"+uuid.NewString()[:8])
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{}', NOW())`, msgID, pid)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, pid)
		require.Error(t, err)
		require.Contains(t, err.Error(), "violates foreign key constraint")
	})

	// K: message DELETE -> occurrence CASCADE
	t.Run("K_message_delete_cascade", func(t *testing.T) {
		pid := insertChatAuthorityUser(t, ctx, appDB, "occ-k-"+uuid.NewString()[:8])
		msgID := insertMsg(t)
		_, err := pool.Exec(ctx, `
			INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, fallback_snapshot, created_at)
			VALUES ($1, 'share_to_chat', $2, '{}', NOW())`, msgID, pid)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM chat_messages WHERE id = $1`, msgID)
		require.NoError(t, err)
		var count int
		err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM chat_message_resource_occurrences WHERE message_id = $1`, msgID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "occurrence must be CASCADE-deleted with message")
	})
}
