//go:build integration

package serverboot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	paymentmethodrepo "github.com/labuda/backend/internal/commerce/paymentmethod/infrastructure/repository"
	coinsentity "github.com/labuda/backend/internal/incentive/coins/entity"
	coinsinfrepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	coinsrepo "github.com/labuda/backend/internal/incentive/coins/repository"
	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	pricingtokeninfrepo "github.com/labuda/backend/internal/pricing/token/infrastructure/repository"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

const (
	canonicalBuyerBase int64 = 110000
	canonicalShipping  int64 = 20000
)

type paymentIntentHarness struct {
	tdb                 *testdb.TestDB
	appDB               *database.DB
	orderRepo           *orderrepo.OrderRepository
	paymentRepo         *paymentrepo.PaymentRepository
	coinsRepo           coinsrepo.CoinsRepository
	paymentMethodRepo   *paymentmethodrepo.PaymentMethodRepository
	pricingTokenService interface {
		GetSnapshot(ctx context.Context, tx db.Tx, token uuid.UUID) (*pricingtokenentity.PricingToken, error)
	}
	handler  *CorePaymentHandler
	buyerID  uuid.UUID
	sellerID uuid.UUID
}

type paymentSnapshot struct {
	ID                 uuid.UUID
	GrossAmount        int64
	ServiceFeeAmount   int64
	CoinsToUse         int64
	CoinDiscountAmount int64
	Status             string
	PaymentMethodCode  *string
	PaymentURL         *string
	MidtransOrderID    string
	ReferenceID        uuid.UUID
}

type orderSnapshot struct {
	ServiceFeeAmount   int64
	TotalPayableAmount int64
	TotalBeforeCoins   int64
	CoinsUsed          int64
	CoinDiscountAmount int64
}

type reservationSnapshot struct {
	PaymentID uuid.UUID
	Amount    int64
	Status    string
}

type paymentMethodPreview struct {
	MethodCode            string `json:"method_code"`
	DisplayName           string `json:"display_name"`
	CoinsToUse            int64  `json:"coins_to_use"`
	CashAmount            int64  `json:"cash_amount"`
	BuyerPaymentFeeAmount int64  `json:"buyer_payment_fee_amount"`
	TotalPayableAmount    int64  `json:"total_payable_amount"`
}

type createPaymentEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		PaymentID          string `json:"payment_id"`
		Status             string `json:"status"`
		PaymentNumber      string `json:"payment_number"`
		PaymentURL         string `json:"payment_url"`
		PaymentMethodCode  string `json:"payment_method_code"`
		BuyerPaymentFeeAmt int64  `json:"buyer_payment_fee_amount"`
		GrossAmount        int64  `json:"gross_amount"`
		CoinsToUse         int64  `json:"coins_to_use"`
		CoinDiscountAmount int64  `json:"coin_discount_amount"`
		ReferenceType      string `json:"reference_type"`
		ReferenceID        any    `json:"reference_id"`
		ExpiredAt          string `json:"expired_at"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type listPaymentMethodsEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		OrderID    string                 `json:"order_id"`
		BaseAmount int64                  `json:"base_amount"`
		Methods    []paymentMethodPreview `json:"methods"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type recordingSnapGateway struct {
	tdb      *testdb.TestDB
	calls    int32
	response *midtrans.SnapResponse
	err      error
	inspect  func(*midtrans.SnapRequest) error
}

func (g *recordingSnapGateway) IsProduction() bool { return false }

func (g *recordingSnapGateway) CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error) {
	atomic.AddInt32(&g.calls, 1)
	if g.err != nil {
		return nil, g.err
	}
	if g.inspect != nil {
		if err := g.inspect(req); err != nil {
			return nil, err
		}
	}
	if g.response != nil {
		return g.response, nil
	}
	return &midtrans.SnapResponse{
		Token:       "snap-token",
		RedirectURL: "https://midtrans.example/redirect",
	}, nil
}

type failingReservationCoinsRepo struct {
	coinsrepo.CoinsRepository
	createReservationErr error
}

func (r *failingReservationCoinsRepo) CreateReservation(ctx context.Context, tx db.Tx, reservation *coinsentity.CoinReservation) error {
	return r.createReservationErr
}

type testPricingTokenSnapshotReader struct {
	tdb *testdb.TestDB
}

func (r *testPricingTokenSnapshotReader) GetSnapshot(ctx context.Context, _ db.Tx, token uuid.UUID) (*pricingtokenentity.PricingToken, error) {
	var pricingToken *pricingtokenentity.PricingToken
	err := r.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		pricingToken, err = pricingtokeninfrepo.NewPricingTokenRepository().GetByToken(ctx, tx, token)
		return err
	})
	return pricingToken, err
}

