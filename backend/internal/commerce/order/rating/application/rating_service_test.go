package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/commerce/order/rating/entity"
)

// Test: Rating successful after completion

func TestCreateRating_Success_AfterCompletion(t *testing.T) {
	// This is an integration-style test placeholder
	// The actual integration with database should be tested separately
	// Here we verify the business logic flow through the entity validation

	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	comment := "Excellent seller!"

	// Create rating entity directly
	rating, err := entity.NewOrderRating(orderID, buyerID, sellerID, 5, &comment)

	if err != nil {
		t.Fatalf("NewOrderRating() error = %v", err)
	}

	if rating.RatingValue != 5 {
		t.Errorf("RatingValue = %v, want 5", rating.RatingValue)
	}

	if rating.Comment == nil || *rating.Comment != comment {
		t.Errorf("Comment = %v, want %v", rating.Comment, comment)
	}

	if rating.OrderID != orderID {
		t.Errorf("OrderID = %v, want %v", rating.OrderID, orderID)
	}

	if rating.BuyerID != buyerID {
		t.Errorf("BuyerID = %v, want %v", rating.BuyerID, buyerID)
	}

	if rating.SellerID != sellerID {
		t.Errorf("SellerID = %v, want %v", rating.SellerID, sellerID)
	}
}

// Test: RatingValue <1 rejected

func TestCreateRating_Rejected_RatingValueTooLow(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	_, err := entity.NewOrderRating(orderID, buyerID, sellerID, 0, nil)

	if err == nil {
		t.Fatal("NewOrderRating() should return error for rating value < 1")
	}

	_, ok := err.(*entity.ErrInvalidRatingValue)
	if !ok {
		t.Fatalf("Error type = %T, want *entity.ErrInvalidRatingValue", err)
	}
}

// Test: RatingValue >5 rejected

func TestCreateRating_Rejected_RatingValueTooHigh(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	_, err := entity.NewOrderRating(orderID, buyerID, sellerID, 6, nil)

	if err == nil {
		t.Fatal("NewOrderRating() should return error for rating value > 5")
	}

	_, ok := err.(*entity.ErrInvalidRatingValue)
	if !ok {
		t.Fatalf("Error type = %T, want *entity.ErrInvalidRatingValue", err)
	}
}

// Test: All valid rating values

func TestCreateRating_AllValidValues(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	for _, value := range []int{1, 2, 3, 4, 5} {
		rating, err := entity.NewOrderRating(orderID, buyerID, sellerID, value, nil)

		if err != nil {
			t.Errorf("NewOrderRating() with value %d returned error: %v", value, err)
		}

		if rating.RatingValue != value {
			t.Errorf("RatingValue = %v, want %v", rating.RatingValue, value)
		}
	}
}

// Test: Pagination limits

func TestNormalizeLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero uses default", 0, 20},
		{"negative uses default", -1, 20},
		{"within range", 10, 10},
		{"at max limit", 50, 50},
		{"exceeds max capped", 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLimit(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeLimit(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// Test: Domain error messages

func TestErrInvalidRatingValue_Error(t *testing.T) {
	err := &entity.ErrInvalidRatingValue{Value: 0}
	expected := "invalid rating value: 0 (must be between 1 and 5)"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrOrderNotCompleted_Error(t *testing.T) {
	err := &entity.ErrOrderNotCompleted{
		OrderID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Status:    "paid",
		OrderType: "offer",
	}
	expected := "cannot rate order: 00000000-0000-0000-0000-000000000001 is not completed (status: paid, type: offer)"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrNotBuyer_Error(t *testing.T) {
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	callerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	buyerID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	err := &entity.ErrNotBuyer{OrderID: orderID, CallerID: callerID, BuyerID: buyerID}
	expected := "caller 00000000-0000-0000-0000-000000000002 is not the buyer of order 00000000-0000-0000-0000-000000000001 (buyer: 00000000-0000-0000-0000-000000000003)"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrAlreadyRated_Error(t *testing.T) {
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	err := &entity.ErrAlreadyRated{OrderID: orderID}
	expected := "order 00000000-0000-0000-0000-000000000001 already rated"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrOrderNotFound_Error(t *testing.T) {
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	err := &entity.ErrOrderNotFound{OrderID: orderID}
	expected := "order not found: 00000000-0000-0000-0000-000000000001"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

// Test: Immutable behavior - verify no update/delete methods exist at entity level

func TestOrderRating_ImmutableDesign(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	rating, _ := entity.NewOrderRating(orderID, buyerID, sellerID, 5, nil)

	// The entity should have no Update or Delete methods
	// If these methods exist, they should be removed to maintain immutability
	// Go doesn't have a way to test for absence of methods at runtime,
	// but the contract is enforced by not providing mutation methods
	_ = rating

	// Verify immutability: entity has only exported fields for reading
	if rating.ID == uuid.Nil {
		t.Error("ID should be set")
	}

	// The domain provides no Update/Delete methods
	// This is enforced at the code review level
}

// Test: Comment handling

func TestOrderRating_CommentHandling(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	t.Run("nil comment", func(t *testing.T) {
		rating, err := entity.NewOrderRating(orderID, buyerID, sellerID, 5, nil)
		if err != nil {
			t.Fatalf("NewOrderRating() error = %v", err)
		}
		if rating.Comment != nil {
			t.Errorf("Comment should be nil, got %v", rating.Comment)
		}
	})

	t.Run("non-empty comment", func(t *testing.T) {
		comment := "Great seller!"
		rating, err := entity.NewOrderRating(orderID, buyerID, sellerID, 5, &comment)
		if err != nil {
			t.Fatalf("NewOrderRating() error = %v", err)
		}
		if rating.Comment == nil || *rating.Comment != comment {
			t.Errorf("Comment = %v, want %v", rating.Comment, comment)
		}
	})

	t.Run("empty string becomes nil", func(t *testing.T) {
		emptyComment := ""
		rating, err := entity.NewOrderRating(orderID, buyerID, sellerID, 5, &emptyComment)
		if err != nil {
			t.Fatalf("NewOrderRating() error = %v", err)
		}
		if rating.Comment != nil {
			t.Errorf("Comment should be nil for empty string, got %v", rating.Comment)
		}
	})
}

// Test: pgx types for repository compatibility

func TestPgxTypes(t *testing.T) {
	// Verify that pgx types are available for repository implementation
	// This test ensures the repository can use pgx.Conn and pgx.Tx

	var _ pgx.Row
	var _ pgx.Rows
	var _ pgconn.CommandTag

	// This is a compile-time check to ensure dependencies are correct
}

// Test: ValidateRatingValue utility function

func TestValidateRatingValue(t *testing.T) {
	validValues := []int{1, 2, 3, 4, 5}
	invalidValues := []int{-1, 0, 6, 10, 100}

	for _, value := range validValues {
		err := entity.ValidateRatingValue(value)
		if err != nil {
			t.Errorf("ValidateRatingValue(%d) should not return error, got: %v", value, err)
		}
	}

	for _, value := range invalidValues {
		err := entity.ValidateRatingValue(value)
		if err == nil {
			t.Errorf("ValidateRatingValue(%d) should return error", value)
		}
		_, ok := err.(*entity.ErrInvalidRatingValue)
		if !ok {
			t.Errorf("Error type for value %d = %T, want *entity.ErrInvalidRatingValue", value, err)
		}
	}
}

// Benchmark test for rating creation

func BenchmarkNewOrderRating(b *testing.B) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	comment := "Great seller!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = entity.NewOrderRating(orderID, buyerID, sellerID, 5, &comment)
	}
}

// Benchmark test for rating validation

func BenchmarkValidateRatingValue(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = entity.ValidateRatingValue(5)
	}
}


