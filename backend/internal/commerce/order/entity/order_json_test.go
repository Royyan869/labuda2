package entity

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrderJSONSerialization verifies that Order entity serializes to JSON with correct field names
func TestOrderJSONSerialization(t *testing.T) {
	// Create a test order
	orderID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	buyerID := uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")
	sellerID := uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")

	order := &Order{
		ID:       orderID,
		BuyerID:  buyerID,
		SellerID: sellerID,
		Status:   StatusPending,
		Quantity: 1,
	}

	// Serialize to JSON
	data, err := json.Marshal(order)
	require.NoError(t, err)

	// Deserialize to map to check field names
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify critical fields have correct snake_case names
	assert.Contains(t, result, "id", "JSON should contain 'id' field (not 'ID')")
	assert.Contains(t, result, "buyer_id", "JSON should contain 'buyer_id' field (not 'BuyerID')")
	assert.Contains(t, result, "seller_id", "JSON should contain 'seller_id' field (not 'SellerID')")
	assert.Contains(t, result, "status", "JSON should contain 'status' field")

	// Verify values are correct
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", result["id"])
	assert.Equal(t, "223e4567-e89b-12d3-a456-426614174000", result["buyer_id"])
	assert.Equal(t, "323e4567-e89b-12d3-a456-426614174000", result["seller_id"])
	assert.Equal(t, "pending_payment", result["status"])

	// Verify PascalCase fields do NOT exist
	assert.NotContains(t, result, "ID", "JSON should NOT contain 'ID' field")
	assert.NotContains(t, result, "BuyerID", "JSON should NOT contain 'BuyerID' field")
	assert.NotContains(t, result, "SellerID", "JSON should NOT contain 'SellerID' field")
}