func newPaymentIntentHarness(t *testing.T, balance int64, gateway MidtransGateway, coinsOverride coinsrepo.CoinsRepository) *paymentIntentHarness {
	t.Helper()

	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	orderRepo := orderrepo.NewOrderRepository()
	paymentRepo := paymentrepo.NewPaymentRepository()
	paymentMethodRepo := paymentmethodrepo.NewPaymentMethodRepository()
	coinsRepo := coinsrepo.CoinsRepository(coinsinfrepo.NewCoinsRepository())
	if coinsOverride != nil {
		coinsRepo = coinsOverride
	}

	err := seedCanonicalPaymentMethods(ctx, tdb)
	require.NoError(t, err)

	buyerID := uuid.New()
	sellerID := uuid.New()
	err = insertPaymentIntentUsers(ctx, tdb, buyerID, sellerID)
	require.NoError(t, err)

	appDB := database.NewFromPgx(db.NewFromPool(tdb.Pool()))
	log, err := logger.New("error", "json", "stdout")
	require.NoError(t, err)

	h := &CorePaymentHandler{
		db:                  appDB,
		paymentRepo:         paymentRepo,
		coinsRepo:           coinsRepo,
		orderRepo:           orderRepo,
		paymentMethodRepo:   paymentMethodRepo,
		pricingTokenService: &testPricingTokenSnapshotReader{tdb: tdb},
		midtransClient:      gateway,
		log:                 log,
	}

	if balance >= 0 {
		err = tdb.WithTx(ctx, func(tx db.Tx) error {
			if err := coinsRepo.EnsureBalanceRow(ctx, tx, buyerID); err != nil {
				return err
			}
			if balance == 0 {
				return nil
			}
			_, err := coinsRepo.AtomicAddBalance(ctx, tx, buyerID, balance)
			return err
		})
		require.NoError(t, err)
	}

	return &paymentIntentHarness{
		tdb:                 tdb,
		appDB:               appDB,
		orderRepo:           orderRepo,
		paymentRepo:         paymentRepo,
		coinsRepo:           coinsRepo,
		paymentMethodRepo:   paymentMethodRepo,
		pricingTokenService: &testPricingTokenSnapshotReader{tdb: tdb},
		handler:             h,
		buyerID:             buyerID,
		sellerID:            sellerID,
	}
}

func seedCanonicalPaymentMethods(ctx context.Context, tdb *testdb.TestDB) error {
	return tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO payment_methods
				(method_code, display_name, enabled, fee_type, flat_amount_rupiah, percent_bps, min_fee_rupiah, max_fee_rupiah,
				 midtrans_channels, sort_order, rate_source, rate_source_note)
			VALUES
				('bank_transfer', 'Transfer Bank (Virtual Account)', true, 'flat', 4000, 0, NULL, NULL,
					ARRAY['bca_va', 'bni_va', 'bri_va', 'permata_va', 'other_va'], 10,
					'public_baseline', 'test seed'),
				('qris', 'QRIS', true, 'percent', 0, 70, 500, NULL,
					ARRAY['other_qris'], 20,
					'public_baseline', 'test seed')
			ON CONFLICT (method_code) DO NOTHING
		`)
		return err
	})
}

func insertPaymentIntentUsers(ctx context.Context, tdb *testdb.TestDB, buyerID, sellerID uuid.UUID) error {
	return tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, role)
			VALUES
				($1, $2, $3, 'user'),
				($4, $5, $6, 'user')
			ON CONFLICT (id) DO NOTHING
		`, sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local",
			buyerID, "fb-"+buyerID.String()[:8], buyerID.String()+"@test.local")
		return err
	})
}

func createCanonicalOrder(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderRepo *orderrepo.OrderRepository, buyerID, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	orderID, _ := createPaymentIntentOrderWithToken(t, ctx, tdb, orderRepo, buyerID, sellerID, canonicalShipping, canonicalBuyerBase)
	return orderID
}

func (h *paymentIntentHarness) createOrder(t *testing.T) uuid.UUID {
	t.Helper()
	return createCanonicalOrder(t, context.Background(), h.tdb, h.orderRepo, h.buyerID, h.sellerID)
}

