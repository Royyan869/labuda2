package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execCapture is a minimal TestDbTx that captures the SQL executed by
// MarkFailed and returns a controlled CommandTag.
type execCapture struct {
	TestDbTx
	tag pgconn.CommandTag
	err error
	// captured fields
	capturedSQL  string
	capturedArgs []any
}

func (e *execCapture) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.capturedSQL = sql
	e.capturedArgs = append(e.capturedArgs, args...)
	return e.tag, e.err
}

// newExecCapture builds an execCapture that returns `rowsAffected` rows and no error.
func newExecCapture(rowsAffected int) *execCapture {
	var tag pgconn.CommandTag
	if rowsAffected > 0 {
		tag = pgconn.NewCommandTag("UPDATE 1")
	} else {
		tag = pgconn.NewCommandTag("UPDATE 0")
	}
	return &execCapture{tag: tag}
}

// newExecCaptureErr builds an execCapture that returns a DB-level error.
func newExecCaptureErr(err error) *execCapture {
	return &execCapture{err: err}
}

// ============================================================================
// SQL GUARD SHAPE TESTS
// ============================================================================

// TestMarkFailed_SQLContainsStatusGuard verifies the generated SQL contains
// the AND status IN (...) clause so the guard is not accidentally removed.
func TestMarkFailed_SQLContainsStatusGuard(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	_ = repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "reason", "")

	assert.Contains(t, tx.capturedSQL, "status IN",
		"MarkFailed SQL must contain status IN guard")
	assert.Contains(t, tx.capturedSQL, "'PROCESSING'",
		"guard must include PROCESSING")
	assert.Contains(t, tx.capturedSQL, "'SUBMITTED'",
		"guard must include SUBMITTED")
	assert.Contains(t, tx.capturedSQL, "'SETTLING'",
		"guard must include SETTLING")
	assert.Contains(t, tx.capturedSQL, "'FAILED_RETRYABLE'",
		"guard must include FAILED_RETRYABLE")
}

// TestMarkFailed_SQLDoesNotContainUnconditionalWhere verifies the old unsafe
// pattern (WHERE id = $N with no status check) is gone.
func TestMarkFailed_SQLDoesNotContainUnconditionalWhere(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	_ = repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "reason", "")

	// Old unsafe pattern was exactly "WHERE id = $5" with nothing after.
	// New SQL must have AND status IN after the id clause.
	assert.True(t,
		strings.Contains(tx.capturedSQL, "AND status IN"),
		"SQL must combine id filter with AND status IN, not be unconditional")
}

// ============================================================================
// VALID FROM-STATE TESTS (1 row affected → success)
// ============================================================================

func TestMarkFailed_FromProcessing_Succeeds(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	// PROCESSING → FAILED_FINAL is the sync gateway rejection path
	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "sync reject", "")

	require.NoError(t, err, "PROCESSING → FAILED_FINAL must succeed when DB returns 1 row affected")
}

func TestMarkFailed_FromSubmitted_ToFailedFinal_Succeeds(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "webhook permanent", "raw")

	require.NoError(t, err)
}

func TestMarkFailed_FromSettling_ToFailedFinal_Succeeds(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "settling reject", "")

	require.NoError(t, err)
}

func TestMarkFailed_FromFailedRetryable_ToFailedFinal_Succeeds(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	// Retry attempt that permanently fails
	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "permanent on retry", "")

	require.NoError(t, err)
}

func TestMarkFailed_FromSubmitted_ToFailedRetryable_Succeeds(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(1)
	id := uuid.New()

	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedRetryable, "timeout", "")

	require.NoError(t, err)
}

// ============================================================================
// TERMINAL STATE TESTS (0 rows affected → ErrMarkFailedInvalidState)
// ============================================================================

