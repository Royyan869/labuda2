package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// REPEATED-EVENT PROOF: an event type that fires more than once on the same
// aggregate must carry a unique key per occurrence, otherwise ON CONFLICT
// (idempotency_key) DO NOTHING drops it silently. That is exactly what
// happened to negotiation.message_sent: every counter after the first left the
// session and price history advanced while the chat proposal never existed.
func TestInsertEventWithSuffix_KeyIsPerOccurrence(t *testing.T) {
	var gotKey string
	tx := &mockTx{
		execFunc: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			gotKey, _ = args[10].(string) // idempotency_key is the 11th column
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	repo := NewOutboxRepository(nil)
	sessionID := uuid.New()

	err := repo.InsertEventWithSuffix(
		context.Background(),
		tx,
		"negotiation.message_sent",
		sessionID,
		[]byte(`{}`),
		"3",
	)
	if err != nil {
		t.Fatalf("InsertEventWithSuffix() error: %v", err)
	}

	want := fmt.Sprintf("negotiation.message_sent.%s.3", sessionID)
	if gotKey != want {
		t.Fatalf("idempotency_key = %q, want %q", gotKey, want)
	}

	// The plain path must keep its own deterministic key shape.
	gotKey = ""
	if err := repo.InsertEvent(
		context.Background(),
		tx,
		"negotiation.started",
		sessionID,
		[]byte(`{}`),
	); err != nil {
		t.Fatalf("InsertEvent() error: %v", err)
	}
	want = fmt.Sprintf("negotiation.started.%s", sessionID)
	if gotKey != want {
		t.Fatalf("InsertEvent idempotency_key = %q, want %q", gotKey, want)
	}
}
