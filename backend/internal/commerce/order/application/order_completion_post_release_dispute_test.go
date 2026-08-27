package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestRefundFromDisputePostRelease_IsParked(t *testing.T) {
	svc := &OrderCompletionService{}

	err := svc.RefundFromDisputePostRelease(context.Background(), nil, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected parked post-release refund method to return an error, got nil")
	}
	if got := err.Error(); got != "post-release dispute refunds are disabled; handle objections outside the app" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestPartialRefundFromDisputePostRelease_IsParked(t *testing.T) {
	svc := &OrderCompletionService{}

	err := svc.PartialRefundFromDisputePostRelease(context.Background(), nil, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected parked post-release partial refund method to return an error, got nil")
	}
	if got := err.Error(); got != "post-release dispute refunds are disabled; handle objections outside the app" {
		t.Fatalf("unexpected error: %s", got)
	}
}


