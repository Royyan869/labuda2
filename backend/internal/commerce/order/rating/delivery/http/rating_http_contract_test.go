package http

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	ratingEntity "github.com/labuda/backend/internal/commerce/order/rating/entity"
	ratingRepo "github.com/labuda/backend/internal/commerce/order/rating/infrastructure/repository"
)

// The Rating HTTP contract is LOCKED to:
//   - exactly the 4 live endpoints: POST /orders/:id/ratings,
//     GET /users/:id/ratings, GET /users/:id/ratings/summary,
//     GET /users/me/ratings/given. There is NO /ratings/state.
//   - bare list responses with limit/cursor (int64 Unix-ns) request semantics.
//   - snake_case JSON keys over the existing OrderRating / RatingSummary domain
//     models, with buyer_id / seller_id as the raw identity.
//   - no reviewer, no verified_purchase, no items/has_more/next_cursor, no
//     opaque RatingCursor.

func TestRatingOrderRatingSerializesSnakeCaseCanonical(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	comment := "great seller"
	rating := &ratingEntity.OrderRating{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		OrderID:     uuid.MustParse("660e8400-e29b-41d4-a716-446655440001"),
		BuyerID:     uuid.MustParse("770e8400-e29b-41d4-a716-446655440002"),
		SellerID:    uuid.MustParse("880e8400-e29b-41d4-a716-446655440003"),
		RatingValue: 5,
		Comment:     &comment,
		CreatedAt:   now,
	}

	b, err := json.Marshal(rating)
	if err != nil {
		t.Fatalf("marshal OrderRating: %v", err)
	}
	raw := string(b)

	for _, key := range []string{
		`"id"`, `"order_id"`, `"buyer_id"`, `"seller_id"`,
		`"rating_value"`, `"comment"`, `"created_at"`,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("OrderRating JSON missing snake_case key %s: %s", key, raw)
		}
	}
	// Raw identity only — no reviewer, no verified_purchase, no PascalCase.
	for _, forbidden := range []string{
		`"reviewer"`, `"verified_purchase"`,
		`"ID"`, `"OrderID"`, `"BuyerID"`, `"SellerID"`, `"RatingValue"`, `"CreatedAt"`,
	} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("OrderRating JSON must not contain %s: %s", forbidden, raw)
		}
	}
}

func TestRatingSummarySerializesSnakeCaseCanonical(t *testing.T) {
	s := &ratingRepo.RatingSummary{
		TotalRatings:   10,
		AverageRating:  4.5,
		OneStarCount:   0,
		TwoStarCount:   0,
		ThreeStarCount: 1,
		FourStarCount:  3,
		FiveStarCount:  6,
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal RatingSummary: %v", err)
	}
	raw := string(b)
	for _, key := range []string{
		`"total_ratings"`, `"average_rating"`, `"one_star_count"`,
		`"two_star_count"`, `"three_star_count"`, `"four_star_count"`, `"five_star_count"`,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("RatingSummary JSON missing snake_case key %s: %s", key, raw)
		}
	}
	for _, forbidden := range []string{`"TotalRatings"`, `"AverageRating"`, `"FiveStarCount"`} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("RatingSummary JSON must not contain %s: %s", forbidden, raw)
		}
	}
}

// TestRatingRoutesExactLiveSetAndNoState pins the locked route set and proves
// there is no /ratings/state route, no GetRatingState handler.
func TestRatingRoutesExactLiveSetAndNoState(t *testing.T) {
	handlerSrc, err := os.ReadFile("rating_handler.go")
	if err != nil {
		t.Fatalf("read rating_handler.go: %v", err)
	}
	handler := string(handlerSrc)

	for _, m := range []string{
		"func (h *RatingHandler) CreateRating(c *gin.Context)",
		"func (h *RatingHandler) ListRatingsReceived(c *gin.Context)",
		"func (h *RatingHandler) ListRatingsGiven(c *gin.Context)",
		"func (h *RatingHandler) GetRatingSummary(c *gin.Context)",
	} {
		if !strings.Contains(handler, m) {
			t.Errorf("rating_handler.go missing %q", m)
		}
	}
	if strings.Contains(handler, "GetRatingState") {
		t.Error("rating_handler.go must NOT define GetRatingState")
	}

	routesSrc, err := os.ReadFile("../../../../../../cmd/core_server/routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	routes := string(routesSrc)
	for _, r := range []string{
		`orderRoutes.POST("/:id/ratings", middleware.RequireActiveAccount(db.Pgx()), deps.RatingHandler.CreateRating)`,
		`v1.GET("/users/:id/ratings", deps.RatingHandler.ListRatingsReceived)`,
		`v1.GET("/users/:id/ratings/summary", deps.RatingHandler.GetRatingSummary)`,
		`v1.GET("/users/me/ratings/given", deps.RatingHandler.ListRatingsGiven)`,
	} {
		if !strings.Contains(routes, r) {
			t.Errorf("routes_core.go missing route %q", r)
		}
	}
	if strings.Contains(routes, `/ratings/state`) {
		t.Error("routes_core.go must NOT register a /ratings/state route")
	}
}

// TestRatingHandlerUsesLimitCursorInt64 proves the live handler reads limit
// and int64 (Unix-ns) cursor request semantics and returns bare collections.
func TestRatingHandlerUsesLimitCursorInt64(t *testing.T) {
	handlerSrc, err := os.ReadFile("rating_handler.go")
	if err != nil {
		t.Fatalf("read rating_handler.go: %v", err)
	}
	handler := string(handlerSrc)

	if !strings.Contains(handler, "Cursor int64") {
		t.Error("rating_handler.go must declare cursor as int64 (Unix-nanosecond semantics)")
	}
	if !strings.Contains(handler, "Limit  int") {
		t.Error("rating_handler.go must declare limit")
	}
	for _, want := range []string{
		"c.ShouldBindQuery(&req)",
		"response.Success(c, ratings)",
		"response.Success(c, summary)",
	} {
		if !strings.Contains(handler, want) {
			t.Errorf("rating_handler.go missing %q (bare-collection response)", want)
		}
	}
	// No rejected envelope / opaque cursor / reviewer DTO in the handler.
	for _, forbidden := range []string{
		"next_cursor", "has_more", "RatingCursor", "reviewer", "verified_purchase",
	} {
		if strings.Contains(handler, forbidden) {
			t.Errorf("rating_handler.go must not reference rejected surface %q", forbidden)
		}
	}
}