func (h *paymentIntentHarness) runCreatePayment(t *testing.T, orderID uuid.UUID, methodCode string, coinsToUse int) *httptest.ResponseRecorder {
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

func (h *paymentIntentHarness) runListPaymentMethods(t *testing.T, orderID uuid.UUID, coinsToUse int) listPaymentMethodsEnvelope {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/payments/methods?order_id="+orderID.String()+fmt.Sprintf("&coins_to_use=%d", coinsToUse), nil)
	req = req.WithContext(context.Background())
	c.Request = req
	c.Set("userID", h.buyerID)

	h.handler.ListPaymentMethods(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var envelope listPaymentMethodsEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	return envelope
}

func loadPaymentSnapshotByMidtransOrderID(ctx context.Context, tdb *testdb.TestDB, midtransOrderID string) (*paymentSnapshot, error) {
	var payment paymentSnapshot
	var paymentMethodCode sql.NullString
	var paymentURL sql.NullString
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
			       status, payment_method_code, payment_url, midtrans_order_id, reference_id
			FROM payments
			WHERE midtrans_order_id = $1
		`, midtransOrderID).Scan(
			&payment.ID,
			&payment.GrossAmount,
			&payment.ServiceFeeAmount,
			&payment.CoinsToUse,
			&payment.CoinDiscountAmount,
			&payment.Status,
			&paymentMethodCode,
			&paymentURL,
			&payment.MidtransOrderID,
			&payment.ReferenceID,
		)
	})
	if err != nil {
		return nil, err
	}
	if paymentMethodCode.Valid {
		payment.PaymentMethodCode = &paymentMethodCode.String
	}
	if paymentURL.Valid {
		payment.PaymentURL = &paymentURL.String
	}
	return &payment, nil
}

func loadPaymentSnapshotByOrderID(ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) (*paymentSnapshot, error) {
	var payment paymentSnapshot
	var paymentMethodCode sql.NullString
	var paymentURL sql.NullString
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
			       status, payment_method_code, payment_url, midtrans_order_id, reference_id
			FROM payments
			WHERE reference_type = 'order'
			  AND reference_id = $1
		`, orderID).Scan(
			&payment.ID,
			&payment.GrossAmount,
			&payment.ServiceFeeAmount,
			&payment.CoinsToUse,
			&payment.CoinDiscountAmount,
			&payment.Status,
			&paymentMethodCode,
			&paymentURL,
			&payment.MidtransOrderID,
			&payment.ReferenceID,
		)
	})
	if err != nil {
		return nil, err
	}
	if paymentMethodCode.Valid {
		payment.PaymentMethodCode = &paymentMethodCode.String
	}
	if paymentURL.Valid {
		payment.PaymentURL = &paymentURL.String
	}
	return &payment, nil
}

type pricingTokenSnapshot struct {
	Subtotal           int64
	ShippingTotal      int64
	DiscountAmount     int64
	OrderValueForCoins int64
	MaxCoinsAllowed    int64
}

func loadPricingTokenSnapshotByTokenID(ctx context.Context, tdb *testdb.TestDB, tokenID uuid.UUID) (*pricingTokenSnapshot, error) {
	var token pricingTokenSnapshot
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT subtotal, shipping_total, discount_amount, order_value_for_coins, max_coins_allowed
			FROM pricing_tokens
			WHERE token = $1
		`, tokenID).Scan(
			&token.Subtotal,
			&token.ShippingTotal,
			&token.DiscountAmount,
			&token.OrderValueForCoins,
			&token.MaxCoinsAllowed,
		)
	})
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func loadOrderSnapshotByID(ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) (*orderSnapshot, error) {
	var order orderSnapshot
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT service_fee_amount, total_payable_amount,
			       total_before_coins_amount, coins_used, coin_discount_amount
			FROM orders
			WHERE id = $1
		`, orderID).Scan(
			&order.ServiceFeeAmount,
			&order.TotalPayableAmount,
			&order.TotalBeforeCoins,
			&order.CoinsUsed,
			&order.CoinDiscountAmount,
		)
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func loadReservationByPaymentID(ctx context.Context, tdb *testdb.TestDB, paymentID uuid.UUID) (*reservationSnapshot, error) {
	var res reservationSnapshot
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT payment_id, amount, status
			FROM coin_reservations
			WHERE payment_id = $1
		`, paymentID).Scan(&res.PaymentID, &res.Amount, &res.Status)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &res, nil
}

func countPaymentsForOrder(ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) (int64, error) {
	var count int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM payments
			WHERE reference_type = 'order'
			  AND reference_id = $1
		`, orderID).Scan(&count)
	})
	return count, err
}

func countReservationsForPayment(ctx context.Context, tdb *testdb.TestDB, paymentID uuid.UUID) (int64, error) {
	var count int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM coin_reservations
			WHERE payment_id = $1
		`, paymentID).Scan(&count)
	})
	return count, err
}

func loadUserCoinBalance(ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) (int64, error) {
	var balance int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT balance FROM user_coin_balance WHERE user_id = $1`, userID).Scan(&balance)
	})
	return balance, err
}