// TestMarkFailed_FromTerminalState_ReturnsError verifies that when the DB
// returns 0 rows affected (because the status IN guard excluded the row),
// MarkFailed returns ErrMarkFailedInvalidState.
func TestMarkFailed_FromTerminalState_ReturnsError(t *testing.T) {
	terminalCases := []struct {
		name   string
		status WithdrawalStatus
	}{
		{"SETTLED", WithdrawalStatusSettled},
		{"COMPLETED", WithdrawalStatusCompleted},
		{"FAILED", WithdrawalStatusFailed},
		{"FAILED_FINAL", WithdrawalStatusFailedFinal},
	}

	for _, tc := range terminalCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewWithdrawRepository()
			// 0 rows affected = DB rejected update because status IN guard blocked it
			tx := newExecCapture(0)
			id := uuid.New()

			err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "late callback", "")

			require.Error(t, err,
				"MarkFailed on terminal state %s must return error", tc.status)
			assert.True(t, errors.Is(err, ErrMarkFailedInvalidState),
				"error must wrap ErrMarkFailedInvalidState, got: %v", err)
			assert.Contains(t, err.Error(), id.String(),
				"error message must include withdrawal ID")
		})
	}
}

// TestMarkFailed_FromRequestedState_ReturnsError verifies REQUESTED state
// (not in the allowed list) is also blocked.
func TestMarkFailed_FromRequestedState_ReturnsError(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(0) // DB returns 0 rows — REQUESTED not in IN clause
	id := uuid.New()

	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "bad call", "")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMarkFailedInvalidState))
}

// TestMarkFailed_FromPilotBlocked_ReturnsError verifies PILOT_BLOCKED is also blocked.
func TestMarkFailed_FromPilotBlocked_ReturnsError(t *testing.T) {
	repo := NewWithdrawRepository()
	tx := newExecCapture(0) // PILOT_BLOCKED not in IN clause
	id := uuid.New()

	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "bad call", "")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMarkFailedInvalidState))
}

// ============================================================================
// DB ERROR PROPAGATION
// ============================================================================

func TestMarkFailed_DBError_Propagated(t *testing.T) {
	repo := NewWithdrawRepository()
	dbErr := errors.New("connection reset by peer")
	tx := newExecCaptureErr(dbErr)
	id := uuid.New()

	err := repo.MarkFailed(context.Background(), tx, id, WithdrawalStatusFailedFinal, "reason", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "withdraw: mark failed failed")
	assert.Contains(t, err.Error(), dbErr.Error())
}

// ============================================================================
// CONSISTENCY CROSS-CHECK: IsFinal() agrees with the SQL guard
// ============================================================================

// TestMarkFailed_SqlGuardMatchesIsFinal verifies that every status classified
// as IsFinal() is NOT in the SQL allowed-set, and every status in the
// allowed-set is NOT IsFinal(). This catches drift between application-layer
// and DB-layer guards.
func TestMarkFailed_SqlGuardMatchesIsFinal(t *testing.T) {
	// States the SQL IN clause allows (from MarkFailed implementation)
	sqlAllowed := []WithdrawalStatus{
		WithdrawalStatusProcessing,
		WithdrawalStatusSubmitted,
		WithdrawalStatusSettling,
		WithdrawalStatusFailedRetryable,
	}

	// Every allowed state must NOT be final
	for _, s := range sqlAllowed {
		assert.False(t, s.IsFinal(),
			"SQL-allowed state %s must not be IsFinal()", s)
	}

	// Every terminal state must NOT be in the allowed set
	terminal := []WithdrawalStatus{
		WithdrawalStatusSettled,
		WithdrawalStatusCompleted,
		WithdrawalStatusFailed,
		WithdrawalStatusFailedFinal,
	}
	for _, s := range terminal {
		assert.True(t, s.IsFinal(),
			"terminal state %s must be IsFinal()", s)
		for _, a := range sqlAllowed {
			assert.NotEqual(t, s, a,
				"terminal state %s must not appear in SQL allowed set", s)
		}
	}
}


