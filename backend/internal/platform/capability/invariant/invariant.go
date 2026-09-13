// Package invariant enforces the canonical full-access admin invariant:
// the system must always retain at least one full-access admin.
//
// WHY THIS IS A DATABASE CONCERN
//
// Full access is derived (admin role + coverage of the entire canonical
// capability universe), so no stored row can be checked. The invariant must
// therefore be evaluated against the live rows that a mutation is about to
// change, inside the same transaction, with the check and the mutation
// serialized. A read-then-write check on the pool would be racy: two
// concurrent demotions could each observe "one other full-access admin
// remains" and together reduce the count to zero.
//
// The guarantee here is:
//
//	Lock  → serializes every mutation that can reduce the count
//	mutate
//	Verify → recomputes the count from the transaction's own visible state
//
// If Verify fails, the caller's transaction rolls back, releasing the lock.
package invariant

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/platform/capability"
)

// ErrLastFullAccessAdmin is returned when a mutation would leave the system
// with zero full-access admins.
var ErrLastFullAccessAdmin = errors.New(
	"cannot apply change: it would remove the last full-access admin; " +
		"grant full access to another admin first",
)

// advisoryLockKey is the stable transaction-scoped lock key that serializes all
// full-access-reducing mutations. Never change this value while transactions
// may be in flight: changing it silently weakens the guarantee.
const advisoryLockKey int64 = 9021051001

// Querier is the minimal database surface required by the invariant guard.
// Both *pgxpool.Pool and db.Tx satisfy it.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Lock acquires the transaction-scoped advisory lock that serializes every
// mutation capable of reducing the full-access admin count.
//
// MUST be called inside a transaction, before the mutation.
func Lock(ctx context.Context, q Querier) error {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockKey); err != nil {
		return fmt.Errorf("acquire full-access invariant lock: %w", err)
	}
	return nil
}

// Count returns the number of live full-access admins visible to the current
// transaction. A full-access admin is an active, non-deleted user whose role is
// "admin" and whose active capability set covers the entire canonical universe.
func Count(ctx context.Context, q Querier) (int, error) {
	var count int
	err := q.QueryRow(ctx, `
		SELECT count(*)
		FROM users u
		WHERE u.role = 'admin'
		  AND u.deleted_at IS NULL
		  AND u.account_status = 'active'
		  AND NOT EXISTS (
		      SELECT 1
		      FROM unnest($1::text[]) AS required(cap)
		      WHERE NOT EXISTS (
		          SELECT 1
		          FROM user_capabilities uc
		          WHERE uc.user_id = u.id
		            AND uc.capability = required.cap
		            AND uc.revoked_at IS NULL
		      )
		  )
	`, capability.AllCapabilityStrings()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count full-access admins: %w", err)
	}
	return count, nil
}

// Verify recomputes the invariant. Call it AFTER the mutation, inside the same
// transaction that took Lock. Returning ErrLastFullAccessAdmin rolls the
// caller's transaction back.
func Verify(ctx context.Context, q Querier) error {
	count, err := Count(ctx, q)
	if err != nil {
		return err
	}
	if count < 1 {
		return ErrLastFullAccessAdmin
	}
	return nil
}

// GuardSerialized runs Lock, mutateKeyCheck, then Verify — the canonical
// sequence for a full-access-reducing mutation.
func GuardSerialized(ctx context.Context, q Querier, mutate func() error) error {
	if err := Lock(ctx, q); err != nil {
		return err
	}
	if err := mutate(); err != nil {
		return err
	}
	return Verify(ctx, q)
}