func seedPaymentIntentPricingToken(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	buyerID uuid.UUID,
	shippingTotal int64,
	totalBeforeCoins int64,
) uuid.UUID {
	t.Helper()

	pricingTokenRepo := pricingtokeninfrepo.NewPricingTokenRepository()
	token := pricingtokenentity.NewPricingToken(
		buyerID,
		uuid.New(),
		"for_sale",
		uuid.New(),
		1,
		money.New(100000),
		money.New(shippingTotal),
		5,
		money.New(4500),
		money.New(totalBeforeCoins),
		money.New(0),
		uuid.New(),
		"JNE Reguler",
		"reguler",
		nil,
		nil,
		uuid.New(),
		[]byte(`{}`),
		nil,
		nil,
		nil,
		nil,
		money.New(10000),
		nil,
		0,
		18000,
		90000,
	)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return pricingTokenRepo.CreateTx(ctx, tx, token)
	})
	require.NoError(t, err)

	return token.Token
}

func createPaymentIntentOrderWithToken(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	orderRepo *orderrepo.OrderRepository,
	buyerID, sellerID uuid.UUID,
	shippingTotal int64,
	totalBeforeCoins int64,
) (uuid.UUID, uuid.UUID) {
	t.Helper()

	tokenID := seedPaymentIntentPricingToken(t, ctx, tdb, buyerID, shippingTotal, totalBeforeCoins)

	order := orderentity.NewOrderFromSource(
		buyerID,
		sellerID,
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,
		1,
		money.New(100000),
		money.New(100000),
		money.New(shippingTotal),
		5,
		money.New(4500),
		money.New(0),
		money.New(totalBeforeCoins),
		nil,
		"JNE Reguler",
		"reguler",
		nil,
		nil,
		nil,
		"immediate",
		nil,
		nil,
		nil,
		nil,
		&tokenID,
		"default",
		time.Now().Add(1*time.Hour),
	)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return orderRepo.CreateOrderTx(ctx, tx, order)
	})
	require.NoError(t, err)
	return order.ID, tokenID
}

