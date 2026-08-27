package presence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

const (
	RedisLeaseKeyPrefix    = "presence:v1:leases:"
	RedisDeadlineKey       = "presence:v1:deadlines"
	RedisStateKeyPrefix    = "presence:v1:state:"
	RedisEventsChannel     = "presence:v1:events"
	PresenceGracePeriod    = 90 * time.Second
	PresenceSubscribeCap   = 200
	PresenceWorkerInterval = 5 * time.Second
	PresenceRetryBackoff   = 5 * time.Second
	PresenceMaxClaimBatch  = 200
)

type State struct {
	UserID     uuid.UUID  `json:"user_id"`
	IsOnline   bool       `json:"is_online"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	Version    int64      `json:"version"`
}

type Event struct {
	Type  string `json:"type"`
	State State  `json:"state"`
}

type LeaseResult struct {
	UserID           uuid.UUID
	ActiveLeaseCount int64
	IsOnline         bool
	Transitioned     bool
	Version          int64
	NextDeadline     *time.Time
	LastSeenAt       *time.Time
	State            State
}

type ClaimedDueUser struct {
	UserID uuid.UUID
	DueAt  time.Time
}

type SubjectFacts struct {
	UserID             uuid.UUID
	AccountStatus      string
	Deleted            bool
	ShowActivityStatus bool
	LastSeenAt         *time.Time
	Version            int64
	IsOnline           bool
}

// LastSeenWriter persists durable last-seen timestamps.
type LastSeenWriter interface {
	UpsertLastSeen(ctx context.Context, tx db.Tx, userID uuid.UUID, occurredAt time.Time) error
}

// OutboxInserter is the minimal outbox surface needed for durable retries.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

type LastSeenRecordPayload struct {
	UserID     uuid.UUID `json:"user_id"`
	LastSeenAt string    `json:"last_seen_at"`
	Version    int64     `json:"version"`
}
