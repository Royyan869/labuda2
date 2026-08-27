//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	financeApp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReleaseGatewayEscrowRollsBackWhenFinanceAccountMissingRealDB(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	buyerID, sellerID, orderID, walletID, escrowID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO users (id,firebase_uid,email) VALUES ($1,$2,$3),($4,$5,$6)`, buyerID, buyerID.String(), buyerID.String()+"@proof.test", sellerID, sellerID.String(), sellerID.String()+"@proof.test")
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO wallets (id,user_id,available_balance,held_balance) VALUES ($1,$2,0,0)`, walletID, buyerID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO orders (id,buyer_id,seller_id,source_type,source_id,quantity,unit_price,subtotal,shipping_total,commission_percent,commission_amount,escrow_amount,total_payable_amount,status,escrow_status,payment_expires_at) VALUES ($1,$2,$3,'for_sale',$1,1,1000,1000,0,0,0,1000,1000,'completed','holding',NOW()+INTERVAL '1 hour')`, orderID, buyerID, sellerID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO escrows (id,order_id,buyer_wallet_id,amount,status,created_at) VALUES ($1,$2,$3,1000,'holding',NOW())`, escrowID, orderID, walletID)
	require.NoError(t, err)

	wallet := walletApp.NewWalletService(db.NewFromPool(tdb.Pool()), zap.NewNop())
	payment := NewOrderPaymentService(wallet)
	payment.SetFinanceReleaseRecorder(financeApp.NewFinanceService())
	order := &entity.Order{ID: orderID, SellerID: sellerID, Subtotal: money.New(1000), ShippingTotal: money.Zero(), CommissionAmount: money.Zero()}
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := payment.ReleaseGatewayEscrowToSeller(ctx, tx, order)
		return err
	})
	require.Error(t, err)

	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT status FROM escrows WHERE id=$1`, escrowID).Scan(&status))
	require.Equal(t, "holding", status)
	var ledgerTransactions, ledgerEntries int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM ledger_transactions WHERE reference_id=$1`, orderID).Scan(&ledgerTransactions))
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries le JOIN ledger_transactions lt ON lt.id=le.transaction_id WHERE lt.reference_id=$1`, orderID).Scan(&ledgerEntries))
	require.Equal(t, 0, ledgerTransactions)
	require.Equal(t, 0, ledgerEntries)
	var orderStatus, escrowStatus string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT status, escrow_status FROM orders WHERE id=$1`, orderID).Scan(&orderStatus, &escrowStatus))
	require.Equal(t, "completed", orderStatus)
	require.Equal(t, "holding", escrowStatus)
}
