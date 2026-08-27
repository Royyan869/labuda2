//go:build integration

// ============================================================================
// COIN RESERVATION CONCURRENCY + STATE-MACHINE PROOF (SCOPE 4B-S2B1-V)
// ============================================================================
//
// Real PostgreSQL proof for MODEL R reservation authority.
//
// PROOFS:
//   Part A — Basic reservation, concurrent oversubscription, exact capacity
//   Part C — Terminal state machine (all transitions, opposite-terminal errors)
//   Part D — Release proof (no balance change, no refund txn)
//   Part E — Consume-state proof (state-machine only)
//   Part F — Available balance read proof
//   Part G — Uniqueness / constraint proof (lifetime uniqueness)
//
// INVARIANTS:
//   TotalUnspentCoins = user_coin_balance.balance
//   ReservedCoins     = SUM(amount) WHERE status = 'reserved'
//   AvailableCoins    = TotalUnspentCoins - ReservedCoins
//   Reserved → Consumed OR Reserved → Released, NEVER both
//   Same-terminal replay = idempotent
//   Opposite-terminal transition = typed error
// ============================================================================

package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coinsentity "github.com/labuda/backend/internal/incentive/coins/entity"
	coinsrepopg "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	coinsrepo "github.com/labuda/backend/internal/incentive/coins/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// FIXTURES
// ============================================================================

func insertCoinTestUser(t *testing.T, ctx context.Context, testDB *testdb.TestDB, userID uuid.UUID) {
	t.Helper()
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user') ON CONFLICT (id) DO NOTHING`,
			userID, "fb-"+userID.String()[:8], userID.String()+"@test.local")
		return err
	})
	require.NoError(t, err, "user fixture failed")
}

