package entity

import (
	"testing"
	"time"
)

func TestShippingQuote_IsExpiredAt(t *testing.T) {
	expiresAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	q := &ShippingQuote{ExpiresAt: &expiresAt}

	t.Run("one instant before expiry is valid", func(t *testing.T) {
		now := expiresAt.Add(-1 * time.Nanosecond)
		if got := q.IsExpiredAt(now); got {
			t.Fatalf("IsExpiredAt(before) = %v, want false", got)
		}
	})

	t.Run("exact expiry equality is expired", func(t *testing.T) {
		if got := q.IsExpiredAt(expiresAt); !got {
			t.Fatalf("IsExpiredAt(exact) = %v, want true", got)
		}
	})

	t.Run("one instant after expiry is expired", func(t *testing.T) {
		now := expiresAt.Add(time.Nanosecond)
		if got := q.IsExpiredAt(now); !got {
			t.Fatalf("IsExpiredAt(after) = %v, want true", got)
		}
	})
}

func TestShippingQuote_NilExpiryIsInvalid(t *testing.T) {
	q := &ShippingQuote{}
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)

	if q.IsCurrent() {
		t.Fatal("nil-expiry quote must not be current")
	}
	if q.IsBuyerUsableAt(now) {
		t.Fatal("nil-expiry quote must not be buyer-usable")
	}
	if !q.IsExpiredAt(now) {
		t.Fatal("nil-expiry quote should be treated as expired/invalid")
	}
}
