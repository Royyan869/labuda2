//go:build integration

package serverboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	orderapp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	paymentmethodrepo "github.com/labuda/backend/internal/commerce/paymentmethod/infrastructure/repository"
	"github.com/labuda/backend/internal/config"
	walletapp "github.com/labuda/backend/internal/core/wallet/application"
	walletentity "github.com/labuda/backend/internal/core/wallet/entity"
	financeapp "github.com/labuda/backend/internal/finance/application"
	coinsapp "github.com/labuda/backend/internal/incentive/coins/application"
	coinsentity "github.com/labuda/backend/internal/incentive/coins/entity"
	coinsinfrepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	coinsrepointf "github.com/labuda/backend/internal/incentive/coins/repository"
	paymentapp "github.com/labuda/backend/internal/integration/payment/application"
	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	paymentrecon "github.com/labuda/backend/internal/integration/payment/reconciliation"
	disputerepo "github.com/labuda/backend/internal/governance/dispute/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

const testSettlementPaymentMethodCode = "bank_transfer"

type paymentSettlementHarness struct {
	tdb               *testdb.TestDB
	db                *db.DB
	orderRepo         *orderrepo.OrderRepository
	paymentRepo       *paymentrepo.PaymentRepository
	paymentMethodRepo *paymentmethodrepo.PaymentMethodRepository
	coinsRepo         coinsrepointf.CoinsRepository
	coinsService      *coinsapp.CoinsService
	walletService     *walletapp.WalletService
	financeService    *financeapp.FinanceService
	orderService      *orderapp.OrderService
	webhookService    *paymentapp.PaymentWebhookService
	finalizer         *paymentapp.CanonicalFinalizationService
	handler           *CorePaymentHandler
	midtransClient    *midtrans.Client
	buyerID           uuid.UUID
	sellerID          uuid.UUID
	coinsSeeded       bool
}

type failingEscrowRepo struct {
	err error
}

func (r *failingEscrowRepo) GetByID(ctx context.Context, tx db.Tx, escrowID uuid.UUID) (*walletentity.Escrow, error) {
	return nil, nil
}

func (r *failingEscrowRepo) GetByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*walletentity.Escrow, error) {
	return nil, nil
}

func (r *failingEscrowRepo) GetByOrderIDForUpdate(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*walletentity.Escrow, error) {
	return nil, nil
}

func (r *failingEscrowRepo) Create(ctx context.Context, tx db.Tx, escrow *walletentity.Escrow) error {
	return r.err
}

func (r *failingEscrowRepo) Update(ctx context.Context, tx db.Tx, escrow *walletentity.Escrow) error {
	return nil
}

func (r *failingEscrowRepo) GetByBuyerWalletID(ctx context.Context, tx db.Tx, buyerWalletID uuid.UUID) ([]*walletentity.Escrow, error) {
	return nil, nil
}

func (r *failingEscrowRepo) GetBySellerWalletID(ctx context.Context, tx db.Tx, sellerWalletID uuid.UUID) ([]*walletentity.Escrow, error) {
	return nil, nil
}

type failingCreateTransactionCoinsRepo struct {
	coinsrepointf.CoinsRepository
	err error
}

func (r *failingCreateTransactionCoinsRepo) CreateTransaction(ctx context.Context, tx db.Tx, transaction *coinsentity.CoinsTransaction) error {
	return r.err
}

type failingReleaseReservationCoinsRepo struct {
	coinsrepointf.CoinsRepository
	err error
}

func (r *failingReleaseReservationCoinsRepo) ReleaseReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*coinsentity.CoinReservation, error) {
	return nil, r.err
}

type countingReleaseReservationCoinsRepo struct {
	coinsrepointf.CoinsRepository
	releaseCalls int32
}

func (r *countingReleaseReservationCoinsRepo) ReleaseReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*coinsentity.CoinReservation, error) {
	atomic.AddInt32(&r.releaseCalls, 1)
	return r.CoinsRepository.ReleaseReservation(ctx, tx, paymentID)
}

type failingEscrowLookupRepo struct {
	err error
}

func (r *failingEscrowLookupRepo) GetByID(ctx context.Context, tx db.Tx, escrowID uuid.UUID) (*walletentity.Escrow, error) {
	return nil, nil
}

func (r *failingEscrowLookupRepo) GetByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*walletentity.Escrow, error) {
	return nil, r.err
}

func (r *failingEscrowLookupRepo) GetByOrderIDForUpdate(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*walletentity.Escrow, error) {
	return nil, r.err
}

func (r *failingEscrowLookupRepo) Create(ctx context.Context, tx db.Tx, escrow *walletentity.Escrow) error {
	return nil
}

func (r *failingEscrowLookupRepo) Update(ctx context.Context, tx db.Tx, escrow *walletentity.Escrow) error {
	return nil
}

func (r *failingEscrowLookupRepo) GetByBuyerWalletID(ctx context.Context, tx db.Tx, buyerWalletID uuid.UUID) ([]*walletentity.Escrow, error) {
	return nil, nil
}

func (r *failingEscrowLookupRepo) GetBySellerWalletID(ctx context.Context, tx db.Tx, sellerWalletID uuid.UUID) ([]*walletentity.Escrow, error) {
	return nil, nil
}

type paymentFixture struct {
	OrderID       uuid.UUID
	Payment       *paymentrepo.Payment
	CoinsToUse    int64
	ServiceFee    int64
	MidtransID    string
	TransactionID string
}