func TestCreatePayment_BasicFlowAndPreviewAuthority(t *testing.T) {
	ctx := context.Background()
	previewGateway := &recordingSnapGateway{response: &midtrans.SnapResponse{Token: "tok-preview", RedirectURL: "https://pay.example.com/preview"}}
	h := newPaymentIntentHarness(t, 20000, previewGateway, nil)

	previewOrderID := h.createOrder(t)
	previewOrder, err := loadOrderSnapshotByID(ctx, h.tdb, previewOrderID)
	require.NoError(t, err)
	require.Equal(t, canonicalBuyerBase, previewOrder.TotalBeforeCoins)

	preview := h.runListPaymentMethods(t, previewOrderID, 18000)
	require.Equal(t, previewOrderID.String(), preview.Data.OrderID)
	require.Equal(t, canonicalBuyerBase, preview.Data.BaseAmount)

	var bankTransfer paymentMethodPreview
	for i := range preview.Data.Methods {
		m := preview.Data.Methods[i]
		if m.MethodCode == "bank_transfer" {
			bankTransfer = m
			break
		}
	}
	require.Equal(t, "bank_transfer", bankTransfer.MethodCode, "bank_transfer should be listed")
	require.Equal(t, int64(18000), bankTransfer.CoinsToUse)
	require.Equal(t, int64(92000), bankTransfer.CashAmount)
	require.Equal(t, int64(4000), bankTransfer.BuyerPaymentFeeAmount)
	require.Equal(t, int64(96000), bankTransfer.TotalPayableAmount)

	paymentOrderID := h.createOrder(t)
	successGateway := &recordingSnapGateway{tdb: h.tdb}
	successGateway.inspect = func(req *midtrans.SnapRequest) error {
		if got := int64(math.Round(req.TransactionDetails.GrossAmount)); got != 96000 {
			return fmt.Errorf("snap gross mismatch: got %d want 96000", got)
		}
		return nil
	}
	successGateway.response = &midtrans.SnapResponse{Token: "tok-success", RedirectURL: "https://midtrans.example/redirect"}
	h.handler.midtransClient = successGateway

	resp := h.runCreatePayment(t, paymentOrderID, "bank_transfer", 18000)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&successGateway.calls))

	orderAfterPayment, err := loadOrderSnapshotByID(ctx, h.tdb, paymentOrderID)
	require.NoError(t, err)
	require.Equal(t, int64(4000), orderAfterPayment.ServiceFeeAmount)
	require.Equal(t, int64(96000), orderAfterPayment.TotalPayableAmount)
	require.Equal(t, canonicalBuyerBase, orderAfterPayment.TotalBeforeCoins)
	require.Equal(t, int64(0), orderAfterPayment.CoinsUsed)
	require.Equal(t, int64(0), orderAfterPayment.CoinDiscountAmount)

	firstPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, paymentOrderID)
	require.NoError(t, err)
	require.NotNil(t, firstPayment.PaymentMethodCode)
	require.Equal(t, "bank_transfer", *firstPayment.PaymentMethodCode)

	reuseResp := h.runCreatePayment(t, paymentOrderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusOK, reuseResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&successGateway.calls), "reused payment must not call Midtrans again")

	reusedPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, paymentOrderID)
	require.NoError(t, err)
	require.NotNil(t, reusedPayment.PaymentMethodCode)
	require.Equal(t, "bank_transfer", *reusedPayment.PaymentMethodCode)
	require.Equal(t, firstPayment.ID, reusedPayment.ID)

	orderAfterReuse, err := loadOrderSnapshotByID(ctx, h.tdb, paymentOrderID)
	require.NoError(t, err)
	require.Equal(t, canonicalBuyerBase, orderAfterReuse.TotalBeforeCoins, "same-intent reuse must not restamp the canonical buyer base")

	conflictKResp := h.runCreatePayment(t, paymentOrderID, "bank_transfer", 17000)
	require.Equal(t, http.StatusConflict, conflictKResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&successGateway.calls))

	orderAfterConflictK, err := loadOrderSnapshotByID(ctx, h.tdb, paymentOrderID)
	require.NoError(t, err)
	require.Equal(t, canonicalBuyerBase, orderAfterConflictK.TotalBeforeCoins, "coin-count conflict must not restamp the canonical buyer base")

	conflictMethodResp := h.runCreatePayment(t, paymentOrderID, "qris", 18000)
	require.Equal(t, http.StatusConflict, conflictMethodResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&successGateway.calls))

	orderAfterConflictMethod, err := loadOrderSnapshotByID(ctx, h.tdb, paymentOrderID)
	require.NoError(t, err)
	require.Equal(t, canonicalBuyerBase, orderAfterConflictMethod.TotalBeforeCoins, "payment-method conflict must not restamp the canonical buyer base")

	zeroGateway := &recordingSnapGateway{tdb: h.tdb}
	zeroGateway.inspect = func(req *midtrans.SnapRequest) error {
		if got := int64(math.Round(req.TransactionDetails.GrossAmount)); got != 114000 {
			return fmt.Errorf("zero-K snap gross mismatch: got %d want 114000", got)
		}
		return nil
	}
	zeroGateway.response = &midtrans.SnapResponse{Token: "tok-zero", RedirectURL: "https://midtrans.example/zero"}
	h.handler.midtransClient = zeroGateway
	zeroOrderID := h.createOrder(t)
	zeroResp := h.runCreatePayment(t, zeroOrderID, "bank_transfer", 0)
	require.Equal(t, http.StatusOK, zeroResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&zeroGateway.calls))

	zeroOrderAfterPayment, err := loadOrderSnapshotByID(ctx, h.tdb, zeroOrderID)
	require.NoError(t, err)
	require.Equal(t, int64(4000), zeroOrderAfterPayment.ServiceFeeAmount)
	require.Equal(t, int64(114000), zeroOrderAfterPayment.TotalPayableAmount)
	require.Equal(t, canonicalBuyerBase, zeroOrderAfterPayment.TotalBeforeCoins)

	zeroPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, zeroOrderID)
	require.NoError(t, err)
	require.NotNil(t, zeroPayment.PaymentMethodCode)
	require.Equal(t, "bank_transfer", *zeroPayment.PaymentMethodCode)

	invalidOrderID := h.createOrder(t)
	invalidResp := h.runCreatePayment(t, invalidOrderID, "bank_transfer", 22001)
	require.Equal(t, http.StatusBadRequest, invalidResp.Code)
	require.Contains(t, invalidResp.Body.String(), "coins_to_use exceeds max allowed")

	insufficientOrderID := h.createOrder(t)
	insufficientResp := h.runCreatePayment(t, insufficientOrderID, "bank_transfer", 15000)
	require.Equal(t, http.StatusConflict, insufficientResp.Code)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestCreatePayment_ShippingPositivePricingTokenCoinCap(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderID, tokenID := createPaymentIntentOrderWithToken(t, ctx, h.tdb, h.orderRepo, h.buyerID, h.sellerID, 50000, 140000)
	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(140000), orderSnap.TotalBeforeCoins)

	tokenSnap, err := loadPricingTokenSnapshotByTokenID(ctx, h.tdb, tokenID)
	require.NoError(t, err)
	require.Equal(t, int64(100000), tokenSnap.Subtotal)
	require.Equal(t, int64(10000), tokenSnap.DiscountAmount)
	require.Equal(t, int64(50000), tokenSnap.ShippingTotal)
	require.Equal(t, int64(90000), tokenSnap.OrderValueForCoins)
	require.Equal(t, int64(18000), tokenSnap.MaxCoinsAllowed)

	preview := h.runListPaymentMethods(t, orderID, 18000)
	require.Equal(t, int64(140000), preview.Data.BaseAmount)

	var bankTransfer paymentMethodPreview
	for i := range preview.Data.Methods {
		m := preview.Data.Methods[i]
		if m.MethodCode == "bank_transfer" {
			bankTransfer = m
			break
		}
	}
	require.Equal(t, "bank_transfer", bankTransfer.MethodCode)
	require.Equal(t, int64(18000), bankTransfer.CoinsToUse)
	require.Equal(t, int64(122000), bankTransfer.CashAmount)
	require.Equal(t, int64(4000), bankTransfer.BuyerPaymentFeeAmount)
	require.Equal(t, int64(126000), bankTransfer.TotalPayableAmount)

	createGateway := &recordingSnapGateway{tdb: h.tdb}
	createGateway.inspect = func(req *midtrans.SnapRequest) error {
		if got := int64(math.Round(req.TransactionDetails.GrossAmount)); got != 126000 {
			return fmt.Errorf("snap gross mismatch: got %d want 126000", got)
		}
		return nil
	}
	createGateway.response = &midtrans.SnapResponse{Token: "tok-shipping", RedirectURL: "https://midtrans.example/shipping"}
	h.handler.midtransClient = createGateway

	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&createGateway.calls))

	createdPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(126000), createdPayment.GrossAmount)
	require.Equal(t, int64(4000), createdPayment.ServiceFeeAmount)
	require.Equal(t, int64(18000), createdPayment.CoinsToUse)
	require.Equal(t, int64(18000), createdPayment.CoinDiscountAmount)

	previewTooMuch := httptest.NewRecorder()
	previewCtx, _ := gin.CreateTestContext(previewTooMuch)
	previewReq := httptest.NewRequest(http.MethodGet, "/payments/methods?order_id="+orderID.String()+"&coins_to_use=18001", nil)
	previewReq = previewReq.WithContext(context.Background())
	previewCtx.Request = previewReq
	previewCtx.Set("userID", h.buyerID)
	h.handler.ListPaymentMethods(previewCtx)
	require.Equal(t, http.StatusBadRequest, previewTooMuch.Code)
	require.Contains(t, previewTooMuch.Body.String(), "coins_to_use exceeds max allowed (18000)")

	createTooMuch := h.runCreatePayment(t, orderID, "bank_transfer", 18001)
	require.Equal(t, http.StatusBadRequest, createTooMuch.Code)
	require.Contains(t, createTooMuch.Body.String(), "coins_to_use exceeds max allowed (18000)")

	createWayTooMuch := h.runCreatePayment(t, orderID, "bank_transfer", 28000)
	require.Equal(t, http.StatusBadRequest, createWayTooMuch.Code)
	require.Contains(t, createWayTooMuch.Body.String(), "coins_to_use exceeds max allowed (18000)")
}

