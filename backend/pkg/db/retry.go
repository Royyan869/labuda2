package db

import (
	"context"
	"fmt"
)

// withRetry executes fn within a transaction with retry on retryable errors.
//
// Critical guarantees:
// 1. Rollback is ALWAYS called if fn returns error
// 2. Rollback is ALWAYS called if commit fails (before retry)
// 3. Commit is never called twice
// 4. Old transaction is closed before any retry
func withRetry(ctx context.Context, db *DB, fn func(Tx) error, maxAttempts int) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		tx, err := db.BeginTx(ctx)
		if err != nil {
			// Failed to begin transaction - not retryable
			return fmt.Errorf("begin tx (attempt %d): %w", attempt+1, err)
		}

		// Execute the user function
		execErr := fn(tx)
		if execErr != nil {
			// CRITICAL: Always rollback on fn error
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				return fmt.Errorf("fn failed: %v, rollback failed: %w", execErr, rbErr)
			}

			// Check if error is retryable
			if IsRetryable(execErr) && attempt < maxAttempts-1 {
				lastErr = execErr
				continue // retry
			}

			// Not retryable or out of attempts
			return execErr
		}

		// fn succeeded, attempt to commit
		commitErr := tx.Commit(ctx)
		if commitErr != nil {
			// CRITICAL: Always rollback before retry to close the old transaction
			// Note: after failed commit, tx is already closed internally, so rollback may return ErrTxClosed
			_ = tx.Rollback(ctx) // Best effort cleanup, ignore error (even if ErrTxClosed)

			// Check if commit error is retryable
			if IsRetryable(commitErr) && attempt < maxAttempts-1 {
				lastErr = commitErr
				continue // retry with NEW transaction
			}

			// Not retryable or out of attempts
			return fmt.Errorf("commit tx (attempt %d): %w", attempt+1, commitErr)
		}

		// Success - transaction committed
		return nil
	}

	// Should not reach here if maxAttempts >= 1
	return fmt.Errorf("max retry attempts exceeded, last error: %w", lastErr)
}
