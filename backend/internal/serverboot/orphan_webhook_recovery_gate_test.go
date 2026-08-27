package serverboot

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestOrphanWebhookRecoveryEnabled_DefaultsOff(t *testing.T) {
	t.Setenv("DISABLE_ORPHAN_WEBHOOK_RECOVERY_WORKER", "")
	assertFalse(t, orphanWebhookRecoveryEnabled(zap.NewNop()))
}

func TestOrphanWebhookRecoveryEnabled_EnvFalseEnables(t *testing.T) {
	t.Setenv("DISABLE_ORPHAN_WEBHOOK_RECOVERY_WORKER", "false")
	assertTrue(t, orphanWebhookRecoveryEnabled(zap.NewNop()))
}

func TestRegisterOrphanWebhookRecoveryWorkerStartup_DisabledDoesNotRegister(t *testing.T) {
	startups := make([]func(), 0)
	registerOrphanWebhookRecoveryWorkerStartup(
		context.Background(),
		&startups,
		false,
		func(context.Context) {
			t.Fatal("start should not be called when disabled")
		},
		zap.NewNop(),
	)

	if len(startups) != 0 {
		t.Fatalf("expected no startup closures when disabled, got %d", len(startups))
	}
}

func TestRegisterOrphanWebhookRecoveryWorkerStartup_EnabledUsesCancelableContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{}, 1)
	exited := make(chan struct{}, 1)
	startups := make([]func(), 0)

	registerOrphanWebhookRecoveryWorkerStartup(
		ctx,
		&startups,
		true,
		func(startCtx context.Context) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-startCtx.Done()
			exited <- struct{}{}
		},
		zap.NewNop(),
	)

	if len(startups) != 1 {
		t.Fatalf("expected one startup closure when enabled, got %d", len(startups))
	}

	startups[0]()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("startup closure did not launch worker goroutine")
	}

	cancel()

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("worker goroutine did not exit after context cancellation")
	}
}

func assertTrue(t *testing.T, got bool) {
	t.Helper()
	if !got {
		t.Fatalf("expected true")
	}
}

func assertFalse(t *testing.T, got bool) {
	t.Helper()
	if got {
		t.Fatalf("expected false")
	}
}