func TestCreatePayment_ActiveIntentReuseConflictAndPercentagePreview(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderID := h.createOrder(t)
	preview := h.runListPaymentMethods(t, orderID, 10000)
	require.Equal(t, orderID.String(), preview.Data.OrderID)
	require.Equal(t, canonicalBuyerBase, preview.Data.BaseAmount)

	var qris paymentMethodPreview
	for i := range preview.Data.Methods {
		m := preview.Data.Methods[i]
		if m.MethodCode == "qris" {
			qris = m
			break
		}
	}
	require.Equal(t, "qris", qris.MethodCode, "qris should be listed")
	require.Equal(t, int64(10000), qris.CoinsToUse)
	require.Equal(t, int64(100000), qris.CashAmount)
	require.Equal(t, int64(700), qris.BuyerPaymentFeeAmount)
	require.Equal(t, int64(100700), qris.TotalPayableAmount)

	gateway.inspect = func(req *midtrans.SnapRequest) error {
		if got := int64(math.Round(req.TransactionDetails.GrossAmount)); got != 100700 {
			return fmt.Errorf("snap gross mismatch: got %d want 100700", got)
		}
		return nil
	}
	gateway.response = &midtrans.SnapResponse{Token: "tok-qris", RedirectURL: "https://midtrans.example/qris"}
	h.handler.midtransClient = gateway

	firstResp := h.runCreatePayment(t, orderID, "qris", 10000)
	require.Equal(t, http.StatusOK, firstResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	firstPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(100700), firstPayment.GrossAmount)
	require.Equal(t, int64(700), firstPayment.ServiceFeeAmount)
	require.Equal(t, int64(10000), firstPayment.CoinsToUse)
	require.Equal(t, int64(10000), firstPayment.CoinDiscountAmount)
	require.Equal(t, paymentrepo.PaymentStatusPending, firstPayment.Status)

	retrySameResp := h.runCreatePayment(t, orderID, "qris", 10000)
	require.Equal(t, http.StatusOK, retrySameResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	samePayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, firstPayment.ID, samePayment.ID)
	require.Equal(t, firstPayment.GrossAmount, samePayment.GrossAmount)
	require.Equal(t, firstPayment.ServiceFeeAmount, samePayment.ServiceFeeAmount)
	require.Equal(t, firstPayment.CoinsToUse, samePayment.CoinsToUse)
	require.Equal(t, firstPayment.CoinDiscountAmount, samePayment.CoinDiscountAmount)

	conflictKResp := h.runCreatePayment(t, orderID, "qris", 8000)
	require.Equal(t, http.StatusConflict, conflictKResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	conflictMethodResp := h.runCreatePayment(t, orderID, "bank_transfer", 10000)
	require.Equal(t, http.StatusConflict, conflictMethodResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	count, err := countPaymentsForOrder(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, firstPayment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, int64(10000), reservation.Amount)
	require.Equal(t, "reserved", reservation.Status)
}

func TestCreatePayment_RollbackWhenReservationInsertFails(t *testing.T) {
	ctx := context.Background()
	h := newPaymentIntentHarness(t, 20000, &recordingSnapGateway{}, nil)
	failingRepo := &failingReservationCoinsRepo{
		CoinsRepository:      h.coinsRepo,
		createReservationErr: errors.New("forced reservation failure"),
	}
	h.handler.coinsRepo = failingRepo

	orderID := h.createOrder(t)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	count, err := countPaymentsForOrder(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count, "payment row must roll back when reservation insert fails")

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)

	order, err := loadOrderSnapshotByID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(0), order.ServiceFeeAmount)
	require.Equal(t, canonicalBuyerBase, order.TotalPayableAmount)
	require.Equal(t, canonicalBuyerBase, order.TotalBeforeCoins)
	require.Equal(t, int64(0), order.CoinsUsed)
	require.Equal(t, int64(0), order.CoinDiscountAmount)
}