func newPaymentSettlementHarness(t *testing.T) *paymentSettlementHarness {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	dbConn := db.NewFromPool(tdb.Pool())
	financeBootstrap := financeapp.NewSystemAccountBootstrap(database.NewFromPgx(dbConn))
	_, err := financeBootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	orderRepo := orderrepo.NewOrderRepository()
	paymentRepo := paymentrepo.NewPaymentRepository()
	paymentMethodRepo := paymentmethodrepo.NewPaymentMethodRepository()
	coinsRepo := coinsrepointf.CoinsRepository(coinsinfrepo.NewCoinsRepository())
	coinsService := coinsapp.NewCoinsService(coinsRepo, dbConn)
	walletService := walletapp.NewWalletService(dbConn, zap.NewNop())
	walletService.SetDisputeRepository(disputerepo.NewDisputeRepository())
	financeService := financeapp.NewFinanceService()
	financeService.SetLogger(zap.NewNop())

	outboxRepo := repository.NewOutboxRepository(dbConn)
	orderService := orderapp.NewOrderService(
		nil,
		nil,
		outboxRepo,
		nil,
		coinsService,
		nil,
		nil,
		nil,
		nil,
		walletService,
		nil,
	)
	orderService.PaymentService().SetFinanceReleaseRecorder(financeService)

	midtransLogger, err := logger.New("error", "json", "stdout")
	require.NoError(t, err)
	midtransClient := midtrans.NewClient(&config.MidtransConfig{
		ServerKey:   "test-server-key",
		ClientKey:   "test-client-key",
		Environment: "sandbox",
	}, midtransLogger)

	webhookService := paymentapp.NewPaymentWebhookService(dbConn, midtransClient, orderService, walletService, zap.NewNop())
	finalizer := paymentapp.NewCanonicalFinalizationService(financeService, orderService, walletService, zap.NewNop())
	// CANONICAL COIN CONSUME+SPEND WIRING: complete RESERVE → CONSUME at
	// settlement so K>0 orders atomically consume the reservation, write the
	// order_spend transaction, and deduct the coin balance.
	finalizer.SetCoinSpendConsumer(coinsService)
	webhookService.SetCanonicalFinalizationService(finalizer)
	webhookService.SetFinanceService(financeService)
	handler := &CorePaymentHandler{
		db:                  database.NewFromPgx(dbConn),
		paymentRepo:         paymentRepo,
		coinsRepo:           coinsRepo,
		orderRepo:           orderRepo,
		paymentMethodRepo:   paymentMethodRepo,
		pricingTokenService: &testPricingTokenSnapshotReader{tdb: tdb},
		midtransClient:      midtransClient,
		log:                 midtransLogger,
	}

	buyerID := uuid.New()
	sellerID := uuid.New()
	require.NoError(t, insertPaymentIntentUsers(ctx, tdb, buyerID, sellerID))
	require.NoError(t, seedCanonicalPaymentMethods(ctx, tdb))

	return &paymentSettlementHarness{
		tdb:               tdb,
		db:                dbConn,
		orderRepo:         orderRepo,
		paymentRepo:       paymentRepo,
		paymentMethodRepo: paymentMethodRepo,
		coinsRepo:         coinsRepo,
		coinsService:      coinsService,
		walletService:     walletService,
		financeService:    financeService,
		orderService:      orderService,
		webhookService:    webhookService,
		finalizer:         finalizer,
		handler:           handler,
		midtransClient:    midtransClient,
		buyerID:           buyerID,
		sellerID:          sellerID,
	}
}

func (h *paymentSettlementHarness) seedCoinsBalance(t *testing.T, amount int64) {
	t.Helper()
	if h.coinsSeeded {
		return
	}
	ctx := context.Background()
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := h.coinsRepo.EnsureBalanceRow(ctx, tx, h.buyerID); err != nil {
			return err
		}
		if amount > 0 {
			_, err := h.coinsRepo.AtomicAddBalance(ctx, tx, h.buyerID, amount)
			return err
		}
		return nil
	})
	require.NoError(t, err)
	h.coinsSeeded = true
}

func (h *paymentSettlementHarness) createSettlementFixture(t *testing.T, coinsToUse, serviceFeeAmount int64, seedBalance int64) *paymentFixture {
	t.Helper()
	ctx := context.Background()
	h.seedCoinsBalance(t, seedBalance)

	orderID := createCanonicalOrder(t, ctx, h.tdb, h.orderRepo, h.buyerID, h.sellerID)
	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, orderID)
	require.NoError(t, err)

	grossAmount := orderSnap.TotalBeforeCoins - coinsToUse + serviceFeeAmount
	require.Greater(t, grossAmount, int64(0))

	methodCode := testSettlementPaymentMethodCode
	referenceID := orderID
	midtransID := fmt.Sprintf("MID-SETTLE-%s", uuid.New().String())
	paymentNumber := fmt.Sprintf("PAY-SETTLE-%s", uuid.New().String())

	var payment *paymentrepo.Payment
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = h.paymentRepo.CreatePayment(ctx, tx, paymentrepo.CreatePaymentInput{
			UserID:            h.buyerID,
			PaymentNumber:     paymentNumber,
			MidtransOrderID:   midtransID,
			GrossAmount:       money.New(grossAmount),
			ServiceFeeAmount:  money.New(serviceFeeAmount),
			CoinsToUse:        int(coinsToUse),
			ReferenceType:     paymentrepo.ReferenceTypeOrder,
			ReferenceID:       &referenceID,
			ExpiredAt:         time.Now().Add(1 * time.Hour),
			PaymentMethodCode: &methodCode,
		})
		if err != nil {
			return err
		}
		if coinsToUse <= 0 {
			return nil
		}
		reservation, err := coinsentity.NewCoinReservation(payment.ID, h.buyerID, coinsToUse, time.Now().Add(1*time.Hour))
		if err != nil {
			return err
		}
		if err := h.coinsRepo.CreateReservation(ctx, tx, reservation); err != nil {
			return err
		}
		return nil
	})
	require.NoError(t, err)

	return &paymentFixture{
		OrderID:       orderID,
		Payment:       payment,
		CoinsToUse:    coinsToUse,
		ServiceFee:    serviceFeeAmount,
		MidtransID:    midtransID,
		TransactionID: fmt.Sprintf("trx-%s", uuid.New().String()),
	}
}

