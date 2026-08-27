//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/core/wallet/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestEscrowRepositoryRealDBTerminalGuard(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	id, orderID, walletID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO users (id,firebase_uid,email) VALUES ($1,$2,$3)`, userID, userID.String(), userID.String()+"@proof.test")
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO wallets (id,user_id,available_balance,held_balance) VALUES ($1,$2,0,0)`, walletID, userID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO orders (id,buyer_id,seller_id,source_type,source_id,quantity,unit_price,subtotal,shipping_total,commission_percent,commission_amount,escrow_amount,total_payable_amount,status,escrow_status,payment_expires_at) VALUES ($1,$2,$2,'for_sale',$1,1,1000,1000,0,0,0,1000,1000,'pending_payment','holding',NOW()+INTERVAL '1 hour')`, orderID, userID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO escrows (id,order_id,buyer_wallet_id,amount,status,created_at) VALUES ($1,$2,$3,1000,'holding',NOW())`, id, orderID, walletID)
	require.NoError(t, err)
	repo := &EscrowRepositoryImpl{}
	releasedAt := time.Now().UTC()
	update := func(status entity.EscrowStatus) error {
		return tdb.WithTx(ctx, func(tx db.Tx) error {
			return repo.Update(ctx, tx, &entity.Escrow{ID: id, Status: status, ReleasedAt: &releasedAt})
		})
	}
	require.NoError(t, update(entity.EscrowStatusReleased))
	require.Error(t, update(entity.EscrowStatusRefunded))
	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT status FROM escrows WHERE id=$1`, id).Scan(&status))
	require.Equal(t, "released", status)

	id2, orderID2 := uuid.New(), uuid.New()
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO orders (id,buyer_id,seller_id,source_type,source_id,quantity,unit_price,subtotal,shipping_total,commission_percent,commission_amount,escrow_amount,total_payable_amount,status,escrow_status,payment_expires_at) VALUES ($1,$2,$2,'for_sale',$1,1,1000,1000,0,0,0,1000,1000,'pending_payment','holding',NOW()+INTERVAL '1 hour')`, orderID2, userID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO escrows (id,order_id,buyer_wallet_id,amount,status,created_at) VALUES ($1,$2,$3,1000,'holding',NOW())`, id2, orderID2, walletID)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, next := range []entity.EscrowStatus{entity.EscrowStatusReleased, entity.EscrowStatusRefunded} {
		wg.Add(1)
		go func(next entity.EscrowStatus) {
			defer wg.Done()
			<-start
			results <- updateEscrowStatus(ctx, tdb, repo, id2, next)
		}(next)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT status FROM escrows WHERE id=$1`, id2).Scan(&status))
	require.Contains(t, []string{"released", "refunded"}, status)
}

func updateEscrowStatus(ctx context.Context, tdb *testdb.TestDB, repo *EscrowRepositoryImpl, id uuid.UUID, status entity.EscrowStatus) error {
	releasedAt := time.Now().UTC()
	return tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Update(ctx, tx, &entity.Escrow{ID: id, Status: status, ReleasedAt: &releasedAt})
	})
}