func TestCreatePayment_DefinitiveValidationRefusalCompensationAndUncertainOutcome(t *testing.T) {
	ctx := context.Background()
	h := newPaymentIntentHarness(t, 20000, &recordingSnapGateway{}, nil)

	definiteGateway := &recordingSnapGateway{
		tdb: h.tdb,
		err: &midtrans.APIError{
			Operation:  "create_snap",
			StatusCode: 400,
			Body:       `{"error_messages":["transaction_details.gross_amount is not equal to the sum of item_details"]}`,
		},
	}
	h.handler.midtransClient = definiteGateway
	definiteOrderID := h.createOrder(t)
	definiteResp := h.runCreatePayment(t, definiteOrderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, definiteResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&definiteGateway.calls))

	definitePayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, definiteOrderID)
	require.NoError(t, err)
	require.NotNil(t, definitePayment)
	require.Equal(t, paymentrepo.PaymentStatusDeny, definitePayment.Status)

	definiteReservation, err := loadReservationByPaymentID(ctx, h.tdb, definitePayment.ID)
	require.NoError(t, err)
	require.NotNil(t, definiteReservation)
	require.Equal(t, "released", definiteReservation.Status)
	require.Equal(t, int64(18000), definiteReservation.Amount)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)

	uncertainGateway := &recordingSnapGateway{
		tdb: h.tdb,
		err: errors.New("network timeout"),
	}
	h.handler.midtransClient = uncertainGateway
	uncertainOrderID := h.createOrder(t)
	uncertainResp := h.runCreatePayment(t, uncertainOrderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, uncertainResp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&uncertainGateway.calls))

	uncertainPayment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, uncertainOrderID)
	require.NoError(t, err)
	require.NotNil(t, uncertainPayment)
	require.Equal(t, paymentrepo.PaymentStatusPending, uncertainPayment.Status)
	require.Nil(t, uncertainPayment.PaymentURL)

	uncertainReservation, err := loadReservationByPaymentID(ctx, h.tdb, uncertainPayment.ID)
	require.NoError(t, err)
	require.NotNil(t, uncertainReservation)
	require.Equal(t, "reserved", uncertainReservation.Status)
	require.Equal(t, int64(18000), uncertainReservation.Amount)
}