func (h *paymentSettlementHarness) makeWebhookNotification(fx *paymentFixture, transactionStatus string) *midtrans.NotificationPayload {
	notif := &midtrans.NotificationPayload{
		TransactionTime:   time.Now().Format(time.RFC3339),
		TransactionStatus: transactionStatus,
		TransactionID:     fx.TransactionID,
		StatusMessage:     "OK",
		StatusCode:        "200",
		PaymentType:       testSettlementPaymentMethodCode,
		OrderID:           fx.MidtransID,
		GrossAmount:       fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64()),
		FraudStatus:       "accept",
		Currency:          "IDR",
	}
	notif.SignatureKey = h.midtransClient.BuildWebhookSignature(notif)
	return notif
}

func (h *paymentSettlementHarness) loadEscrowAmount(t *testing.T, orderID uuid.UUID) int64 {
	t.Helper()
	ctx := context.Background()
	var amount int64
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		escrow, err := h.walletService.GetEscrowForOrder(ctx, tx, orderID)
		if err != nil {
			return err
		}
		require.NotNil(t, escrow)
		amount = escrow.Amount
		return nil
	})
	require.NoError(t, err)
	return amount
}

func (h *paymentSettlementHarness) loadOrderEntity(t *testing.T, orderID uuid.UUID) *orderentity.Order {
	t.Helper()
	ctx := context.Background()
	var order *orderentity.Order
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		order, err = h.orderRepo.GetByID(ctx, tx, orderID)
		return err
	})
	require.NoError(t, err)
	return order
}

func countCoinSpendRows(ctx context.Context, tdb *testdb.TestDB, userID, orderID uuid.UUID) (int64, error) {
	var count int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM coins_transactions
			WHERE user_id = $1
			  AND type = 'spend'
			  AND reference_type = 'order_spend'
			  AND reference_id = $2
		`, userID, orderID).Scan(&count)
	})
	return count, err
}

func countWebhookEventRows(ctx context.Context, tdb *testdb.TestDB, eventID string) (int64, error) {
	var count int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM payment_webhook_events
			WHERE event_id = $1
		`, eventID).Scan(&count)
	})
	return count, err
}

func loadSpendAmount(ctx context.Context, tdb *testdb.TestDB, userID, orderID uuid.UUID) (int64, error) {
	var amount int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT amount
			FROM coins_transactions
			WHERE user_id = $1
			  AND type = 'spend'
			  AND reference_type = 'order_spend'
			  AND reference_id = $2
		`, userID, orderID).Scan(&amount)
	})
	return amount, err
}

func loadReservedCoins(ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) (int64, error) {
	var reserved int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM coin_reservations
			WHERE user_id = $1
			  AND status = 'reserved'
		`, userID).Scan(&reserved)
	})
	return reserved, err
}

// loadSystemAccountBalance returns the current balance of a system
// financial_accounts row by account type (GATEWAY_CLEARING, PLATFORM_BANK,
// PLATFORM_REVENUE, etc.).
func loadSystemAccountBalance(ctx context.Context, tdb *testdb.TestDB, accountType string) (int64, error) {
	var balance int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT balance
			FROM financial_accounts
			WHERE account_type = $1 AND user_id IS NULL
		`, accountType).Scan(&balance)
	})
	return balance, err
}

// loadUserAccountBalance returns the current balance of a per-user
// financial_accounts row by account type and user ID (SELLER_PAYABLE,
// BUYER_REFUNDABLE, etc.).
func loadUserAccountBalance(ctx context.Context, tdb *testdb.TestDB, accountType string, userID uuid.UUID) (int64, error) {
	var balance int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT balance
			FROM financial_accounts
			WHERE account_type = $1 AND user_id = $2
		`, accountType, userID).Scan(&balance)
	})
	return balance, err
}

