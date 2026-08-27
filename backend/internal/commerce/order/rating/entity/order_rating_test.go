package entity

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewOrderRating_Valid(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	ratingValue := 5
	comment := strPtr("Great seller!")

	rating, err := NewOrderRating(orderID, buyerID, sellerID, ratingValue, comment)

	if err != nil {
		t.Fatalf("NewOrderRating() error = %v", err)
	}

	if rating.ID == uuid.Nil {
		t.Error("ID should be generated")
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
	if rating.RatingValue != ratingValue {
		t.Errorf("RatingValue = %v, want %v", rating.RatingValue, ratingValue)
	}
	if rating.Comment == nil || *rating.Comment != "Great seller!" {
		t.Errorf("Comment = %v, want %v", rating.Comment, comment)
	}
	if rating.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestNewOrderRating_NoComment(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	rating, err := NewOrderRating(orderID, buyerID, sellerID, 4, nil)

	if err != nil {
		t.Fatalf("NewOrderRating() error = %v", err)
	}

	if rating.Comment != nil {
		t.Errorf("Comment should be nil, got %v", rating.Comment)
	}
}

func TestNewOrderRating_EmptyCommentBecomesNil(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	emptyComment := strPtr("")

	rating, err := NewOrderRating(orderID, buyerID, sellerID, 3, emptyComment)

	if err != nil {
		t.Fatalf("NewOrderRating() error = %v", err)
	}

	if rating.Comment != nil {
		t.Errorf("Comment should be nil for empty string, got %v", rating.Comment)
	}
}

func TestNewOrderRating_InvalidRatingValue_TooLow(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	_, err := NewOrderRating(orderID, buyerID, sellerID, 0, nil)

	if err == nil {
		t.Fatal("NewOrderRating() should return error for rating value 0")
	}

	ratingErr, ok := err.(*ErrInvalidRatingValue)
	if !ok {
		t.Fatalf("Error type = %T, want *ErrInvalidRatingValue", err)
	}
	if ratingErr.Value != 0 {
		t.Errorf("Error.Value = %v, want 0", ratingErr.Value)
	}
}

func TestNewOrderRating_InvalidRatingValue_TooHigh(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	_, err := NewOrderRating(orderID, buyerID, sellerID, 6, nil)

	if err == nil {
		t.Fatal("NewOrderRating() should return error for rating value 6")
	}

	ratingErr, ok := err.(*ErrInvalidRatingValue)
	if !ok {
		t.Fatalf("Error type = %T, want *ErrInvalidRatingValue", err)
	}
	if ratingErr.Value != 6 {
		t.Errorf("Error.Value = %v, want 6", ratingErr.Value)
	}
}

func TestNewOrderRating_AllValidRatingValues(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	validValues := []int{1, 2, 3, 4, 5}

	for _, value := range validValues {
		rating, err := NewOrderRating(orderID, buyerID, sellerID, value, nil)
		if err != nil {
			t.Errorf("NewOrderRating() with value %d returned error: %v", value, err)
		}
		if rating.RatingValue != value {
			t.Errorf("RatingValue = %v, want %v", rating.RatingValue, value)
		}
	}
}

func TestValidateRatingValue_Valid(t *testing.T) {
	validValues := []int{1, 2, 3, 4, 5}

	for _, value := range validValues {
		err := ValidateRatingValue(value)
		if err != nil {
			t.Errorf("ValidateRatingValue(%d) returned error: %v", value, err)
		}
	}
}

func TestValidateRatingValue_Invalid(t *testing.T) {
	invalidValues := []int{-1, 0, 6, 10, 100}

	for _, value := range invalidValues {
		err := ValidateRatingValue(value)
		if err == nil {
			t.Errorf("ValidateRatingValue(%d) should return error", value)
		}
		_, ok := err.(*ErrInvalidRatingValue)
		if !ok {
			t.Errorf("Error type for value %d = %T, want *ErrInvalidRatingValue", value, err)
		}
	}
}

// Error message tests

func TestErrInvalidRatingValue_Error(t *testing.T) {
	err := &ErrInvalidRatingValue{Value: 0}
	expected := "invalid rating value: 0 (must be between 1 and 5)"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrOrderNotCompleted_Error(t *testing.T) {
	err := &ErrOrderNotCompleted{
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

	err := &ErrNotBuyer{OrderID: orderID, CallerID: callerID, BuyerID: buyerID}
	expected := "caller 00000000-0000-0000-0000-000000000002 is not the buyer of order 00000000-0000-0000-0000-000000000001 (buyer: 00000000-0000-0000-0000-000000000003)"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrAlreadyRated_Error(t *testing.T) {
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	err := &ErrAlreadyRated{OrderID: orderID}
	expected := "order 00000000-0000-0000-0000-000000000001 already rated"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestErrOrderNotFound_Error(t *testing.T) {
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	err := &ErrOrderNotFound{OrderID: orderID}
	expected := "order not found: 00000000-0000-0000-0000-000000000001"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

// Helper function

func strPtr(s string) *string {
	return &s
}


