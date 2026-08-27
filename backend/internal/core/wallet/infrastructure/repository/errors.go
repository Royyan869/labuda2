package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// isDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation.
// PostgreSQL error code 23505 = "unique_violation"
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// Check for pgx.Error with SQLState 23505 (unique_violation)
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// isNoRowsError checks if the error is a "no rows" error.
func isNoRowsError(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}


