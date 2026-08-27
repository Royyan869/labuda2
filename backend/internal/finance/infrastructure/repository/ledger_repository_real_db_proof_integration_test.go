//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestLedgerRepositoryRealDBIdempotencyAuthority(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	repo := NewLedgerRepository()
	debit, credit := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{debit, credit} {
		_, err := tdb.Pool().Exec(ctx, `INSERT INTO financial_accounts (id, account_type, name, balance) VALUES ($1, $2, $3, 10000)`, id, "proof_"+id.String(), id.String())
		require.NoError(t, err)
	}
	orderID, paymentID, referenceID := uuid.New(), uuid.New(), uuid.New()

	call := func(key string, ref uuid.UUID, order, payment *uuid.UUID, amount int64, a, b uuid.UUID) error {
		return tdb.WithTx(ctx, func(tx db.Tx) error {
			return repo.CreateTransaction(ctx, tx, key, "proof", ref, order, payment, []repository.Entry{{AccountID: a, Amount: money.New(amount)}, {AccountID: b, Amount: money.New(-amount)}})
		})
	}
	key := "ledger-proof-" + uuid.NewString()
	require.NoError(t, call(key, referenceID, &orderID, &paymentID, 1000, debit, credit))
	require.NoError(t, call(key, referenceID, &orderID, &paymentID, 1000, debit, credit))
	for name, fn := range map[string]func() error{
		"amount":         func() error { return call(key, referenceID, &orderID, &paymentID, 1001, debit, credit) },
		"reference":      func() error { return call(key, uuid.New(), &orderID, &paymentID, 1000, debit, credit) },
		"order":          func() error { v := uuid.New(); return call(key, referenceID, &v, &paymentID, 1000, debit, credit) },
		"payment":        func() error { v := uuid.New(); return call(key, referenceID, &orderID, &v, 1000, debit, credit) },
		"debit account":  func() error { return call(key, referenceID, &orderID, &paymentID, 1000, credit, debit) },
		"credit account": func() error { return call(key, referenceID, &orderID, &paymentID, 1000, debit, uuid.New()) },
	} {
		t.Run(name, func(t *testing.T) { require.Error(t, fn()) })
	}
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM ledger_transactions WHERE idempotency_key=$1`, key).Scan(&count))
	require.Equal(t, 1, count)

	concurrentKey := "ledger-concurrent-" + uuid.NewString()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, amount := range []int64{2000, 3000} {
		wg.Add(1)
		go func(amount int64) {
			defer wg.Done()
			results <- call(concurrentKey, referenceID, &orderID, &paymentID, amount, debit, credit)
		}(amount)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	require.LessOrEqual(t, successes, 1)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM ledger_transactions WHERE idempotency_key=$1`, concurrentKey).Scan(&count))
	require.Equal(t, 1, count)
}