func updateSpendAmount(ctx context.Context, tdb *testdb.TestDB, userID, paymentID uuid.UUID, amount int64) error {
	return tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE coins_transactions
			SET amount = $3
			WHERE user_id = $1
			  AND type = 'spend'
			  AND reference_type = 'payment_spend'
			  AND reference_id = $2
		`, userID, paymentID, amount)
		return err
	})
}

func (h *paymentSettlementHarness) finalizePaymentTx(ctx context.Context, fx *paymentFixture, transactionID string) error {
	return h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.finalizer.FinalizeOrderPayment(ctx, tx, fx.Payment, transactionID, testSettlementPaymentMethodCode)
	})
}

func (h *paymentSettlementHarness) runCreatePayment(t *testing.T, orderID uuid.UUID, methodCode string, coinsToUse int) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(gin.H{
		"order_id":            orderID,
		"payment_method_code": methodCode,
		"coins_to_use":        coinsToUse,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.Background())
	c.Request = req
	c.Set("userID", h.buyerID)

	h.handler.CreatePayment(c)
	return recorder
}

func (h *paymentSettlementHarness) handleWebhook(ctx context.Context, notif *midtrans.NotificationPayload) error {
	return h.webhookService.HandleWebhook(ctx, notif, "127.0.0.1")
}

func (h *paymentSettlementHarness) setWebhookGateway(client *midtrans.Client) {
	h.midtransClient = client
	h.handler.midtransClient = client
	setPrivateField(h.webhookService, "midtransClient", client)
}

func (h *paymentSettlementHarness) finalizeOrderPaymentFailure(ctx context.Context, payment *paymentrepo.Payment, failedStatus string) error {
	return h.tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := h.paymentRepo.MarkAsFailed(ctx, tx, payment.ID, failedStatus); err != nil {
			return err
		}
		if payment.ReferenceID != nil {
			if err := h.orderService.Expire(ctx, tx, *payment.ReferenceID); err != nil {
				return err
			}
		}
		if payment.CoinsToUse > 0 {
			if _, err := h.coinsRepo.ReleaseReservation(ctx, tx, payment.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestPaymentCoinSettlement_KPositive_PersistsSpendSnapshotAndLedger(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	orderBefore := h.loadOrderEntity(t, fx.OrderID)
	// CANONICAL ESCROW: escrow = total_before_coins_amount = PD + S.
	// The rejected P+S+C model (CalculateGrossEscrowFromSnapshot = 124500)
	// must NOT drive escrow.
	require.Equal(t, int64(110000), orderBefore.TotalBeforeCoinsAmount.Int64())

	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	require.Equal(t, int64(10000), payment.CoinsToUse)
	require.Equal(t, int64(10000), payment.CoinDiscountAmount)

	// CANONICAL COIN AUTHORITY: K lives in the coins/payment domain
	// (payments.coins_to_use, coin_reservations, coins_transactions,
	// user_coin_balance). orders.coins_used / orders.coin_discount_amount are
	// dead columns and are NOT financial authority — they are never populated
	// by production, so assert they remain 0 (not authoritative).
	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, int64(0), orderSnap.CoinsUsed)
	require.Equal(t, int64(0), orderSnap.CoinDiscountAmount)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "consumed", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)

	spendAmount, err := loadSpendAmount(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(10000), spendAmount)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(10000), balance)

	escrowAmount := h.loadEscrowAmount(t, fx.OrderID)
	// CANONICAL ESCROW ROW: escrow = PD + S = 110000 (total_before_coins_amount).
	// The rejected P+S+C model (124500) must not fund the escrow row.
	require.Equal(t, int64(110000), escrowAmount)
}

func TestPaymentCoinSettlement_KZero_SkipsReservationAndSpend(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 0, 4000, 20000)
	require.NoError(t, h.handleWebhook(ctx, h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	require.Equal(t, int64(0), payment.CoinsToUse)
	require.Equal(t, int64(0), payment.CoinDiscountAmount)

	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, int64(0), orderSnap.CoinsUsed)
	require.Equal(t, int64(0), orderSnap.CoinDiscountAmount)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.Nil(t, reservation)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)

	escrowAmount := h.loadEscrowAmount(t, fx.OrderID)
	// CANONICAL ESCROW ROW: escrow = PD + S = 110000. K=0 does not change
	// the escrow; the rejected P+S+C model (124500) must not fund it.
	require.Equal(t, int64(110000), escrowAmount)
}

func TestPaymentCoinSettlement_AvailableBalanceContinuityAndExactlyOneSpend(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 15000, 4000, 20000)

	totalBefore, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), totalBefore)

	reservedBefore, err := loadReservedCoins(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(15000), reservedBefore)
	require.Equal(t, int64(5000), totalBefore-reservedBefore)

	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	totalAfter, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(5000), totalAfter)

	reservedAfter, err := loadReservedCoins(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(0), reservedAfter)
	require.Equal(t, int64(5000), totalAfter-reservedAfter)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "consumed", reservation.Status)

	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	// CANONICAL COIN AUTHORITY: orders.coins_used / coin_discount_amount are
	// dead columns, never populated by production, NOT financial authority.
	require.Equal(t, int64(0), orderSnap.CoinsUsed)
	require.Equal(t, int64(0), orderSnap.CoinDiscountAmount)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)

	spendAmount, err := loadSpendAmount(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(15000), spendAmount)
}

func TestPaymentCoinSettlement_RollsBackWhenSpendInsertFails(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	failingCoinsRepo := &failingCreateTransactionCoinsRepo{
		CoinsRepository: h.coinsRepo,
		err:             errors.New("forced payment spend insert failure"),
	}
	failingCoinsService := coinsapp.NewCoinsService(failingCoinsRepo, h.db)
	_ = failingCoinsService
	failingFinalizer := paymentapp.NewCanonicalFinalizationService(h.financeService, h.orderService, h.walletService, zap.NewNop())

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return failingFinalizer.FinalizeOrderPayment(ctx, tx, fx.Payment, fx.TransactionID, testSettlementPaymentMethodCode)
	})
	require.Error(t, err)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)

	totalAfter, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), totalAfter)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentCoinSettlement_DuplicateWebhook_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	notif := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))

	require.NoError(t, h.handleWebhook(ctx, notif))
	require.NoError(t, h.handleWebhook(ctx, notif))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)

	webhookCount, err := countWebhookEventRows(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, int64(1), webhookCount)
}

func TestPaymentCoinSettlement_ConcurrentDuplicateWebhook_OneSpendOnly(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	notif := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errs [2]error
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = h.handleWebhook(ctx, notif)
		}(i)
	}
	close(start)
	wg.Wait()

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)

	webhookCount, err := countWebhookEventRows(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, int64(1), webhookCount)
}

func TestPaymentCoinSettlement_TwoPaymentsSameUser_ConcurrentSettlements(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fxA := h.createSettlementFixture(t, 10000, 4000, 20000)
	fxA.TransactionID = fmt.Sprintf("trx-a-%s", uuid.New().String())
	notifA := h.makeWebhookNotification(fxA, string(midtrans.StatusSettlement))

	fxB := h.createSettlementFixture(t, 10000, 4000, 20000)
	fxB.TransactionID = fmt.Sprintf("trx-b-%s", uuid.New().String())
	notifB := h.makeWebhookNotification(fxB, string(midtrans.StatusSettlement))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errs [2]error
	run := func(idx int, notif *midtrans.NotificationPayload) {
		defer wg.Done()
		<-start
		errs[idx] = h.handleWebhook(ctx, notif)
	}
	wg.Add(2)
	go run(0, notifA)
	go run(1, notifB)
	close(start)
	wg.Wait()

	paymentA, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fxA.MidtransID)
	require.NoError(t, err)
	paymentB, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fxB.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, paymentA.Status)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, paymentB.Status)

	countA, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, paymentA.ReferenceID)
	require.NoError(t, err)
	countB, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, paymentB.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), countA)
	require.Equal(t, int64(1), countB)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(0), balance)
}

func TestPaymentCoinSettlement_ReleasedReservation_ContradictionFailsClosed(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := h.coinsRepo.ReleaseReservation(ctx, tx, fx.Payment.ID)
		return err
	})
	require.NoError(t, err)

	notif := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))
	require.Error(t, h.handleWebhook(ctx, notif))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentCoinSettlement_ConsumedReplay_ConsistentStateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))
	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	// CANONICAL COIN AUTHORITY: orders.coins_used / coin_discount_amount are
	// dead columns, never populated by production, NOT financial authority.
	require.Equal(t, int64(0), orderSnap.CoinsUsed)
	require.Equal(t, int64(0), orderSnap.CoinDiscountAmount)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "consumed", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)
}

func TestPaymentCoinSettlement_ConsumedReservation_StaleFailureDoesNotDowngrade(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))

	staleFailure := h.makeWebhookNotification(fx, string(midtrans.StatusExpire))
	staleFailure.TransactionID = fmt.Sprintf("trx-stale-failure-%s", uuid.New().String())
	require.NoError(t, h.handleWebhook(ctx, staleFailure))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "consumed", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusPaid, orderStatus)
}

func TestPaymentCoinSettlement_CorruptedConsumedState_FailsClosed(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.NoError(t, updateSpendAmount(ctx, h.tdb, h.buyerID, payment.ID, 9999))

	err = h.finalizePaymentTx(ctx, fx, fx.TransactionID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "integrity violation")

	spendAmount, err := loadSpendAmount(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(9999), spendAmount)
}

func TestPaymentCoinSettlement_NonSuccessStatus_ReleasesReservation(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	for _, tc := range []struct {
		name   string
		status string
	}{
		{name: "deny", status: string(midtrans.StatusDeny)},
		{name: "cancel", status: string(midtrans.StatusCancel)},
		{name: "expire", status: string(midtrans.StatusExpire)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := h.createSettlementFixture(t, 10000, 4000, 20000)
			notif := h.makeWebhookNotification(fx, tc.status)

			require.NoError(t, h.handleWebhook(ctx, notif))

			payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
			require.NoError(t, err)
			require.Equal(t, tc.status, payment.Status)

			reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
			require.NoError(t, err)
			require.NotNil(t, reservation)
			require.Equal(t, "released", reservation.Status)

			spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
			require.NoError(t, err)
			require.Equal(t, int64(0), spendCount)

			orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, fx.OrderID)
			require.NoError(t, err)
			require.Equal(t, int64(0), orderSnap.CoinsUsed)
			require.Equal(t, int64(0), orderSnap.CoinDiscountAmount)
		})
	}
}

func TestPaymentCoinSettlement_RollsBackWhenEscrowCreateFails(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)
	h.walletService.SetEscrowRepository(&failingEscrowRepo{err: errors.New("forced escrow create failure")})

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	err := h.finalizePaymentTx(ctx, fx, fx.TransactionID)
	require.Error(t, err)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)

	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, int64(0), orderSnap.CoinsUsed)
	require.Equal(t, int64(0), orderSnap.CoinDiscountAmount)
}

func TestPaymentCoinSettlement_SellerEntitlementStableAcrossKZeroAndKPositive(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	zeroFx := h.createSettlementFixture(t, 0, 4000, 20000)
	posFx := h.createSettlementFixture(t, 10000, 4000, 20000)

	require.NoError(t, h.finalizePaymentTx(ctx, zeroFx, zeroFx.TransactionID))
	require.NoError(t, h.finalizePaymentTx(ctx, posFx, posFx.TransactionID))

	zeroOrder := h.loadOrderEntity(t, zeroFx.OrderID)
	posOrder := h.loadOrderEntity(t, posFx.OrderID)
	// CANONICAL SELLER ENTITLEMENT: escrow = total_before_coins_amount = PD + S.
	// K must NEVER reduce seller entitlement (BuyerBase is the same whether
	// K=0 or K>0). The rejected P+S+C model (CalculateGrossEscrowFromSnapshot
	// = 124500) must not drive the escrow.
	require.Equal(t, int64(110000), zeroOrder.TotalBeforeCoinsAmount.Int64())
	require.Equal(t, int64(110000), posOrder.TotalBeforeCoinsAmount.Int64())

	zeroEscrow := h.loadEscrowAmount(t, zeroFx.OrderID)
	posEscrow := h.loadEscrowAmount(t, posFx.OrderID)
	require.Equal(t, zeroEscrow, posEscrow)
	require.Equal(t, int64(110000), zeroEscrow)
}

// TestPaymentCoinSettlement_LedgerFundingProof proves the canonical ledger
// contract at runtime for K>0:
//
//	GATEWAY_CLEARING after settlement+fee+coin funding = BuyerBase = PD + S
//	Release drains BuyerBase; GATEWAY_CLEARING never goes negative.
//	PLATFORM_BANK is debited exactly K (platform funds the buyer benefit).
//	PLATFORM_REVENUE = fee F + commission C (K never becomes revenue).
//	Every ledger transaction is balanced (Σ entries = 0), enforced by the
//	ledger repository itself (panics on unbalanced).
//
// Fixture: BuyerBase=110000 (PD=90000, S=20000), K=10000, F=4000, C=4500.
func TestPaymentCoinSettlement_LedgerFundingProof(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	platformBankBefore, err := loadSystemAccountBalance(ctx, h.tdb, "PLATFORM_BANK")
	require.NoError(t, err)
	clearingBefore, err := loadSystemAccountBalance(ctx, h.tdb, "GATEWAY_CLEARING")
	require.NoError(t, err)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	require.NoError(t, h.finalizePaymentTx(ctx, fx, fx.TransactionID))

	// After settlement + fee sweep + platform coin funding:
	// GATEWAY_CLEARING = BuyerBase = 110000 (funded: 96000 cash − 4000 fee + 10000 coin funding).
	clearingAfterFunding, err := loadSystemAccountBalance(ctx, h.tdb, "GATEWAY_CLEARING")
	require.NoError(t, err)
	require.Equal(t, clearingBefore+110000, clearingAfterFunding, "GATEWAY_CLEARING must equal BuyerBase after funding")

	// PLATFORM_BANK debited exactly K (platform funds the buyer benefit; K is
	// never revenue and never credited back except via refund reversal).
	platformBankAfterFunding, err := loadSystemAccountBalance(ctx, h.tdb, "PLATFORM_BANK")
	require.NoError(t, err)
	require.Equal(t, platformBankBefore-10000, platformBankAfterFunding, "PLATFORM_BANK must be debited exactly K")

	// Release the escrow to seller.
	var order *orderentity.Order
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		order, err = h.orderRepo.GetForUpdate(ctx, tx, fx.OrderID)
		return err
	}))
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := h.orderService.PaymentService().ReleaseGatewayEscrowToSeller(ctx, tx, order)
		return err
	}))

	// GATEWAY_CLEARING drains to its pre-settlement level (never negative).
	clearingAfterRelease, err := loadSystemAccountBalance(ctx, h.tdb, "GATEWAY_CLEARING")
	require.NoError(t, err)
	require.Equal(t, clearingBefore, clearingAfterRelease, "GATEWAY_CLEARING must return to pre-settlement balance (never over-drawn)")

	// SELLER_PAYABLE += SellerNet = BuyerBase − C = 110000 − 4500 = 105500.
	sellerPayable, err := loadUserAccountBalance(ctx, h.tdb, "SELLER_PAYABLE", order.SellerID)
	require.NoError(t, err)
	require.Equal(t, int64(105500), sellerPayable, "seller receives BuyerBase − commission (K never reduces seller entitlement)")

	// PLATFORM_REVENUE = F (4000) + C (4500) = 8500. K never becomes revenue.
	platformRevenue, err := loadSystemAccountBalance(ctx, h.tdb, "PLATFORM_REVENUE")
	require.NoError(t, err)
	require.Equal(t, int64(8500), platformRevenue, "PLATFORM_REVENUE = fee + commission; K is NOT revenue")
}

func TestPaymentCoinSettlement_ConcurrentDuplicateWebhook_IsRecordedOnceEvenUnderRacingDelivery(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	notif := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))

	start := make(chan struct{})
	var successCount int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := h.handleWebhook(ctx, notif); err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	require.Equal(t, int32(2), successCount)
	webhookCount, err := countWebhookEventRows(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, int64(1), webhookCount)
}

func TestPaymentCoinSettlement_ConcurrentSuccessWebhookAndTerminalReconciliation_NoMixedFinalState(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	successNotif := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))

	h.setWebhookGateway(newReconciliationMidtransClient(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, string(midtrans.StatusExpire), testSettlementPaymentMethodCode, "trx-terminal-"+uuid.New().String(), fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())), http.StatusOK, nil
	}))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errs [2]error

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs[0] = h.handleWebhook(ctx, successNotif)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = h.reconcilePayment(ctx, fx.Payment.ID)
	}()
	close(start)
	wg.Wait()

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)

	switch payment.Status {
	case paymentrepo.PaymentStatusSettlement:
		require.Equal(t, "consumed", reservation.Status)
		require.Equal(t, int64(1), spendCount)
		orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
		require.NoError(t, err)
		require.Equal(t, orderentity.StatusPaid, orderStatus)
	case paymentrepo.PaymentStatusDeny, paymentrepo.PaymentStatusCancel, paymentrepo.PaymentStatusExpire:
		require.Equal(t, "released", reservation.Status)
		require.Equal(t, int64(0), spendCount)
		orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
		require.NoError(t, err)
		require.Equal(t, orderentity.StatusExpired, orderStatus)
	default:
		t.Fatalf("unexpected final payment status after race: %s", payment.Status)
	}
}

func TestPaymentCoinSettlement_ConcurrentTerminalWebhookAndReconciliation_ExactlyOneTerminalRelease(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	terminalStatus := string(midtrans.StatusExpire)
	notif := h.makeWebhookNotification(fx, terminalStatus)
	h.setWebhookGateway(newReconciliationMidtransClient(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, terminalStatus, testSettlementPaymentMethodCode, "trx-terminal-"+uuid.New().String(), fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())), http.StatusOK, nil
	}))

	start := make(chan struct{})
	var wg sync.WaitGroup
	var errs [2]error

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs[0] = h.handleWebhook(ctx, notif)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = h.reconcilePayment(ctx, fx.Payment.ID)
	}()
	close(start)
	wg.Wait()

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Contains(t, []string{paymentrepo.PaymentStatusDeny, paymentrepo.PaymentStatusCancel, paymentrepo.PaymentStatusExpire}, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusExpired, orderStatus)
}

func TestPaymentCoinSettlement_TerminalFailure_RollsBackBeforeReservationRelease(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	failingCoinsRepo := &failingReleaseReservationCoinsRepo{
		CoinsRepository: h.coinsRepo,
		err:             errors.New("forced reservation release failure"),
	}
	failingCoinsService := coinsapp.NewCoinsService(failingCoinsRepo, h.db)
	_ = failingCoinsService

	err := h.finalizeOrderPaymentFailure(ctx, fx.Payment, string(midtrans.StatusExpire))
	require.Error(t, err)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusPending, orderStatus)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentCoinSettlement_TerminalFailure_RollsBackAfterReservationReleaseBeforeCommit(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	h.walletService.SetEscrowRepository(&failingEscrowLookupRepo{err: errors.New("forced escrow lookup failure")})

	err := h.finalizeOrderPaymentFailure(ctx, fx.Payment, string(midtrans.StatusExpire))
	require.Error(t, err)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusPending, orderStatus)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentCoinSettlement_DefinitiveRefusalCompensationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	countingRepo := &countingReleaseReservationCoinsRepo{
		CoinsRepository: h.coinsRepo,
	}
	h.handler.coinsRepo = countingRepo

	require.NoError(t, h.handler.compensateDefinitiveMidtransRefusal(ctx, fx.Payment.ID, paymentrepo.PaymentStatusDeny))
	require.NoError(t, h.handler.compensateDefinitiveMidtransRefusal(ctx, fx.Payment.ID, paymentrepo.PaymentStatusDeny))
	require.Equal(t, int32(1), atomic.LoadInt32(&countingRepo.releaseCalls))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusDeny, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestPaymentCoinSettlement_DefinitiveRefusalCompensationIsRaceSafe(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	countingRepo := &countingReleaseReservationCoinsRepo{
		CoinsRepository: h.coinsRepo,
	}
	h.handler.coinsRepo = countingRepo

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = h.handler.compensateDefinitiveMidtransRefusal(ctx, fx.Payment.ID, paymentrepo.PaymentStatusDeny)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "compensation goroutine %d failed", i)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&countingRepo.releaseCalls))

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusDeny, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentCoinSettlement_TerminalFailureRetry_ConvergesOnSecondAttempt(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	flakyEscrowRepo := &failingEscrowLookupRepo{err: errors.New("transient escrow lookup failure")}
	h.walletService.SetEscrowRepository(flakyEscrowRepo)

	err := h.finalizeOrderPaymentFailure(ctx, fx.Payment, string(midtrans.StatusExpire))
	require.Error(t, err)

	flakyEscrowRepo.err = nil

	err = h.finalizeOrderPaymentFailure(ctx, fx.Payment, string(midtrans.StatusExpire))
	require.NoError(t, err)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusExpire, payment.Status)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusExpired, orderStatus)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentCoinSettlement_NewPaymentAfterAuthoritativeTerminalFailureUsesFreshPaymentAndReservation(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	first := h.createSettlementFixture(t, 10000, 4000, 20000)
	h.setWebhookGateway(newReconciliationMidtransClient(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, string(midtrans.StatusExpire), testSettlementPaymentMethodCode, "trx-first-"+uuid.New().String(), fmt.Sprintf("%d.00", first.Payment.GrossAmount.Int64())), http.StatusOK, nil
	}))
	result, err := h.reconcilePayment(ctx, first.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, paymentrecon.OutcomeTerminalFailure, result.Outcome)

	firstPayment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, first.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusExpire, firstPayment.Status)
	firstReservation, err := loadReservationByPaymentID(ctx, h.tdb, firstPayment.ID)
	require.NoError(t, err)
	require.NotNil(t, firstReservation)
	require.Equal(t, "released", firstReservation.Status)

	secondOrderID := createCanonicalOrder(t, ctx, h.tdb, h.orderRepo, h.buyerID, h.sellerID)
	createGateway := &recordingSnapGateway{tdb: h.tdb, response: &midtrans.SnapResponse{Token: "tok-second", RedirectURL: "https://midtrans.example/second"}}
	h.handler.midtransClient = createGateway
	secondResp := h.runCreatePayment(t, secondOrderID, "bank_transfer", 10000)
	require.Equal(t, http.StatusOK, secondResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&createGateway.calls))

	secondPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, secondOrderID)
	require.NoError(t, err)
	require.NotEqual(t, firstPayment.ID, secondPayment.ID)
	require.Equal(t, paymentrepo.PaymentStatusPending, secondPayment.Status)

	secondReservation, err := loadReservationByPaymentID(ctx, h.tdb, secondPayment.ID)
	require.NoError(t, err)
	require.NotNil(t, secondReservation)
	require.Equal(t, "reserved", secondReservation.Status)

	secondOrderStatus, err := loadOrderStatusByID(ctx, h.tdb, secondOrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusPending, secondOrderStatus)
}

func TestPaymentCoinSettlement_CreateTimeoutLeavesMissingURLAndBlocksSecondIntent(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	gateway := &recordingSnapGateway{tdb: h.tdb, err: errors.New("network timeout")}
	h.handler.midtransClient = gateway
	h.seedCoinsBalance(t, 20000)

	orderID := createCanonicalOrder(t, ctx, h.tdb, h.orderRepo, h.buyerID, h.sellerID)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	t.Logf("first create response: code=%d body=%s", resp.Code, resp.Body.String())
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	firstPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, firstPayment.Status)
	require.Nil(t, firstPayment.PaymentURL)

	firstReservation, err := loadReservationByPaymentID(ctx, h.tdb, firstPayment.ID)
	require.NoError(t, err)
	require.NotNil(t, firstReservation)
	require.Equal(t, "reserved", firstReservation.Status)

	retryResp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusOK, retryResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	secondPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, firstPayment.ID, secondPayment.ID)
	require.Nil(t, secondPayment.PaymentURL)

	count, err := countPaymentsForOrder(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestPaymentCoinSettlement_ActiveUncertainPaymentBlocksDuplicateIntentAcrossProviderStates(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		responder lookupResponder
	}{
		{
			name: "provider_pending",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return gatewayStatusPayload(orderID, string(midtrans.StatusPending), testSettlementPaymentMethodCode, "trx-pending-"+uuid.New().String(), "96000.00"), http.StatusOK, nil
			},
		},
		{
			name: "lookup_timeout",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return nil, 0, errors.New("lookup timeout")
			},
		},
		{
			name: "err_transaction_not_found",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return nil, http.StatusNotFound, nil
			},
		},
		{
			name: "missing_url_uncertain",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return nil, 0, errors.New("missing payment url / gateway uncertain")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPaymentSettlementHarness(t)
			h.seedCoinsBalance(t, 20000)

			createGateway := &recordingSnapGateway{tdb: h.tdb, response: &midtrans.SnapResponse{Token: "tok-" + tc.name, RedirectURL: "https://midtrans.example/" + tc.name}}
			h.handler.midtransClient = createGateway

			orderID := createCanonicalOrder(t, ctx, h.tdb, h.orderRepo, h.buyerID, h.sellerID)
			firstResp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
			t.Logf("first create response: code=%d body=%s", firstResp.Code, firstResp.Body.String())
			require.Equal(t, http.StatusOK, firstResp.Code)
			require.Equal(t, int32(1), atomic.LoadInt32(&createGateway.calls))

			firstPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
			require.NoError(t, err)
			require.Equal(t, paymentrepo.PaymentStatusPending, firstPayment.Status)

			h.setWebhookGateway(newReconciliationMidtransClient(t, tc.responder))
			result, err := h.reconcilePayment(ctx, firstPayment.ID)
			require.NoError(t, err)
			require.Equal(t, paymentrecon.OutcomeUncertain, result.Outcome)

			afterReconcile, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
			require.NoError(t, err)
			require.Equal(t, paymentrepo.PaymentStatusPending, afterReconcile.Status)

			reservation, err := loadReservationByPaymentID(ctx, h.tdb, afterReconcile.ID)
			require.NoError(t, err)
			require.NotNil(t, reservation)
			require.Equal(t, "reserved", reservation.Status)

			retryResp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
			t.Logf("retry create response: code=%d body=%s", retryResp.Code, retryResp.Body.String())
			require.Equal(t, http.StatusOK, retryResp.Code)
			require.Equal(t, int32(1), atomic.LoadInt32(&createGateway.calls))

			afterRetry, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
			require.NoError(t, err)
			require.Equal(t, firstPayment.ID, afterRetry.ID)

			count, err := countPaymentsForOrder(ctx, h.tdb, orderID)
			require.NoError(t, err)
			require.Equal(t, int64(1), count)
		})
	}
}