func TestCreatePayment_CreateSnapDuplicateOrderID_StaysProtected(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{
		err: &midtrans.APIError{
			Operation:  "create_snap",
			StatusCode: 406,
			Body:       `{"error_messages":["transaction_details.order_id has been paid and utilized, please use another order ID"]}`,
		},
	}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderID := h.createOrder(t)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	payment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
	require.Nil(t, payment.PaymentURL)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestCreatePayment_CreateSnapUnknown4xx_StaysProtected(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{
		err: &midtrans.APIError{
			Operation:  "create_snap",
			StatusCode: 429,
			Body:       `{"error_messages":["too many requests"]}`,
		},
	}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderID := h.createOrder(t)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	payment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
	require.Nil(t, payment.PaymentURL)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestCreatePayment_CreateSnap5xx_StaysProtected(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{
		err: &midtrans.APIError{
			Operation:  "create_snap",
			StatusCode: 503,
			Body:       `{"error_messages":["Bank/partner is experiencing connection issue."]}`,
		},
	}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderID := h.createOrder(t)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	payment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
	require.Nil(t, payment.PaymentURL)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestCreatePayment_DefinitiveRefusalKZero_TerminalizesWithoutReservation(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{
		err: &midtrans.APIError{
			Operation:  "create_snap",
			StatusCode: 400,
			Body:       `{"error_messages":["transaction_details.gross_amount is not equal to the sum of item_details"]}`,
		},
	}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderID := h.createOrder(t)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 0)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	payment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusDeny, payment.Status)
	require.Equal(t, int64(0), payment.CoinsToUse)
	require.Nil(t, payment.PaymentURL)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.Nil(t, reservation)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestCreatePayment_DefinitiveRefusalCompensationRollbackLeavesPendingAndReserved(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{
		err: &midtrans.APIError{
			Operation:  "create_snap",
			StatusCode: 400,
			Body:       `{"error_messages":["transaction_details.gross_amount is not equal to the sum of item_details"]}`,
		},
	}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)
	h.handler.coinsRepo = &failingReleaseReservationCoinsRepo{
		CoinsRepository: h.coinsRepo,
		err:             errors.New("forced release failure"),
	}

	orderID := h.createOrder(t)
	resp := h.runCreatePayment(t, orderID, "bank_transfer", 18000)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls))

	payment, err := loadPaymentSnapshotByOrderID(ctx, h.tdb, orderID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
	require.Nil(t, payment.PaymentURL)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "reserved", reservation.Status)
	require.Equal(t, int64(18000), reservation.Amount)

	balance, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), balance)
}

func TestCreatePayment_ConcurrentOrders_RespectAvailableCoins(t *testing.T) {
	ctx := context.Background()
	gateway := &recordingSnapGateway{}
	h := newPaymentIntentHarness(t, 20000, gateway, nil)

	orderA := h.createOrder(t)
	orderB := h.createOrder(t)

	start := make(chan struct{})
	type result struct {
		code int
		body string
	}
	var (
		mu      sync.Mutex
		results []result
		wg      sync.WaitGroup
	)
	for _, orderID := range []uuid.UUID{orderA, orderB} {
		wg.Add(1)
		go func(orderID uuid.UUID) {
			defer wg.Done()
			<-start
			resp := h.runCreatePayment(t, orderID, "bank_transfer", 15000)
			mu.Lock()
			results = append(results, result{code: resp.Code, body: resp.Body.String()})
			mu.Unlock()
		}(orderID)
	}
	close(start)
	wg.Wait()

	require.Len(t, results, 2)
	var successCount, conflictCount int
	for _, res := range results {
		if res.code == http.StatusOK {
			successCount++
		}
		if res.code == http.StatusConflict {
			conflictCount++
		}
	}
	require.Equal(t, 1, successCount)
	require.Equal(t, 1, conflictCount)
	require.Equal(t, int32(1), atomic.LoadInt32(&gateway.calls), "only the financially valid request should reach Midtrans")

	payCountA, err := countPaymentsForOrder(ctx, h.tdb, orderA)
	require.NoError(t, err)
	payCountB, err := countPaymentsForOrder(ctx, h.tdb, orderB)
	require.NoError(t, err)
	require.Equal(t, int64(1), payCountA+payCountB)

	var totalReserved int64
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM coin_reservations
			WHERE status = 'reserved'
		`).Scan(&totalReserved)
	})
	require.NoError(t, err)
	require.Equal(t, int64(15000), totalReserved)
}
