package worker

import (
	"context"
	"time"

	"github.com/labuda/backend/pkg/db"
)

const (
	// maxWorkerLoopDuration prevents a single worker loop from running too long.
	// If processing exceeds this limit, the loop breaks and continues on the next poll cycle.
	maxWorkerLoopDuration = 2 * time.Second
)

// OutboxInserter defines the interface for emitting outbox events.
// This is a minimal interface to avoid circular dependencies.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}


