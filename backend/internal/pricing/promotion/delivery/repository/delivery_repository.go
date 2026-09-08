// Package repository defines the persistence surface for the canonical
// Delivery Ticket and Qualified Impression aggregates.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	"github.com/labuda/backend/pkg/db"
)

// ErrTicketNotFound is returned when no delivery ticket row matches the
// requested id.
var ErrTicketNotFound = errors.New("promotion delivery ticket not found")

// Repository persists delivery tickets and qualified impressions. All methods
// run inside the caller-provided transaction so ticket/QI state transitions
// stay atomic with their ledger movements.
type Repository interface {
	// GetDBTime returns the database clock. All lifecycle-sensitive time
	// (issued_at, expires_at, server_occurred_at, expiry checks) MUST use the
	// DB clock, never the application clock.
	GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error)

	// CreateTicket inserts an issued ticket. It performs NO ledger movement —
	// issuance is authorization/state only (Model A money rule).
	CreateTicket(ctx context.Context, tx db.Tx, ticket *entity.DeliveryTicket) error

	// GetByID reads a ticket without locking. Used only to discover the
	// contract id before the canonical contract-first lock order; all
	// validation uses the FOR UPDATE re-read.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.DeliveryTicket, error)

	// ReadTicket reads a ticket through the pool, OUTSIDE any transaction.
	// It exists ONLY for the qualification pre-flight target-eligibility
	// gate: the canonical OperabilityChecker reads through its own pool
	// connection, and acquiring a second pool connection while a transaction
	// holds row locks can exhaust a small pool and deadlock concurrent
	// qualifications. The gate therefore runs before the tx. Money validity
	// is NEVER decided by this read — the qualification tx re-reads the
	// ticket FOR UPDATE and re-validates everything that moves money.
	ReadTicket(ctx context.Context, id uuid.UUID) (*entity.DeliveryTicket, error)

	// GetForUpdate reads a ticket with FOR UPDATE. All qualification
	// validation reads the locked row.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.DeliveryTicket, error)

	// MarkConsumed transitions a ticket to 'consumed' with the given DB time.
	// Called in the same transaction that created its Qualified Impression.
	MarkConsumed(ctx context.Context, tx db.Tx, id uuid.UUID, consumedAt time.Time) error

	// InvalidateIssuedForContract transitions every outstanding 'issued'
	// ticket of a contract to 'invalidated'. Called from canonical
	// finalization so a finalized contract can never let an old ticket charge.
	InvalidateIssuedForContract(ctx context.Context, tx db.Tx, contractID uuid.UUID) error

	// CountQualifiedImpressions returns the number of qualified impressions
	// already persisted for the contract. Callers MUST hold the contract row
	// lock (FOR UPDATE) so the count is stable; the next sequence N is
	// count+1. UNIQUE(contract_id, sequence_n) is the DB backstop.
	CountQualifiedImpressions(ctx context.Context, tx db.Tx, contractID uuid.UUID) (int64, error)

	// CreateQualifiedImpression inserts the immutable billable fact. The
	// UNIQUE(ticket_id) constraint makes one-ticket -> one-QI a DB-level
	// guarantee.
	CreateQualifiedImpression(ctx context.Context, tx db.Tx, qi *entity.QualifiedImpression) error
}
