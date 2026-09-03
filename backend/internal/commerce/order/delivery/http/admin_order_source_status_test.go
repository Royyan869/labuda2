package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestOrderDetailResponse_SourceStatus regression-locks:
//   - source_status field is present in JSON output when non-nil
//   - source_status is omitted from JSON when nil
//   - all expected listing/auction status values are representable
func TestOrderDetailResponse_SourceStatus(t *testing.T) {
	now := time.Now()

	t.Run("source_status present when set", func(t *testing.T) {
		status := "active"
		detail := OrderDetailResponse{
			ID:           uuid.New(),
			SourceType:   "for_sale",
			SourceID:     uuid.New(),
			SourceStatus: &status,
			Status:       "paid",
			EscrowStatus: "holding",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		b, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}

		got, ok := m["source_status"]
		if !ok {
			t.Error("source_status missing from JSON output")
		}
		if got != "active" {
			t.Errorf("source_status: want %q, got %v", "active", got)
		}
	})

	t.Run("source_status omitted when nil", func(t *testing.T) {
		detail := OrderDetailResponse{
			ID:           uuid.New(),
			SourceType:   "negotiation",
			SourceID:     uuid.New(),
			SourceStatus: nil,
			Status:       "paid",
			EscrowStatus: "holding",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		b, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}

		if _, ok := m["source_status"]; ok {
			t.Error("source_status should be omitted from JSON when nil")
		}
	})

	// Regression: all known listing/auction status values are strings that
	// roundtrip cleanly through the DTO.
	knownStatuses := []struct {
		sourceType string
		status     string
	}{
		{"for_sale", "draft"},
		{"for_sale", "active"},
		{"for_sale", "sold"},
		{"for_sale", "withdrawn"},
		{"auction", "scheduled"},
		{"auction", "active"},
		{"auction", "waiting_settlement"},
		{"auction", "ended"},
		{"auction", "cancelled"},
	}

	for _, tc := range knownStatuses {
		t.Run("status_"+tc.sourceType+"_"+tc.status, func(t *testing.T) {
			s := tc.status
			detail := OrderDetailResponse{
				ID:           uuid.New(),
				SourceType:   tc.sourceType,
				SourceID:     uuid.New(),
				SourceStatus: &s,
				Status:       "completed",
				EscrowStatus: "released",
				CreatedAt:    now,
				UpdatedAt:    now,
			}

			b, _ := json.Marshal(detail)
			var m map[string]interface{}
			json.Unmarshal(b, &m) //nolint:errcheck

			got, ok := m["source_status"]
			if !ok {
				t.Errorf("source_status missing for %s/%s", tc.sourceType, tc.status)
			}
			if got != tc.status {
				t.Errorf("want %q, got %v", tc.status, got)
			}
		})
	}
}


