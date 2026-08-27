package application

import (
	"testing"

	"github.com/google/uuid"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/identity/auth"
)

func TestEnforceAppDisputeFinality_BlocksCompletedReleasedForAppCaller(t *testing.T) {
	svc := &DisputeService{}
	order := &orderEntity.Order{
		Status:       orderEntity.StatusCompleted,
		EscrowStatus: orderEntity.EscrowStatusReleased,
	}

	err := svc.enforceAppDisputeFinality(order, uuid.New())
	if err == nil {
		t.Fatal("expected finality guard to block app caller, got nil")
	}
	if got := err.Error(); got != "cannot open dispute after order completion; handle objections outside the app" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestEnforceAppDisputeFinality_BlocksSystemCaller(t *testing.T) {
	svc := &DisputeService{}
	order := &orderEntity.Order{
		Status:       orderEntity.StatusCompleted,
		EscrowStatus: orderEntity.EscrowStatusReleased,
	}

	err := svc.enforceAppDisputeFinality(order, auth.SystemCallerID)
	if err == nil {
		t.Fatal("expected system caller to be blocked by finality guard, got nil")
	}
	if got := err.Error(); got != "cannot open dispute after order completion; handle objections outside the app" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestEnforceAppDisputeFinality_AllowsPreReleaseAppCaller(t *testing.T) {
	svc := &DisputeService{}
	order := &orderEntity.Order{
		Status:       orderEntity.StatusShipped,
		EscrowStatus: orderEntity.EscrowStatusHolding,
	}

	if err := svc.enforceAppDisputeFinality(order, uuid.New()); err != nil {
		t.Fatalf("expected pre-release dispute to pass finality guard, got %v", err)
	}
}


