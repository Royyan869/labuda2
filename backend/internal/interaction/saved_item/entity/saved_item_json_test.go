package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Negative contract for the saved-items wire shape.
//
// The mobile SavedItemModel parses snake_case keys only. Before the entity
// carried JSON tags, Go field-name (PascalCase) serialization shipped: the
// POST persisted the row while the client failed to parse it, and
// GET /saved-items silently rendered an empty list. This test runs without a
// database, so the contract stays guarded even when the DB-backed handler
// contract test is skipped.
func TestSavedItemJSONSerialization(t *testing.T) {
	userID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	sellerID := uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")
	targetID := uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")
	createdAt := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

	t.Run("created saved item uses snake_case", func(t *testing.T) {
		item := &SavedItem{
			ID:         uuid.New(),
			UserID:     userID,
			TargetType: TargetTypeForSale,
			TargetID:   targetID,
			IntentType: IntentTypeBookmark,
			SellerID:   &sellerID,
			CreatedAt:  createdAt,
		}

		raw, err := json.Marshal(item)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		for _, key := range []string{"id", "user_id", "target_type", "target_id", "intent_type", "seller_id", "created_at"} {
			assert.Contains(t, result, key)
		}
		for _, key := range []string{"ID", "UserID", "TargetType", "TargetID", "IntentType", "SellerID", "CreatedAt"} {
			assert.NotContains(t, result, key)
		}
		assert.Equal(t, "for_sale", result["target_type"])
		assert.Equal(t, "bookmark", result["intent_type"])
		assert.Equal(t, sellerID.String(), result["seller_id"])
	})

	t.Run("saved forSale projects media snapshot as readable URL array", func(t *testing.T) {
		item := &SavedItemWithForSale{
			SavedItem: SavedItem{
				ID:         uuid.New(),
				UserID:     userID,
				TargetType: TargetTypeForSale,
				TargetID:   targetID,
				IntentType: IntentTypeBookmark,
				SellerID:   &sellerID,
				CreatedAt:  createdAt,
			},
			ForSaleTitle:      "Showa Koi",
			ForSalePrice:      1250000,
			ForSaleType:       "fixed_price",
			QuantityAvailable: 3,
			ForSaleStatus:     "active",
			ForSaleVisibility: "public",
			ForSaleMediaURLs:  []byte(`[{"url":"https://cdn.example.com/koi.jpg","blurhash":"LEHV6n"}]`),
		}

		raw, err := json.Marshal(item)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		assert.Equal(t, "for_sale", result["target_type"])
		assert.Equal(t, "Showa Koi", result["for_sale_title"])
		assert.EqualValues(t, 1250000, result["for_sale_price"])
		assert.Equal(t, "fixed_price", result["for_sale_type"])
		assert.EqualValues(t, 3, result["quantity_available"])
		assert.Equal(t, "active", result["for_sale_status"])
		assert.Equal(t, "public", result["for_sale_visibility"])
		assert.Equal(t, []interface{}{"https://cdn.example.com/koi.jpg"}, result["for_sale_media_urls"])
		assert.NotContains(t, result, "ForSaleMediaURLs")
		assert.NotContains(t, result, "ForSaleTitle")
	})

	t.Run("legacy bare-string media snapshot is tolerated", func(t *testing.T) {
		item := &SavedItemWithForSale{
			SavedItem: SavedItem{
				ID:         uuid.New(),
				UserID:     userID,
				TargetType: TargetTypeForSale,
				TargetID:   targetID,
				IntentType: IntentTypeBookmark,
				CreatedAt:  createdAt,
			},
			ForSaleTitle:     "Legacy Koi",
			ForSaleMediaURLs: []byte(`["https://cdn.example.com/legacy.jpg"]`),
		}

		raw, err := json.Marshal(item)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		assert.Equal(t, []interface{}{"https://cdn.example.com/legacy.jpg"}, result["for_sale_media_urls"])
	})

	t.Run("saved auction uses snake_case snapshot keys", func(t *testing.T) {
		startPrice := int64(1500000)
		currentBid := int64(1750000)
		endAt := createdAt.Add(2 * time.Hour)

		item := &SavedItemWithAuction{
			SavedItem: SavedItem{
				ID:         uuid.New(),
				UserID:     userID,
				TargetType: TargetTypeAuction,
				TargetID:   targetID,
				IntentType: IntentTypeWatch,
				CreatedAt:  createdAt,
			},
			AuctionTitle:  "Auction Koi",
			AuctionStatus: "active",
			StartPrice:    &startPrice,
			CurrentBid:    &currentBid,
			EndAt:         &endAt,
		}

		raw, err := json.Marshal(item)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		assert.Equal(t, "auction", result["target_type"])
		assert.Equal(t, "watch", result["intent_type"])
		assert.Equal(t, "Auction Koi", result["auction_title"])
		assert.Equal(t, "active", result["auction_status"])
		assert.EqualValues(t, 1500000, result["start_price"])
		assert.EqualValues(t, 1750000, result["current_bid"])
		assert.Contains(t, result, "end_at")
		assert.NotContains(t, result, "seller_id", "auction snapshot has no seller authority")
		for _, key := range []string{"AuctionTitle", "AuctionStatus", "StartPrice", "CurrentBid", "EndAt"} {
			assert.NotContains(t, result, key)
		}
	})

	t.Run("saved list envelope uses snake_case", func(t *testing.T) {
		list := &SavedItemList{
			UserID: userID,
			Items: []*SavedItemWithForSale{
				{SavedItem: SavedItem{ID: uuid.New(), UserID: userID, TargetType: TargetTypeForSale, TargetID: targetID, IntentType: IntentTypeBookmark, CreatedAt: createdAt}},
			},
			Auctions: []*SavedItemWithAuction{
				{SavedItem: SavedItem{ID: uuid.New(), UserID: userID, TargetType: TargetTypeAuction, TargetID: targetID, IntentType: IntentTypeWatch, CreatedAt: createdAt}},
			},
			Total:   2,
			Page:    1,
			PerPage: 20,
		}

		raw, err := json.Marshal(list)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		for _, key := range []string{"user_id", "items", "auctions", "total", "page", "per_page"} {
			assert.Contains(t, result, key)
		}
		for _, key := range []string{"UserID", "Items", "Auctions", "Total", "Page", "PerPage"} {
			assert.NotContains(t, result, key)
		}

		items, ok := result["items"].([]interface{})
		require.True(t, ok)
		require.Len(t, items, 1)
		first, ok := items[0].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, first, "target_type")
		assert.NotContains(t, first, "TargetType")
	})
}
