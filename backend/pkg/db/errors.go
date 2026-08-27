package db

import (
	"errors"
	"strings"
)

var (
	// PgError is the interface for PostgreSQL errors.
	// We use errors.As to check for this interface.
	pgErrType interface{ Code() string }
)

func init() {
	// Initialize the type for errors.As
	// We'll use github.com/jackc/pgx/v5/pgconn.PgError
	// This is a lightweight way to get the type without importing pgconn directly
}

// IsUniqueViolation checks if an error is a PostgreSQL unique constraint violation (23505).
func IsUniqueViolation(err error) bool {
	var pgErr interface{ Code() string }
	if errors.As(err, &pgErr) {
		return pgErr.Code() == "23505"
	}
	return false
}

// IsSerializationFailure checks if an error is a serialization failure (40001).
// This occurs when a transaction fails due to serialization anomalies.
func IsSerializationFailure(err error) bool {
	var pgErr interface{ Code() string }
	if errors.As(err, &pgErr) {
		return pgErr.Code() == "40001"
	}
	return false
}

// IsDeadlock checks if an error is a deadlock detected (40P01).
func IsDeadlock(err error) bool {
	var pgErr interface{ Code() string }
	if errors.As(err, &pgErr) {
		return pgErr.Code() == "40P01"
	}
	return false
}

// IsCheckViolation checks if an error is a PostgreSQL CHECK constraint violation (23514).
// This occurs when a CHECK constraint fails during INSERT or UPDATE.
func IsCheckViolation(err error) bool {
	var pgErr interface{ Code() string }
	if errors.As(err, &pgErr) {
		return pgErr.Code() == "23514"
	}
	return false
}

// IsRetryable returns true if the error is retryable (serialization failure,
// deadlock, or an aborted-transaction commit).
//
// "commit unexpectedly resulted in rollback" (pgx.ErrTxCommitRollback) occurs
// when PostgreSQL aborts a transaction due to a concurrent write conflict and
// Commit() is then called. This is the commit-time surface of the same
// serialization/deadlock class and must be retried with a fresh transaction so
// a racing duplicate webhook converges to the winner's committed state instead
// of surfacing as an error.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if IsSerializationFailure(err) || IsDeadlock(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "commit unexpectedly resulted in rollback") ||
		strings.Contains(msg, "errtxcommitrollback") ||
		strings.Contains(msg, "current transaction is aborted")
}
