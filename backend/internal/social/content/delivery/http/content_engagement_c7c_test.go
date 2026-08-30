package http

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// C7C — Structural tests for EngagementResponse contract.
//
// Verifies that:
// 1. ContentResponse serialises engagement with camelCase keys matching
//    mobile ContentEngagementDto expectations.
// 2. Zero-value EngagementResponse serialises likeCount and commentCount
//    explicitly (no omitempty on int fields — zero IS meaningful).
// 3. engagement=nil omits the field entirely (omitempty on the pointer).
// 4. Removed phantom fields (viewCount, shareCount, saveCount, reportCount)
//    are no longer emitted.

func TestEngagementResponse_CamelCaseKeys(t *testing.T) {
	eng := &EngagementResponse{
		LikeCount:    5,
		CommentCount: 3,
	}

	data, err := json.Marshal(eng)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Only canonical fields should be present.
	wantKeys := []string{"likeCount", "commentCount"}
	for _, k := range wantKeys {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q in engagement JSON", k)
		}
	}

	// Verify removed phantom fields are NOT present.
	removedKeys := []string{"viewCount", "shareCount", "saveCount", "reportCount"}
	for _, k := range removedKeys {
		if _, ok := raw[k]; ok {
			t.Errorf("removed phantom field %q still present in engagement JSON", k)
		}
	}

	// Verify no snake_case keys leaked.
	snakeKeys := []string{"like_count", "comment_count"}
	for _, k := range snakeKeys {
		if _, ok := raw[k]; ok {
			t.Errorf("unexpected snake_case key %q — mobile expects camelCase", k)
		}
	}
}

func TestEngagementResponse_ZeroValuesPresent(t *testing.T) {
	eng := &EngagementResponse{} // all zeros

	data, err := json.Marshal(eng)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Both canonical fields must be present even at zero (no omitempty on int).
	wantKeys := []string{"likeCount", "commentCount"}
	for _, k := range wantKeys {
		v, ok := raw[k]
		if !ok {
			t.Errorf("zero engagement missing key %q", k)
			continue
		}
		if v.(float64) != 0 {
			t.Errorf("zero engagement key %q = %v; want 0", k, v)
		}
	}
}

func TestContentResponse_EngagementNil_Omitted(t *testing.T) {
	resp := ContentResponse{
		ID:        uuid.New(),
		Caption:   "test",
		AuthorID:  uuid.New(),
		Status:    "active",
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := raw["engagement"]; ok {
		t.Error("nil engagement should be omitted (omitempty on pointer)")
	}
}

func TestContentResponse_EngagementPresent(t *testing.T) {
	resp := ContentResponse{
		ID:        uuid.New(),
		Caption:   "test",
		AuthorID:  uuid.New(),
		Status:    "active",
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
		Engagement: &EngagementResponse{
			LikeCount: 42,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	engRaw, ok := raw["engagement"]
	if !ok {
		t.Fatal("engagement field missing when set")
	}

	engMap, ok := engRaw.(map[string]interface{})
	if !ok {
		t.Fatal("engagement is not a JSON object")
	}

	if got := engMap["likeCount"].(float64); got != 42 {
		t.Errorf("likeCount = %v; want 42", got)
	}
}
