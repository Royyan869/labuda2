package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Negative contract for the dispute wire shape.
//
// POST /orders/:id/dispute returns the Dispute entity directly, and the mobile
// consumer (DisputeDto.fromJson) parses snake_case keys only. Without JSON tags
// the entity serialized Go field names (ID, OrderID, ...), which the tolerant
// client parser silently turned into empty ids/status. This test runs without
// a database so the contract stays guarded when DB-backed tests are skipped.
func TestDisputeJSONSerialization(t *testing.T) {
	disputeID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	orderID := uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")
	buyerID := uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")
	sellerID := uuid.MustParse("423e4567-e89b-12d3-a456-426614174000")
	adminID := uuid.MustParse("523e4567-e89b-12d3-a456-426614174000")
	openedAt := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	createdAt := openedAt
	updatedAt := openedAt.Add(time.Hour)

	t.Run("open dispute uses the mobile snake_case contract", func(t *testing.T) {
		dispute := &Dispute{
			ID:           disputeID,
			OrderID:      orderID,
			BuyerID:      buyerID,
			SellerID:     sellerID,
			Reason:       "Barang rusak",
			Description:  strPtr("Item arrived damaged"),
			Status:       DisputeStatusUnderReview,
			OpenedAt:     openedAt,
			CallerID:     &buyerID,
			ReasonCode:   strPtr(ReasonCodeItemNotReceived),
			EvidenceURLs: []string{"https://cdn.example.com/evidence.mp4"},
			TimeoutDays:  DefaultDisputeTimeoutDays,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		}

		raw, err := json.Marshal(dispute)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		for _, key := range []string{
			"id", "order_id", "buyer_id", "seller_id", "reason", "description",
			"status", "opened_at", "caller_id", "reason_code", "evidence_urls",
			"timeout_days", "is_overdue", "created_at", "updated_at",
		} {
			assert.Contains(t, result, key)
		}
		for _, key := range []string{
			"ID", "OrderID", "BuyerID", "SellerID", "Reason", "Description",
			"Status", "OpenedAt", "CallerID", "ReasonCode", "EvidenceURLs",
			"TimeoutDays", "IsOverdue", "CreatedAt", "UpdatedAt",
		} {
			assert.NotContains(t, result, key)
		}
		assert.Equal(t, disputeID.String(), result["id"])
		assert.Equal(t, orderID.String(), result["order_id"])
		assert.Equal(t, "under_review", result["status"])
		assert.Equal(t, "Barang rusak", result["reason"])
		assert.Equal(t, []interface{}{"https://cdn.example.com/evidence.mp4"}, result["evidence_urls"])

		// Unresolved dispute must not leak resolution keys.
		assert.NotContains(t, result, "resolved_at")
		assert.NotContains(t, result, "resolved_by")
		assert.NotContains(t, result, "resolution_notes")
	})

	t.Run("resolved dispute carries resolution fields", func(t *testing.T) {
		resolvedAt := openedAt.Add(48 * time.Hour)
		dispute := &Dispute{
			ID:              disputeID,
			OrderID:         orderID,
			BuyerID:         buyerID,
			SellerID:        sellerID,
			Reason:          "Tidak sesuai deskripsi",
			Status:          DisputeStatusResolvedRefund,
			OpenedAt:        openedAt,
			ResolvedAt:      &resolvedAt,
			ResolvedBy:      &adminID,
			ResolutionNotes: strPtr("Refund penuh untuk pembeli"),
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}

		raw, err := json.Marshal(dispute)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &result))

		assert.Equal(t, "resolved_refund", result["status"])
		assert.Equal(t, adminID.String(), result["resolved_by"])
		assert.Equal(t, "Refund penuh untuk pembeli", result["resolution_notes"])
		assert.Contains(t, result, "resolved_at")
		assert.NotContains(t, result, "ResolvedAt")
		assert.NotContains(t, result, "ResolvedBy")
	})
}

func strPtr(v string) *string { return &v }