func insertCoinTestPayment(t *testing.T, ctx context.Context, testDB *testdb.TestDB, paymentID, userID uuid.UUID) {
	t.Helper()
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO payments (id, user_id, payment_number, midtrans_order_id,
				gross_amount, status, reference_type, reference_id, expired_at)
			VALUES ($1, $2, $3, $4, 100000, 'pending', 'order', $5, NOW() + INTERVAL '1 hour')
			ON CONFLICT (id) DO NOTHING`,
			paymentID, userID, "PAY-TEST-"+paymentID.String()[:8],
			"LAB-TEST-"+paymentID.String()[:8],
			paymentID,
		)
		return err
	})
	require.NoError(t, err, "payment fixture failed")
}

func setupReservationTest(t *testing.T) (*testdb.TestDB, coinsrepo.CoinsRepository, uuid.UUID, func()) {
	t.Helper()
	testDB, cleanup := testdb.SetupDB(t)
	repo := coinsrepopg.NewCoinsRepository()
	userID := uuid.New()
	ctx := context.Background()

	insertCoinTestUser(t, ctx, testDB, userID)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.EnsureBalanceRow(ctx, tx, userID)
	})
	require.NoError(t, err)

	return testDB, repo, userID, cleanup
}

func seedReservationBalance(t *testing.T, testDB *testdb.TestDB, repo coinsrepo.CoinsRepository, userID uuid.UUID, amount int64) {
	t.Helper()
	err := testDB.WithTx(context.Background(), func(tx db.Tx) error {
		if err := repo.EnsureBalanceRow(context.Background(), tx, userID); err != nil {
			return err
		}
		_, err := repo.AtomicAddBalance(context.Background(), tx, userID, amount)
		return err
	})
	require.NoError(t, err)
}

func newPaymentWithFixture(t *testing.T, testDB *testdb.TestDB, userID uuid.UUID) uuid.UUID {
	t.Helper()
	paymentID := uuid.New()
	insertCoinTestPayment(t, context.Background(), testDB, paymentID, userID)
	return paymentID
}

// ============================================================================
// PART A+B: BASIC RESERVATION + CONCURRENT OVERSUBSCRIPTION
// ============================================================================

// Test 1: Basic reservation — total unchanged, reserved correct, available correct, zero spend txns
func TestReservationBasic(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		require.NotNil(t, balanceRow)
		assert.Equal(t, int64(20000), balanceRow.Balance)

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), reserved)

		available := balanceRow.Balance - reserved
		assert.Equal(t, int64(20000), available)

		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 15000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRow(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), balanceRow.Balance, "total balance unchanged")

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(15000), reserved)

		available := balanceRow.Balance - reserved
		assert.Equal(t, int64(5000), available)

		activeBalance, err := repo.GetActiveBalance(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), activeBalance, "zero spend transactions")

		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 1: basic reservation — total=20000 reserved=15000 available=5000 zero_spend")
}

// Test 2: Concurrent oversubscription — exactly ONE wins, never 30,000 reserved
func TestReservationConcurrentSameUser(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)

	paymentA := newPaymentWithFixture(t, testDB, userID)
	paymentB := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	var successA, successB bool
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a barrier so both goroutines start their DB txs at approximately the same time
	barrier := make(chan struct{})
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-barrier
		err := testDB.WithTx(ctx, func(tx db.Tx) error {
			balanceRow, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
			if err != nil || balanceRow == nil {
				return fmt.Errorf("lock failed: %w", err)
			}
			reserved, err := repo.SumActiveReservations(ctx, tx, userID)
			if err != nil {
				return err
			}
			available := balanceRow.Balance - reserved
			if available < 15000 {
				return fmt.Errorf("insufficient available: %d < 15000", available)
			}
			reservation, err := coinsentity.NewCoinReservation(paymentA, userID, 15000, expiresAt)
			if err != nil {
				return err
			}
			return repo.CreateReservation(ctx, tx, reservation)
		})
		mu.Lock()
		successA = (err == nil)
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		<-barrier
		err := testDB.WithTx(ctx, func(tx db.Tx) error {
			balanceRow, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
			if err != nil || balanceRow == nil {
				return fmt.Errorf("lock failed: %w", err)
			}
			reserved, err := repo.SumActiveReservations(ctx, tx, userID)
			if err != nil {
				return err
			}
			available := balanceRow.Balance - reserved
			if available < 15000 {
				return fmt.Errorf("insufficient available: %d < 15000", available)
			}
			reservation, err := coinsentity.NewCoinReservation(paymentB, userID, 15000, expiresAt)
			if err != nil {
				return err
			}
			return repo.CreateReservation(ctx, tx, reservation)
		})
		mu.Lock()
		successB = (err == nil)
		mu.Unlock()
	}()

	// Release barrier — both goroutines start competing for the FOR UPDATE lock
	close(barrier)
	wg.Wait()

	require.True(t, successA != successB,
		"exactly one must succeed: A=%v B=%v", successA, successB)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRow(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), balanceRow.Balance, "total unchanged")

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(15000), reserved, "15,000 not 30,000")

		activeBalance, err := repo.GetActiveBalance(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), activeBalance, "zero spend txns")

		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 2: concurrent same-user — one won, total=20000 reserved=15000 zero_spend")
}

// Test 3: Exact capacity — both succeed, reserved=20000, available=0
func TestReservationExactCapacity(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)

	paymentA := newPaymentWithFixture(t, testDB, userID)
	paymentB := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentA, userID, 12000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		require.Equal(t, int64(20000), balanceRow.Balance)

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(12000), reserved)

		available := balanceRow.Balance - reserved
		assert.Equal(t, int64(8000), available)

		reservation, err := coinsentity.NewCoinReservation(paymentB, userID, 8000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRow(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), balanceRow.Balance)

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), reserved)

		available := balanceRow.Balance - reserved
		assert.Equal(t, int64(0), available)

		activeBalance, err := repo.GetActiveBalance(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), activeBalance)

		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 3: exact capacity — reserved=20000 available=0 total=20000 zero_spend")
}

// Test 4: Duplicate same payment — one row, retry rejected
func TestReservationDuplicateSamePayment(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, err)
		err = repo.CreateReservation(ctx, tx, reservation)
		require.True(t, coinsrepo.IsReservationDuplicate(err), "must be ErrReservationDuplicate, got: %v", err)
		return err
	})
	require.Error(t, err, "duplicate must fail")

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), reserved, "only one reservation")

		res, err := repo.GetReservationByPaymentID(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, coinsentity.CoinReservationStatusReserved, res.Status)
		assert.Equal(t, int64(10000), res.Amount)
		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 4: duplicate rejected, one row, K=10000 preserved")
}

// Test 5: Conflicting retry — original K preserved, no silent mutation
func TestReservationConflictingRetry(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		existing, err := repo.GetReservationByPaymentID(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, existing)
		assert.Equal(t, int64(10000), existing.Amount)

		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 8000, expiresAt)
		require.NoError(t, err)
		err = repo.CreateReservation(ctx, tx, reservation)
		require.True(t, coinsrepo.IsReservationDuplicate(err), "must be ErrReservationDuplicate")
		return err
	})
	require.Error(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.GetReservationByPaymentID(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, int64(10000), res.Amount, "K preserved, not mutated")
		assert.Equal(t, coinsentity.CoinReservationStatusReserved, res.Status)
		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 5: conflicting retry rejected, K=10000 immutable")
}

// ============================================================================
// PART D: RELEASE PROOF
// ============================================================================

// Test 6: Release — total unchanged, active=0, available restored, no refund tx
func TestReservationRelease(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Create
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 15000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	// VERIFY before release
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, _ := repo.GetBalanceRow(ctx, tx, userID)
		reserved, _ := repo.SumActiveReservations(ctx, tx, userID)
		assert.Equal(t, int64(20000), balanceRow.Balance, "total before release")
		assert.Equal(t, int64(15000), reserved, "reserved before release")
		assert.Equal(t, int64(5000), balanceRow.Balance-reserved, "available before release")
		return nil
	})
	require.NoError(t, err)

	// Release
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.ReleaseReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, coinsentity.CoinReservationStatusReleased, res.Status)
		assert.NotNil(t, res.ReleasedAt)
		return nil
	})
	require.NoError(t, err)

	// VERIFY after release
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRow(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), balanceRow.Balance, "total unchanged after release")

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), reserved, "active reserved=0 after release")

		available := balanceRow.Balance - reserved
		assert.Equal(t, int64(20000), available, "available=20000 after release")

		activeBalance, err := repo.GetActiveBalance(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), activeBalance, "zero transactions (no refund_earn)")

		return nil
	})
	require.NoError(t, err)

	// Duplicate release — idempotent, same-terminal
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.ReleaseReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		assert.Nil(t, res, "duplicate release returns nil (idempotent same-terminal)")
		return nil
	})
	require.NoError(t, err)

	// VERIFY no monetary change after duplicate release
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, _ := repo.GetBalanceRow(ctx, tx, userID)
		reserved, _ := repo.SumActiveReservations(ctx, tx, userID)
		assert.Equal(t, int64(20000), balanceRow.Balance)
		assert.Equal(t, int64(0), reserved)
		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 6: release restores availability, total unchanged, no refund tx, dup idempotent")
}

// ============================================================================
// PART C+E: TERMINAL STATE MACHINE + CONSUME-STATE PROOF
// ============================================================================

// Test 7: Consume state transition + opposite-terminal rejection
func TestReservationConsumeState(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Create
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.NoError(t, err)

	// Consume: reserved → consumed
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.ConsumeReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, coinsentity.CoinReservationStatusConsumed, res.Status)
		assert.NotNil(t, res.ConsumedAt)
		return nil
	})
	require.NoError(t, err)

	// VERIFY: consumed not in active reservations, total unchanged
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, err := repo.GetBalanceRow(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), balanceRow.Balance, "total unchanged")

		reserved, err := repo.SumActiveReservations(ctx, tx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), reserved, "consumed NOT active")

		res, err := repo.GetReservationByPaymentID(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, coinsentity.CoinReservationStatusConsumed, res.Status)

		return nil
	})
	require.NoError(t, err)

	// Same-terminal: consume(consumed) → idempotent, returns nil
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.ConsumeReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		assert.Nil(t, res, "duplicate consume returns nil (same-terminal idempotent)")
		return nil
	})
	require.NoError(t, err)

	// Opposite-terminal: release(consumed) → HARD FAILURE
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.ReleaseReservation(ctx, tx, paymentID)
		require.Error(t, err, "release(consumed) must return error")
		assert.Nil(t, res, "release(consumed) must not return reservation")
		var alreadyConsumed *coinsentity.ErrReservationAlreadyConsumed
		assert.ErrorAs(t, err, &alreadyConsumed, "must be ErrReservationAlreadyConsumed")
		assert.Equal(t, paymentID, alreadyConsumed.PaymentID)
		return nil // Don't propagate the expected error
	})
	require.NoError(t, err)

	t.Logf("PASS Test 7: consume state OK, same-terminal idempotent, opposite-terminal blocked with typed error")
}

// Test 8: Opposite-terminal — consume after release returns typed error
func TestReservationConsumeAfterReleaseBlocked(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Create + Release
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		reservation, err := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, err)
		require.NoError(t, repo.CreateReservation(ctx, tx, reservation))

		res, err := repo.ReleaseReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, coinsentity.CoinReservationStatusReleased, res.Status)
		return nil
	})
	require.NoError(t, err)

	// Opposite-terminal: consume(released) → HARD FAILURE
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		res, err := repo.ConsumeReservation(ctx, tx, paymentID)
		require.Error(t, err, "consume(released) must return error")
		assert.Nil(t, res)
		var alreadyReleased *coinsentity.ErrReservationAlreadyReleased
		assert.ErrorAs(t, err, &alreadyReleased, "must be ErrReservationAlreadyReleased")
		assert.Equal(t, paymentID, alreadyReleased.PaymentID)
		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 8: consume(released) blocked with typed ErrReservationAlreadyReleased")
}

// ============================================================================
// PART F: AVAILABLE BALANCE READ PROOF
// ============================================================================

// Test 9: GetAvailableCoins with multiple reservations, release, consume
func TestAvailableBalanceReadProof(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 50000)
	paymentA := newPaymentWithFixture(t, testDB, userID)
	paymentB := newPaymentWithFixture(t, testDB, userID)
	paymentC := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Verify initial available = total
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		reserved, _ := repo.SumActiveReservations(ctx, tx, userID)
		assert.Equal(t, int64(0), reserved)
		return nil
	})
	require.NoError(t, err)

	// Reserve A=10000, B=15000 → available=25000
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		r, _ := coinsentity.NewCoinReservation(paymentA, userID, 10000, expiresAt)
		require.NoError(t, repo.CreateReservation(ctx, tx, r))
		return nil
	})
	require.NoError(t, err)
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		r, _ := coinsentity.NewCoinReservation(paymentB, userID, 15000, expiresAt)
		require.NoError(t, repo.CreateReservation(ctx, tx, r))
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, _ := repo.GetBalanceRow(ctx, tx, userID)
		reserved, _ := repo.SumActiveReservations(ctx, tx, userID)
		assert.Equal(t, int64(50000), balanceRow.Balance, "total=50000")
		assert.Equal(t, int64(25000), reserved, "reserved=25000")
		assert.Equal(t, int64(25000), balanceRow.Balance-reserved, "available=25000")
		return nil
	})
	require.NoError(t, err)

	// Release A → available=35000
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.ReleaseReservation(ctx, tx, paymentA)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, _ := repo.GetBalanceRow(ctx, tx, userID)
		reserved, _ := repo.SumActiveReservations(ctx, tx, userID)
		assert.Equal(t, int64(50000), balanceRow.Balance, "total unchanged after release")
		assert.Equal(t, int64(15000), reserved, "only B=15000 active after A released")
		assert.Equal(t, int64(35000), balanceRow.Balance-reserved, "available=35000 after release")
		return nil
	})
	require.NoError(t, err)

	// Consume B → available=50000 (consumed doesn't count as active)
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.ConsumeReservation(ctx, tx, paymentB)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		balanceRow, _ := repo.GetBalanceRow(ctx, tx, userID)
		reserved, _ := repo.SumActiveReservations(ctx, tx, userID)
		assert.Equal(t, int64(50000), balanceRow.Balance, "total unchanged after consume")
		assert.Equal(t, int64(0), reserved, "no active reservations after all released/consumed")
		assert.Equal(t, int64(50000), balanceRow.Balance-reserved, "available=50000 full")
		return nil
	})
	require.NoError(t, err)

	// Release and Consume do NOT show in active reservations
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		resA, _ := repo.GetReservationByPaymentID(ctx, tx, paymentA)
		resB, _ := repo.GetReservationByPaymentID(ctx, tx, paymentB)
		resC := paymentC // unused
		_ = resC
		require.NotNil(t, resA)
		require.NotNil(t, resB)
		assert.Equal(t, coinsentity.CoinReservationStatusReleased, resA.Status, "A is released")
		assert.Equal(t, coinsentity.CoinReservationStatusConsumed, resB.Status, "B is consumed")
		// Both are terminal, neither in SumActiveReservations
		return nil
	})
	require.NoError(t, err)

	t.Logf("PASS Test 9: available balance — only status='reserved' contributes to ReservedCoins")
}

// ============================================================================
// PART G: LIFETIME UNIQUENESS PROOF
// ============================================================================

// Test 10: No second reservation after release (lifetime UNIQUE on payment_id)
func TestReservationLifetimeUniquenessAfterRelease(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Create + Release
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		r, _ := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, repo.CreateReservation(ctx, tx, r))
		_, err = repo.ReleaseReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Attempt second reservation for same payment — must FAIL (UNIQUE on payment_id)
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		r, _ := coinsentity.NewCoinReservation(paymentID, userID, 5000, expiresAt)
		err = repo.CreateReservation(ctx, tx, r)
		require.True(t, coinsrepo.IsReservationDuplicate(err),
			"second reservation after release must fail: got %v", err)
		return err
	})
	require.Error(t, err, "lifetime uniqueness: no second reservation after release")

	t.Logf("PASS Test 10: no second reservation after release (lifetime UNIQUE on payment_id)")
}

// Test 11: No second reservation after consume (lifetime UNIQUE on payment_id)
func TestReservationLifetimeUniquenessAfterConsume(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	paymentID := newPaymentWithFixture(t, testDB, userID)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Create + Consume
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		r, _ := coinsentity.NewCoinReservation(paymentID, userID, 10000, expiresAt)
		require.NoError(t, repo.CreateReservation(ctx, tx, r))
		_, err = repo.ConsumeReservation(ctx, tx, paymentID)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Attempt second reservation for same payment — must FAIL
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
		require.NoError(t, err)
		r, _ := coinsentity.NewCoinReservation(paymentID, userID, 5000, expiresAt)
		err = repo.CreateReservation(ctx, tx, r)
		require.True(t, coinsrepo.IsReservationDuplicate(err),
			"second reservation after consume must fail: got %v", err)
		return err
	})
	require.Error(t, err, "lifetime uniqueness: no second reservation after consume")

	t.Logf("PASS Test 11: no second reservation after consume (lifetime UNIQUE on payment_id)")
}

// ============================================================================
// PART G: FULL TERMINAL-STATE TRANSITION MATRIX
// ============================================================================

// Test 12: Complete state transition matrix
func TestReservationFullTransitionMatrix(t *testing.T) {
	testDB, repo, userID, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	seedReservationBalance(t, testDB, repo, userID, 20000)
	expiresAt := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name          string
		setupState    coinsentity.CoinReservationStatus
		action        string // "consume" or "release"
		expectError   bool
		expectErrType string // e.g. "*entity.ErrReservationAlreadyConsumed"
	}{
		// Allowed transitions
		{"reserved→consumed", coinsentity.CoinReservationStatusReserved, "consume", false, ""},
		{"reserved→released", coinsentity.CoinReservationStatusReserved, "release", false, ""},

		// Same-terminal idempotent replay
		{"consumed→consume (idempotent)", coinsentity.CoinReservationStatusConsumed, "consume", false, ""},
		{"released→release (idempotent)", coinsentity.CoinReservationStatusReleased, "release", false, ""},

		// Opposite-terminal FORBIDDEN
		{"consumed→release (blocked)", coinsentity.CoinReservationStatusConsumed, "release", true, "*entity.ErrReservationAlreadyConsumed"},
		{"released→consume (blocked)", coinsentity.CoinReservationStatusReleased, "consume", true, "*entity.ErrReservationAlreadyReleased"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paymentID := newPaymentWithFixture(t, testDB, userID)

			// Setup: create reservation in desired initial state
			err := testDB.WithTx(ctx, func(tx db.Tx) error {
				_, err := repo.GetBalanceRowForUpdate(ctx, tx, userID)
				require.NoError(t, err)
				r, err := coinsentity.NewCoinReservation(paymentID, userID, 500, expiresAt)
				require.NoError(t, err)
				require.NoError(t, repo.CreateReservation(ctx, tx, r))

				// Transition to the desired setup state
				switch tt.setupState {
				case coinsentity.CoinReservationStatusConsumed:
					_, err = repo.ConsumeReservation(ctx, tx, paymentID)
				case coinsentity.CoinReservationStatusReleased:
					_, err = repo.ReleaseReservation(ctx, tx, paymentID)
				case coinsentity.CoinReservationStatusReserved:
					// Already reserved, no-op
				}
				return err
			})
			require.NoError(t, err, "setup for %s failed", tt.name)

			// Execute the action under test
			err = testDB.WithTx(ctx, func(tx db.Tx) error {
				var actionErr error
				switch tt.action {
				case "consume":
					_, actionErr = repo.ConsumeReservation(ctx, tx, paymentID)
				case "release":
					_, actionErr = repo.ReleaseReservation(ctx, tx, paymentID)
				}
				if tt.expectError {
					require.Error(t, actionErr, "%s: expected error", tt.name)
					switch tt.expectErrType {
					case "*entity.ErrReservationAlreadyConsumed":
						var alreadyConsumed *coinsentity.ErrReservationAlreadyConsumed
						assert.ErrorAs(t, actionErr, &alreadyConsumed,
							"%s: error should be typed as ErrReservationAlreadyConsumed", tt.name)
					case "*entity.ErrReservationAlreadyReleased":
						var alreadyReleased *coinsentity.ErrReservationAlreadyReleased
						assert.ErrorAs(t, actionErr, &alreadyReleased,
							"%s: error should be typed as ErrReservationAlreadyReleased", tt.name)
					default:
						t.Fatalf("%s: unsupported expected error type %q", tt.name, tt.expectErrType)
					}
				} else {
					require.NoError(t, actionErr, "%s: expected no error", tt.name)
				}
				return nil // Don't propagate expected errors
			})
			require.NoError(t, err, "%s: transaction should complete because expected errors were handled inside", tt.name)
		})
	}

	t.Logf("PASS Test 12: full transition matrix — 6/6 correct")
}

// ============================================================================
// PART G: SCHEMA CONSTRAINT PROOFS
// ============================================================================

// Test 13: Entity-level constraints
func TestReservationSchemaConstraints(t *testing.T) {
	paymentID := uuid.New()
	userID := uuid.New()

	// Amount=0 rejected
	_, err := coinsentity.NewCoinReservation(paymentID, userID, 0, time.Now().Add(1*time.Hour))
	require.Error(t, err, "amount=0 rejected")
	assert.Contains(t, err.Error(), "positive")

	// Negative amount rejected
	_, err = coinsentity.NewCoinReservation(paymentID, userID, -5, time.Now().Add(1*time.Hour))
	require.Error(t, err, "negative amount rejected")

	// Zero expires_at rejected
	_, err = coinsentity.NewCoinReservation(paymentID, userID, 100, time.Time{})
	require.Error(t, err, "zero expires_at rejected")
	assert.Contains(t, err.Error(), "expires_at")

	// Valid creation
	res, err := coinsentity.NewCoinReservation(paymentID, userID, 100, time.Now().Add(1*time.Hour))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, coinsentity.CoinReservationStatusReserved, res.Status)

	// Entity-level double consume
	res2, _ := coinsentity.NewCoinReservation(uuid.New(), userID, 100, time.Now().Add(1*time.Hour))
	require.NoError(t, res2.Consume())
	err = res2.Consume()
	require.Error(t, err, "double consume rejected at entity level")
	assert.Contains(t, err.Error(), "cannot consume")

	// Entity-level double release
	res3, _ := coinsentity.NewCoinReservation(uuid.New(), userID, 100, time.Now().Add(1*time.Hour))
	require.NoError(t, res3.Release())
	err = res3.Release()
	require.Error(t, err, "double release rejected at entity level")
	assert.Contains(t, err.Error(), "cannot release")

	// Entity-level consumed->release
	res4, _ := coinsentity.NewCoinReservation(uuid.New(), userID, 100, time.Now().Add(1*time.Hour))
	require.NoError(t, res4.Consume())
	err = res4.Release()
	require.Error(t, err, "consumed->release rejected at entity level")

	// Entity-level released->consume
	res5, _ := coinsentity.NewCoinReservation(uuid.New(), userID, 100, time.Now().Add(1*time.Hour))
	require.NoError(t, res5.Release())
	err = res5.Consume()
	require.Error(t, err, "released->consume rejected at entity level")

	t.Logf("PASS Test 13: all entity-level constraints enforced")
}

// Test 14: FK enforcement
func TestReservationFKEnforcement(t *testing.T) {
	testDB, repo, _, cleanup := setupReservationTest(t)
	defer cleanup()
	ctx := context.Background()

	nonexistentUserID := uuid.New()
	paymentID := uuid.New()
	expiresAt := time.Now().Add(1 * time.Hour)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		reservation, err := coinsentity.NewCoinReservation(paymentID, nonexistentUserID, 100, expiresAt)
		require.NoError(t, err)
		return repo.CreateReservation(ctx, tx, reservation)
	})
	require.Error(t, err, "FK violation must prevent reservation with no user/payment")
	assert.Contains(t, err.Error(), "user_coin_balance", "error should mention the table that actually rejects the insert")

	t.Logf("PASS Test 14: FK enforcement")
}
