package entity

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewNegotiationMessage(t *testing.T) {
	sessionID := uuid.New()
	senderID := uuid.New()
	price := int64(10000)
	note := "I'll pay 100"

	message, err := NewNegotiationMessage(sessionID, senderID, price, note)

	if err != nil {
		t.Errorf("NewNegotiationMessage() unexpected error: %v", err)
	}

	if message.ID == uuid.Nil {
		t.Error("NewNegotiationMessage() should generate non-nil ID")
	}

	if message.SessionID != sessionID {
		t.Errorf("NewNegotiationMessage() SessionID = %s, want %s", message.SessionID, sessionID)
	}

	if message.SenderID != senderID {
		t.Errorf("NewNegotiationMessage() SenderID = %s, want %s", message.SenderID, senderID)
	}

	if message.Price != price {
		t.Errorf("NewNegotiationMessage() Price = %d, want %d", message.Price, price)
	}

	if message.Note != note {
		t.Errorf("NewNegotiationMessage() Note = %s, want %s", message.Note, note)
	}

	if message.CreatedAt == 0 {
		t.Error("NewNegotiationMessage() CreatedAt should be non-zero")
	}
}

func TestNewNegotiationMessageInvalidPrice(t *testing.T) {
	sessionID := uuid.New()
	senderID := uuid.New()

	tests := []struct {
		name  string
		price int64
	}{
		{"zero price", 0},
		{"negative price", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := NewNegotiationMessage(sessionID, senderID, tt.price, "test")

			if err == nil {
				t.Error("NewNegotiationMessage() should return error for invalid price")
			}

			if message != nil {
				t.Error("NewNegotiationMessage() should return nil message for invalid price")
			}

			var invalidPriceErr *InvalidPriceError
			if err == nil || err.(*InvalidPriceError) == nil {
				_, ok := err.(*InvalidPriceError)
				if !ok {
					t.Errorf("NewNegotiationMessage() should return InvalidPriceError, got %T", err)
				}
			} else {
				invalidPriceErr = err.(*InvalidPriceError)
				if invalidPriceErr.Price != tt.price {
					t.Errorf("InvalidPriceError.Price = %d, want %d", invalidPriceErr.Price, tt.price)
				}
			}
		})
	}
}

func TestInvalidPriceError(t *testing.T) {
	err := &InvalidPriceError{Price: -100}

	expected := "invalid price for negotiation: session_id=00000000-0000-0000-0000-000000000000, price=-100, reason="
	if err.Error() != expected {
		t.Errorf("InvalidPriceError.Error() = %s, want %s", err.Error(), expected)
	}
}

func TestNegotiationMessageImmutability(t *testing.T) {
	// Messages are immutable by design - once created, they cannot be modified
	// This test documents that behavior
	sessionID := uuid.New()
	senderID := uuid.New()
	originalPrice := int64(10000)

	message, err := NewNegotiationMessage(sessionID, senderID, originalPrice, "test")
	if err != nil {
		t.Fatalf("NewNegotiationMessage() unexpected error: %v", err)
	}

	// Verify message has all fields set
	if message.Price != originalPrice {
		t.Errorf("Message.Price = %d, want %d", message.Price, originalPrice)
	}

	// Messages have no modification methods - they are immutable
	// Any changes require creating a new message
}


