//go:build integration

package serverboot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	orderapp "github.com/labuda/backend/internal/commerce/order/application"
	orderrepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	"github.com/labuda/backend/internal/config"
	escrowapp "github.com/labuda/backend/internal/core/escrow/application"
	financeapp "github.com/labuda/backend/internal/finance/application"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	coinsapp "github.com/labuda/backend/internal/incentive/coins/application"
	coinsinfrepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	coinsrepointf "github.com/labuda/backend/internal/incentive/coins/repository"
	paymentapp "github.com/labuda/backend/internal/integration/payment/application"
	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// ---------------------------------------------------------------------------
// Discovery integration test harness
// ---------------------------------------------------------------------------

type discoveryTestHarness struct {
	t              *testing.T
	tdb            *testdb.TestDB
	dbConn         *db.DB
	orderRepo      *orderrepo.OrderRepository
	paymentRepo    *paymentrepo.PaymentRepository
	escrowService  *escrowapp.EscrowService
	financeService *financeapp.FinanceService
	orderService   *orderapp.OrderService
	finalizer      *paymentapp.CanonicalFinalizationService
	refundService  *refundapp.RefundService
	webhookService *paymentapp.PaymentWebhookService
	midtransClient *midtrans.Client
	coinsRepo      coinsrepointf.CoinsRepository
	coinsService   *coinsapp.CoinsService
	buyerID        uuid.UUID
	sellerID       uuid.UUID
}

func newDiscoveryTestHarness(t *testing.T) *discoveryTestHarness {
	t.Helper()
	ctx := context.Background()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	// dbConn is a real *db.DB (WithTx retries serialization/deadlock errors).
	// It wraps the SAME pool as tdb, so both see the same disposable database.
	dbConn := db.NewFromPool(tdb.Pool())

	// Bootstrap system financial accounts
	financeBootstrap := financeapp.NewSystemAccountBootstrap(database.NewFromPgx(dbConn))
	_, err := financeBootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	orderRepo := orderrepo.NewOrderRepository()
	paymentRepo := paymentrepo.NewPaymentRepository()
	coinsRepo := coinsrepointf.CoinsRepository(coinsinfrepo.NewCoinsRepository())
	coinsService := coinsapp.NewCoinsService(coinsRepo, dbConn)
	escrowService := escrowapp.NewEscrowService(dbConn, zap.NewNop())
	financeService := financeapp.NewFinanceService()
	financeService.SetLogger(zap.NewNop())
	outboxRepo := repository.NewOutboxRepository(dbConn)
	orderService := orderapp.NewOrderService(
		nil, nil, outboxRepo, nil, coinsService, nil, nil, nil, nil, escrowService, nil,
	)
	orderService.PaymentService().SetFinanceReleaseRecorder(financeService)

	finalizer := paymentapp.NewCanonicalFinalizationService(financeService, orderService, escrowService, zap.NewNop())
	finalizer.SetCoinSpendConsumer(coinsService)

	// Real Midtrans client used ONLY for webhook signature building/verification.
	midtransLogger, err := logger.New("error", "json", "stdout")
	require.NoError(t, err)
	midtransClient := midtrans.NewClient(&config.MidtransConfig{
		ServerKey:   "test-server-key",
		ClientKey:   "test-client-key",
		Environment: "sandbox",
	}, midtransLogger)

	// REAL canonical webhook service — the same production actor used by the
	// HTTP webhook handler, wired to the same canonical finalizer.
	webhookService := paymentapp.NewPaymentWebhookService(
		dbConn, midtransClient, orderService, escrowService, zap.NewNop(),
	)
	webhookService.SetCanonicalFinalizationService(finalizer)
	webhookService.SetFinanceService(financeService)

	// REC-6: the canonical refund-intent authority is a MANDATORY dependency for
	// every producer. Drive it with the real RefundService so the REC-6
	// assertions exercise the production authority, not a fallback.
	refundService := refundapp.NewRefundService(escrowService, outboxRepo)
	refundService.SetOrderRefundStatusSyncer(orderService)
	webhookService.SetRefundService(refundService)

	buyerID := uuid.New()
	sellerID := uuid.New()
	require.NoError(t, insertPaymentIntentUsers(ctx, tdb, buyerID, sellerID))
	require.NoError(t, seedCanonicalPaymentMethods(ctx, tdb))

	return &discoveryTestHarness{
		t:              t,
		tdb:            tdb,
		dbConn:         dbConn,
		orderRepo:      orderRepo,
		paymentRepo:    paymentRepo,
		escrowService:  escrowService,
		financeService: financeService,
		orderService:   orderService,
		finalizer:      finalizer,
		refundService:  refundService,
		webhookService: webhookService,
		midtransClient: midtransClient,
		coinsRepo:      coinsRepo,
		coinsService:   coinsService,
		buyerID:        buyerID,
		sellerID:       sellerID,
	}
}

// newDiscoveryWorker constructs the discovery worker with the MANDATORY REC-6
// refund-intent authority already wired. No test may construct a worker without
// it while asserting that a REC-6 intent exists.
func (h *discoveryTestHarness) newDiscoveryWorker(
	gw worker.GatewayTransactionStatuser,
	finalizer worker.OrderPaymentFinalizer,
	subProcessor worker.SubscriptionPaymentProcessor,
) *worker.PaymentDiscoveryWorker {
	w := worker.NewPaymentDiscoveryWorker(h.dbConn, gw, finalizer, subProcessor, zap.NewNop(), discoveryCfg())
	w.SetRec6RefundCreator(h.refundService)
	return w
}

// countingOrderFinalizer wraps the canonical finalizer so a test can assert how
// many times finalization was ATTEMPTED. It does not add any behaviour — it
// delegates verbatim.
type countingOrderFinalizer struct {
	inner worker.OrderPaymentFinalizer
	calls atomic.Int32
}

func (c *countingOrderFinalizer) FinalizeOrderPayment(
	ctx context.Context, tx db.Tx, payment *paymentrepo.Payment, transactionID, paymentType string,
) error {
	c.calls.Add(1)
	return c.inner.FinalizeOrderPayment(ctx, tx, payment, transactionID, paymentType)
}

type discoveryFixture struct {
	OrderID       uuid.UUID
	Payment       *paymentrepo.Payment
	MidtransID    string
	GrossAmount   int64
	TransactionID string
}

func (h *discoveryTestHarness) createDiscoveryFixture(t *testing.T) *discoveryFixture {
	t.Helper()
	ctx := context.Background()

	orderID := createCanonicalOrder(t, ctx, h.tdb, h.orderRepo, h.buyerID, h.sellerID)
	orderSnap, err := loadOrderSnapshotByID(ctx, h.tdb, orderID)
	require.NoError(t, err)

	grossAmount := orderSnap.TotalBeforeCoins
	midtransID := fmt.Sprintf("MID-DISCOVERY-%s", uuid.New().String()[:8])
	paymentNumber := fmt.Sprintf("PAY-DISCOVERY-%s", uuid.New().String()[:8])
	methodCode := "bank_transfer"
	referenceID := orderID

	var payment *paymentrepo.Payment
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = h.paymentRepo.CreatePayment(ctx, tx, paymentrepo.CreatePaymentInput{
			UserID:            h.buyerID,
			PaymentNumber:     paymentNumber,
			MidtransOrderID:   midtransID,
			GrossAmount:       money.New(grossAmount),
			ServiceFeeAmount:  money.New(0),
			CoinsToUse:        0,
			ReferenceType:     paymentrepo.ReferenceTypeOrder,
			ReferenceID:       &referenceID,
			ExpiredAt:         time.Now().Add(1 * time.Hour),
			PaymentMethodCode: &methodCode,
		})
		return err
	})
	require.NoError(t, err)

	return &discoveryFixture{
		OrderID:       orderID,
		Payment:       payment,
		MidtransID:    midtransID,
		GrossAmount:   grossAmount,
		TransactionID: fmt.Sprintf("trx-disc-%s", uuid.New().String()[:8]),
	}
}

func (h *discoveryTestHarness) createSubscriptionDiscoveryFixture(t *testing.T) *discoveryFixture {
	t.Helper()
	ctx := context.Background()

	midtransID := fmt.Sprintf("MID-SUB-%s", uuid.New().String()[:8])
	paymentNumber := fmt.Sprintf("PAY-SUB-%s", uuid.New().String()[:8])
	methodCode := "bank_transfer"

	var payment *paymentrepo.Payment
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = h.paymentRepo.CreatePayment(ctx, tx, paymentrepo.CreatePaymentInput{
			UserID:            h.sellerID,
			PaymentNumber:     paymentNumber,
			MidtransOrderID:   midtransID,
			GrossAmount:       money.New(50000),
			ServiceFeeAmount:  money.New(0),
			CoinsToUse:        0,
			ReferenceType:     paymentrepo.ReferenceTypeSubscription,
			ReferenceID:       &h.sellerID,
			ExpiredAt:         time.Now().Add(1 * time.Hour),
			PaymentMethodCode: &methodCode,
		})
		return err
	})
	require.NoError(t, err)

	return &discoveryFixture{
		OrderID:       uuid.Nil,
		Payment:       payment,
		MidtransID:    midtransID,
		GrossAmount:   50000,
		TransactionID: fmt.Sprintf("trx-sub-%s", uuid.New().String()[:8]),
	}
}

// makeDiscoveryEligible backdates created_at so the payment passes the
// "10 minutes old" eligibility predicate.
func (h *discoveryTestHarness) makeDiscoveryEligible(t *testing.T, paymentID uuid.UUID) {
	t.Helper()
	require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		_, err := tx.Exec(context.Background(),
			`UPDATE payments SET created_at = NOW() - INTERVAL '15 minutes' WHERE id = $1`, paymentID)
		return err
	}))
}

func (h *discoveryTestHarness) settlementPayload(fx *discoveryFixture) *midtrans.NotificationPayload {
	return &midtrans.NotificationPayload{
		OrderID:           fx.MidtransID,
		TransactionStatus: string(midtrans.StatusSettlement),
		TransactionID:     fx.TransactionID,
		GrossAmount:       fmt.Sprintf("%d.00", fx.GrossAmount),
		FraudStatus:       "accept",
		PaymentType:       "bank_transfer",
	}
}

// makeSignedWebhookNotification builds a signature-valid settlement
// notification, exactly as Midtrans would deliver it.
func (h *discoveryTestHarness) makeSignedWebhookNotification(fx *discoveryFixture) *midtrans.NotificationPayload {
	notif := &midtrans.NotificationPayload{
		TransactionTime:   time.Now().Format(time.RFC3339),
		TransactionStatus: string(midtrans.StatusSettlement),
		TransactionID:     fx.TransactionID,
		StatusMessage:     "OK",
		StatusCode:        "200",
		PaymentType:       "bank_transfer",
		OrderID:           fx.MidtransID,
		GrossAmount:       fmt.Sprintf("%d.00", fx.GrossAmount),
		FraudStatus:       "accept",
		Currency:          "IDR",
	}
	notif.SignatureKey = h.midtransClient.BuildWebhookSignature(notif)
	return notif
}

func (h *discoveryTestHarness) loadPaymentByID(ctx context.Context, paymentID uuid.UUID) (*paymentrepo.Payment, error) {
	var payment *paymentrepo.Payment
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = h.paymentRepo.GetByID(ctx, tx, paymentID)
		return err
	})
	return payment, err
}

func (h *discoveryTestHarness) countEscrows(orderID uuid.UUID) int64 {
	var n int64
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM escrows WHERE order_id = $1`, orderID).Scan(&n)
	})
	require.NoError(h.t, err)
	return n
}

func (h *discoveryTestHarness) countCapturedAfterExpiry(midtransOrderID string) int64 {
	var n int64
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM payment_webhook_events
			WHERE midtrans_order_id = $1 AND status = 'captured_after_expiry'
		`, midtransOrderID).Scan(&n)
	})
	require.NoError(h.t, err)
	return n
}

// countRec6RefundIntents counts refund intents created by REC-6 for an order.
func (h *discoveryTestHarness) countRec6RefundIntents(orderID uuid.UUID) int64 {
	var n int64
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM refunds
			WHERE order_id = $1 AND reason = 'gateway_captured_after_order_invalid'
		`, orderID).Scan(&n)
	})
	require.NoError(h.t, err)
	return n
}

func (h *discoveryTestHarness) loadOrderStatus(orderID uuid.UUID) string {
	var status string
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
	})
	require.NoError(h.t, err)
	return status
}

// ---------------------------------------------------------------------------
// Instrumented gateway: hooks for deterministic race construction
// ---------------------------------------------------------------------------

type integrationGateway struct {
	responses map[string]*midtrans.NotificationPayload
	errors    map[string]error
	called    int64
	mu        sync.Mutex

	// inquiryArrived, when non-nil, receives the orderID of EVERY inquiry so a
	// test can align actors deterministically.
	inquiryArrived chan string
	// release, when non-nil, parks the inquiry until the test closes it. This
	// is the window in which a second worker / the webhook runs while the
	// caller holds NO database lock (the discovery transaction already
	// committed).
	release chan struct{}
	// onInquiry, when non-nil, runs synchronously at inquiry time. Used to
	// mutate canonical state mid-flight (the expiry/cancellation race).
	onInquiry func(orderID string)
}

func newIntegrationGateway() *integrationGateway {
	return &integrationGateway{
		responses: make(map[string]*midtrans.NotificationPayload),
		errors:    make(map[string]error),
	}
}

func (g *integrationGateway) setSuccess(midtransOrderID string, payload *midtrans.NotificationPayload) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.responses[midtransOrderID] = payload
}

func (g *integrationGateway) setError(midtransOrderID string, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.errors[midtransOrderID] = err
}

func (g *integrationGateway) QueryProviderState(orderID string) (*midtrans.ProviderStatus, error) {
	atomic.AddInt64(&g.called, 1)

	// Hooks run OUTSIDE the mutex and OUTSIDE any DB transaction the caller
	// may hold — the worker never calls the gateway while locked.
	if g.onInquiry != nil {
		g.onInquiry(orderID)
	}
	if g.inquiryArrived != nil {
		select {
		case g.inquiryArrived <- orderID:
		default:
		}
	}
	if g.release != nil {
		<-g.release
	}

	g.mu.Lock()
	err, hasErr := g.errors[orderID]
	resp, hasResp := g.responses[orderID]
	g.mu.Unlock()

	if hasErr {
		return nil, err
	}
	if hasResp {
		if resp == nil {
			return &midtrans.ProviderStatus{State: midtrans.ProviderStateNotPresent}, nil
		}
		return &midtrans.ProviderStatus{State: resp.ProviderState(), Notification: resp}, nil
	}
	pending := &midtrans.NotificationPayload{
		OrderID:           orderID,
		TransactionStatus: string(midtrans.StatusPending),
		GrossAmount:       "0.00",
	}
	return &midtrans.ProviderStatus{State: midtrans.ProviderStatePending, Notification: pending}, nil
}

func (g *integrationGateway) getCallCount() int64 {
	return atomic.LoadInt64(&g.called)
}

func discoveryCfg() worker.PaymentDiscoveryConfig {
	return worker.PaymentDiscoveryConfig{
		PollInterval:          time.Hour, // ScanOnce is called directly
		BatchSize:             100,
		InquiryEligibilityAge: 1 * time.Millisecond,
		GatewayTimeout:        10 * time.Second,
	}
}

// ---------------------------------------------------------------------------
// Scenario A — pending + gateway settlement → canonical order finalization
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioA_SettlementFinalizesOrder(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	w.ScanOnce()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	assert.Equal(t, "paid", h.loadOrderStatus(fx.OrderID))
	assert.Equal(t, int64(1), h.countEscrows(fx.OrderID), "exactly one escrow created")
	assert.Equal(t, int64(1), gw.getCallCount())
}

// ---------------------------------------------------------------------------
// Scenario B — pending + gateway failure → canonical failure
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioB_GatewayFailure(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	payload := h.settlementPayload(fx)
	payload.TransactionStatus = string(midtrans.StatusDeny)
	gw.setSuccess(fx.MidtransID, payload)
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	w.ScanOnce()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusDeny, payment.Status)
	assert.Equal(t, int64(0), h.countEscrows(fx.OrderID), "no escrow on failure")
	assert.Equal(t, int64(1), gw.getCallCount())
}

// ---------------------------------------------------------------------------
// Scenario C — two discovery workers, same payment → exactly one financial effect
//
// The discovery lock is RELEASED before the gateway inquiry, so SKIP LOCKED does
// NOT serialize the whole workflow. This test DETERMINISTICALLY parks worker 1
// inside its gateway inquiry (no DB lock held), lets worker 2 discover the SAME
// payment, then releases both into the finalization transaction together. The
// single-financial-effect guarantee therefore comes from the finalization
// transaction's ORDER→PAYMENT row locks + re-read + conditional status
// transition, NOT from SKIP LOCKED.
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioC_TwoWorkersOnePayment(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	gw.inquiryArrived = make(chan string, 4)
	gw.release = make(chan struct{})
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	counting := &countingOrderFinalizer{inner: h.finalizer}
	w1 := h.newDiscoveryWorker(gw, counting, nil)
	w2 := h.newDiscoveryWorker(gw, counting, nil)

	done := make(chan struct{})
	go func() { defer close(done); w1.ScanOnce() }()

	// Wait until worker 1 is parked inside the gateway inquiry (lock released).
	select {
	case <-gw.inquiryArrived:
	case <-time.After(20 * time.Second):
		t.Fatal("worker 1 never reached the gateway inquiry")
	}

	done2 := make(chan struct{})
	go func() { defer close(done2); w2.ScanOnce() }()

	// Worker 2 must ALSO discover the same payment now that worker 1 holds no lock.
	select {
	case <-gw.inquiryArrived:
	case <-time.After(20 * time.Second):
		t.Fatal("worker 2 never reached the gateway inquiry (SKIP LOCKED did not dedup)")
	}

	// Release both; they race into the finalization transaction.
	close(gw.release)
	<-done
	<-done2

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	assert.Equal(t, int64(1), h.countEscrows(fx.OrderID), "exactly one escrow despite two workers racing")
	assert.Equal(t, int32(1), counting.calls.Load(), "canonical finalizer invoked exactly once")
	assert.GreaterOrEqual(t, gw.getCallCount(), int64(2), "both workers performed a gateway inquiry")
}

// ---------------------------------------------------------------------------
// Scenario D — discovery worker VS real PaymentWebhookService → one effect
//
// This is a genuine worker-vs-webhook race: the second actor is the canonical
// PaymentWebhookService.HandleWebhook (the production HTTP webhook path), not a
// second worker. The worker is parked in its gateway inquiry (no lock) while the
// webhook commits settlement; the worker then re-enters finalization and must
// not duplicate the effect.
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioD_WorkerPlusWebhook(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	gw.inquiryArrived = make(chan string, 4)
	gw.release = make(chan struct{})
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)

	workerDone := make(chan struct{})
	go func() { defer close(workerDone); w.ScanOnce() }()

	// Park the worker inside its gateway inquiry. At this point it holds NO DB lock.
	select {
	case <-gw.inquiryArrived:
	case <-time.After(20 * time.Second):
		t.Fatal("worker never reached the gateway inquiry")
	}

	// The real webhook settles the SAME payment while the worker is in flight.
	notif := h.makeSignedWebhookNotification(fx)
	require.NoError(t, h.webhookService.HandleWebhook(ctx, notif, "127.0.0.1"))

	// Release the worker; its finalization transaction must observe the settled
	// payment and produce no second financial effect.
	close(gw.release)
	<-workerDone

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	assert.Equal(t, int64(1), h.countEscrows(fx.OrderID), "exactly one escrow across worker + webhook")

	// The webhook's own event identity must be preserved (one succeeded row).
	var succeeded int64
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM payment_webhook_events
			WHERE midtrans_order_id = $1 AND status = 'succeeded'
		`, fx.MidtransID).Scan(&succeeded)
	}))
	assert.Equal(t, int64(1), succeeded, "webhook event recorded once as succeeded")
}

// TestPaymentDiscovery_Integration_ScenarioD2_WorkerAndWebhookTrulyConcurrent
// runs both actors from a common start line with no artificial ordering.
func TestPaymentDiscovery_Integration_ScenarioD2_WorkerAndWebhookTrulyConcurrent(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	notif := h.makeSignedWebhookNotification(fx)

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; w.ScanOnce() }()
	go func() { defer wg.Done(); <-start; _ = h.webhookService.HandleWebhook(ctx, notif, "127.0.0.1") }()
	close(start)
	wg.Wait()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	assert.Equal(t, int64(1), h.countEscrows(fx.OrderID), "exactly one escrow under a true concurrent race")
}

// ---------------------------------------------------------------------------
// Scenario E — gateway settlement + order EXPIRES mid-flight → must NOT settle
//
// The order is flipped to its canonical expired state DURING the gateway
// inquiry, proving the finalization transaction re-reads order state after
// acquiring locks rather than trusting the discovery snapshot.
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioE_OrderExpiresMidFlight(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	gw.onInquiry = func(orderID string) {
		require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
			_, err := tx.Exec(context.Background(), `
				UPDATE orders SET status = 'expired', payment_expires_at = NOW() - INTERVAL '1 hour'
				WHERE id = $1
			`, fx.OrderID)
			return err
		}))
	}
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	w.ScanOnce()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.NotEqual(t, paymentrepo.PaymentStatusSettlement, payment.Status,
		"payment must NOT settle when the order expired mid-flight")
	assert.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
	assert.Equal(t, int64(0), h.countEscrows(fx.OrderID), "no escrow may be created")
	// REC-6 SLICE 1: refund intent must be created instead of captured_after_expiry
	assert.Equal(t, int64(1), h.countRec6RefundIntents(fx.OrderID),
		"REC-6 refund intent must be created for expired order")
	assert.Nil(t, h.rec6RefundReviewedBy(fx.OrderID),
		"automatic system refund must persist reviewed_by as NULL")
}

// ---------------------------------------------------------------------------
// Scenario F — gateway settlement + order CANCELLED mid-flight → must NOT settle
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioF_OrderCancelledMidFlight(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	gw.onInquiry = func(orderID string) {
		require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
			_, err := tx.Exec(context.Background(),
				`UPDATE orders SET status = 'cancelled' WHERE id = $1`, fx.OrderID)
			return err
		}))
	}
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	w.ScanOnce()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusPending, payment.Status,
		"payment must NOT settle when the order was cancelled mid-flight")
	assert.Equal(t, int64(0), h.countEscrows(fx.OrderID), "no escrow may be created")
	// REC-6 SLICE 1: refund intent must be created instead of captured_after_expiry
	assert.Equal(t, int64(1), h.countRec6RefundIntents(fx.OrderID),
		"REC-6 refund intent must be created for cancelled order")
	assert.Nil(t, h.rec6RefundReviewedBy(fx.OrderID),
		"automatic system refund must persist reviewed_by as NULL")
}

// ---------------------------------------------------------------------------
// Scenario G — retry after successful finalization → no duplicate effect
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioG_RetryAfterFinalization(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	counting := &countingOrderFinalizer{inner: h.finalizer}
	w := h.newDiscoveryWorker(gw, counting, nil)

	w.ScanOnce()
	payment1, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment1.Status)
	escrowAfterFirst := h.countEscrows(fx.OrderID)
	callsAfterFirst := counting.calls.Load()

	w.ScanOnce()
	payment2, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment2.Status)
	assert.Equal(t, escrowAfterFirst, h.countEscrows(fx.OrderID), "no duplicate escrow on retry")
	assert.Equal(t, callsAfterFirst, counting.calls.Load(), "no extra finalization attempt on retry")
}

// ---------------------------------------------------------------------------
// Scenario H — subscription: payment settled FIRST, then canonical processor
// ---------------------------------------------------------------------------

type recordingSubscriptionProcessor struct {
	tdb           *testdb.TestDB
	callCount     atomic.Int32
	statusAtCall  atomic.Value // string
	mu            sync.Mutex
	lastPaymentID uuid.UUID
}

func (r *recordingSubscriptionProcessor) ProcessSuccessfulPayment(
	ctx context.Context, paymentID uuid.UUID, userID uuid.UUID, providerEventID string,
) error {
	r.callCount.Add(1)
	// Capture the payment status AS OBSERVED at processor invocation time. This
	// proves the worker settled the payment BEFORE handing off to the canonical
	// subscription activation.
	var status string
	if err := r.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, paymentID).Scan(&status)
	}); err == nil {
		r.statusAtCall.Store(status)
	}
	r.mu.Lock()
	r.lastPaymentID = paymentID
	r.mu.Unlock()
	return nil
}

func TestPaymentDiscovery_Integration_ScenarioH_SubscriptionSettlesThenActivates(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createSubscriptionDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	h.makeDiscoveryEligible(t, fx.Payment.ID)

	subProcessor := &recordingSubscriptionProcessor{tdb: h.tdb}
	w := h.newDiscoveryWorker(gw, h.finalizer, subProcessor)
	w.ScanOnce()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	assert.Equal(t, int32(1), subProcessor.callCount.Load(), "canonical subscription processor invoked once")
	assert.Equal(t, fx.Payment.ID, subProcessor.lastPaymentID)

	statusAtCall, _ := subProcessor.statusAtCall.Load().(string)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, statusAtCall,
		"payment MUST already be settled when the subscription processor is invoked")

	// The worker must NOT have inserted a subscription row itself.
	var subs int64
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM seller_subscriptions`).Scan(&subs)
	}))
	assert.Equal(t, int64(0), subs, "worker must not insert subscription rows directly")
}

// ---------------------------------------------------------------------------
// Scenario I — candidate A gateway error → candidate B still processed
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ScenarioI_CandidateAErrorCandidateBProceeds(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()

	fxA := h.createDiscoveryFixture(t)
	fxB := h.createDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setError(fxA.MidtransID, fmt.Errorf("circuit breaker open"))
	gw.setSuccess(fxB.MidtransID, h.settlementPayload(fxB))

	h.makeDiscoveryEligible(t, fxA.Payment.ID)
	h.makeDiscoveryEligible(t, fxB.Payment.ID)

	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	w.ScanOnce()

	payA, err := h.loadPaymentByID(ctx, fxA.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusPending, payA.Status, "candidate A stays pending on gateway error")

	payB, err := h.loadPaymentByID(ctx, fxB.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payB.Status, "candidate B still settled")
}

// ---------------------------------------------------------------------------
// EXPLAIN ANALYZE — actual plan against disposable PostgreSQL
// ---------------------------------------------------------------------------

func TestPaymentDiscovery_Integration_ExplainAnalyze_CandidateQuery(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()

	for i := 0; i < 200; i++ {
		midtransID := fmt.Sprintf("MID-EXPLAIN-%d-%s", i, uuid.New().String()[:8])
		paymentNumber := fmt.Sprintf("PAY-EXPLAIN-%d-%s", i, uuid.New().String()[:8])
		methodCode := "bank_transfer"
		referenceID := uuid.New()

		createdAt := "NOW()"
		if i < 150 {
			createdAt = "NOW() - INTERVAL '15 minutes'"
		}
		expiredAt := "NOW() + INTERVAL '1 hour'"
		if i%10 == 0 {
			expiredAt = "NOW() - INTERVAL '1 hour'"
		}

		require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO payments (
					id, user_id, payment_number, midtrans_order_id,
					gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
					status, reference_type, reference_id,
					expired_at, created_at, updated_at, payment_method_code
				) VALUES (
					$1, $2, $3, $4,
					50000, 0, 0, 0,
					'pending', 'order', $5,
					%s, %s, NOW(), $6
				)`, expiredAt, createdAt),
				uuid.New(), h.buyerID, paymentNumber, midtransID, referenceID, &methodCode,
			)
			return err
		}))
	}

	planLines := explainCandidateQuery(t, h, ctx)
	plan := strings.Join(planLines, "\n")
	t.Logf("EXPLAIN ANALYZE output:\n%s", plan)

	assert.Contains(t, plan, "LockRows", "plan must contain LockRows for FOR UPDATE SKIP LOCKED")
	assert.Contains(t, plan, "payments", "plan must reference the payments table")

	var actualCount int64
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM (
				SELECT id FROM payments
				WHERE status = $1
				  AND created_at < NOW() - $2::interval
				  AND expired_at > NOW()
				FOR UPDATE SKIP LOCKED
				LIMIT $3
			) sub
		`, "pending", "600 seconds", 100).Scan(&actualCount)
	}))
	assert.Equal(t, int64(100), actualCount, "LIMIT 100 caps the batch")
}

func TestPaymentDiscovery_Integration_ExplainAnalyze_IndexInventory(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()

	var indexInfo string
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT string_agg(indexdef, E'\n' ORDER BY indexname)
			FROM pg_indexes WHERE tablename = 'payments'
		`).Scan(&indexInfo)
	}))
	t.Logf("Payments table indexes:\n%s", indexInfo)
	// OBSERVED: these indexes exist. This test does NOT claim they are chosen by
	// the planner at any given scale.
	assert.Contains(t, indexInfo, "idx_payments_status")
	assert.Contains(t, indexInfo, "idx_payments_expires_at")

	plan := strings.Join(explainCandidateQuery(t, h, ctx), "\n")
	assert.Contains(t, plan, "LockRows")
	t.Logf("Candidate query plan:\n%s", plan)
}

// ---------------------------------------------------------------------------
// REC-6 SLICE 1 — runtime/integration proof (webhook / orphan / duplicate)
// ---------------------------------------------------------------------------

// setOrderStatus flips the canonical order status directly (test fixture only).
func (h *discoveryTestHarness) setOrderStatus(orderID uuid.UUID, status string) {
	h.t.Helper()
	require.NoError(h.t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE orders SET status = $2 WHERE id = $1`, orderID, status)
		return err
	}))
}

// expireOrderPaymentWindow pushes the order's payment window into the past.
func (h *discoveryTestHarness) expireOrderPaymentWindow(orderID uuid.UUID) {
	h.t.Helper()
	require.NoError(h.t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE orders SET payment_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, orderID)
		return err
	}))
}

// setPaymentStatus flips the payment row status directly (test fixture only).
func (h *discoveryTestHarness) setPaymentStatus(paymentID uuid.UUID, status string) {
	h.t.Helper()
	require.NoError(h.t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE payments SET status = $2 WHERE id = $1`, paymentID, status)
		return err
	}))
}

// loadRec6Refund reads the single REC-6 refund row for an order.
func (h *discoveryTestHarness) loadRec6Refund(orderID uuid.UUID) (amount int64, idemKey string, status string, reason string) {
	h.t.Helper()
	var key *string
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `
			SELECT requested_amount, gateway_idempotency_key, status, reason
			FROM refunds
			WHERE order_id = $1 AND reason = 'gateway_captured_after_order_invalid'
		`, orderID).Scan(&amount, &key, &status, &reason)
	})
	require.NoError(h.t, err)
	if key != nil {
		idemKey = *key
	}
	return amount, idemKey, status, reason
}

// rec6RefundReviewedBy returns refunds.reviewed_by for the REC-6 refund of an
// order (nil == SQL NULL). Automatic system refunds MUST be NULL.
func (h *discoveryTestHarness) rec6RefundReviewedBy(orderID uuid.UUID) *uuid.UUID {
	h.t.Helper()
	var reviewedBy *uuid.UUID
	require.NoError(h.t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `
			SELECT reviewed_by FROM refunds
			WHERE order_id = $1 AND reason = 'gateway_captured_after_order_invalid'
		`, orderID).Scan(&reviewedBy)
	}))
	return reviewedBy
}

// rec6RefundSellerID returns refunds.seller_id for the REC-6 refund of an order.
func (h *discoveryTestHarness) rec6RefundSellerID(orderID uuid.UUID) uuid.UUID {
	h.t.Helper()
	var sellerID uuid.UUID
	require.NoError(h.t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `
			SELECT seller_id FROM refunds
			WHERE order_id = $1 AND reason = 'gateway_captured_after_order_invalid'
		`, orderID).Scan(&sellerID)
	}))
	return sellerID
}

// webhookEventStatus reads one stored notification's status by its identity.
func (h *discoveryTestHarness) webhookEventStatus(notif *midtrans.NotificationPayload) string {
	h.t.Helper()
	var status string
	require.NoError(h.t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT status FROM payment_webhook_events WHERE notification_key = $1`,
			midtrans.NotificationIdentity(notif)).Scan(&status)
	}))
	return status
}

// Scenario J — WEBHOOK: gateway SUCCESS arrives while the order is cancelled.
func TestPaymentDiscovery_Integration_ScenarioJ_WebhookSuccessOrderInvalid(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	h.setOrderStatus(fx.OrderID, "cancelled")

	notif := h.makeSignedWebhookNotification(fx)
	require.NoError(t, h.webhookService.HandleWebhook(ctx, notif, "127.0.0.1"))

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusPending, payment.Status,
		"invalid order must NOT be finalized")
	assert.Equal(t, int64(0), h.countEscrows(fx.OrderID), "no escrow for invalid order")
	assert.Equal(t, int64(1), h.countRec6RefundIntents(fx.OrderID),
		"webhook must create exactly one REC-6 refund intent")

	amount, idemKey, refundStatus, reason := h.loadRec6Refund(fx.OrderID)
	assert.Equal(t, fx.GrossAmount, amount, "refund amount must equal payment.gross_amount")
	assert.Equal(t, refundapp.Rec6IdempotencyKey(fx.Payment.ID), idemKey,
		"idempotency key must be rec6:payment:<payment_id>")
	assert.Equal(t, "system_refunded", refundStatus)
	assert.Equal(t, "gateway_captured_after_order_invalid", reason)
	assert.Nil(t, h.rec6RefundReviewedBy(fx.OrderID),
		"automatic system refund must persist reviewed_by as NULL (no human reviewer)")

	assert.Equal(t, "succeeded", h.webhookEventStatus(notif),
		"webhook event is succeeded only because the intent was created")

	// A second, distinct notification of the same gateway transaction must be
	// idempotent — no second refund intent.
	dup := h.makeSignedWebhookNotification(fx)
	dup.TransactionID = fx.TransactionID + "-dup"
	dup.SignatureKey = h.midtransClient.BuildWebhookSignature(dup)
	require.NoError(t, h.webhookService.HandleWebhook(ctx, dup, "127.0.0.1"))
	assert.Equal(t, int64(1), h.countRec6RefundIntents(fx.OrderID),
		"duplicate late success must not create a second REC-6 intent")
}

// INPUT CONVERGENCE — the removed orphan-recovery input, delivered through the
// canonical webhook endpoint.
//
// The orphan recovery worker existed for exactly one input: a gateway SUCCESS
// notification whose payment row was not visible when the event was first
// stored. That input is delivered here through the canonical producer, and the
// business result is asserted to be identical to what the worker produced: the
// same REC-6 refund intent, the same canonical order seller attribution, the
// same event status, and the same idempotency on redelivery. Nothing the worker
// did is lost by removing it.
func TestPaymentDiscovery_Integration_ScenarioK_LateSuccessOrderInvalidViaWebhook(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	h.setPaymentStatus(fx.Payment.ID, paymentrepo.PaymentStatusDeny)
	h.setOrderStatus(fx.OrderID, "expired")

	notif := h.makeSignedWebhookNotification(fx)
	require.NoError(t, h.webhookService.HandleWebhook(ctx, notif, "127.0.0.1"))

	assert.Equal(t, int64(1), h.countRec6RefundIntents(fx.OrderID),
		"late SUCCESS for an invalid order must create exactly one REC-6 refund intent")
	amount, idemKey, _, _ := h.loadRec6Refund(fx.OrderID)
	assert.Equal(t, fx.GrossAmount, amount, "refund amount must equal payment.gross_amount")
	assert.Equal(t, refundapp.Rec6IdempotencyKey(fx.Payment.ID), idemKey)
	assert.Equal(t, h.sellerID, h.rec6RefundSellerID(fx.OrderID),
		"refund seller must be the canonical order seller_id")
	assert.Nil(t, h.rec6RefundReviewedBy(fx.OrderID),
		"automatic system refund must persist reviewed_by as NULL")
	assert.Equal(t, "succeeded", h.webhookEventStatus(notif),
		"event is marked succeeded (only after the intent exists)")

	// A second, distinct redelivery of the same gateway transaction is
	// idempotent.
	dup := h.makeSignedWebhookNotification(fx)
	dup.TransactionID = fx.TransactionID + "-dup"
	dup.SignatureKey = h.midtransClient.BuildWebhookSignature(dup)
	require.NoError(t, h.webhookService.HandleWebhook(ctx, dup, "127.0.0.1"))
	assert.Equal(t, int64(1), h.countRec6RefundIntents(fx.OrderID),
		"duplicate late success must not create a second REC-6 intent")
}

// Scenario L — DUPLICATE-SAFE: an already-paid order must NOT be converted
// into a REC-6 refund intent, even after the payment window elapsed.
func TestPaymentDiscovery_Integration_ScenarioL_PaidDuplicateNoIntent(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	h.setOrderStatus(fx.OrderID, "paid")
	h.expireOrderPaymentWindow(fx.OrderID)
	h.setPaymentStatus(fx.Payment.ID, paymentrepo.PaymentStatusSettlement)

	notif := h.makeSignedWebhookNotification(fx)
	require.NoError(t, h.webhookService.HandleWebhook(ctx, notif, "127.0.0.1"))

	assert.Equal(t, int64(0), h.countRec6RefundIntents(fx.OrderID),
		"already-paid duplicate SUCCESS must NOT create a REC-6 refund intent")

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status,
		"duplicate must not change the settled payment")
	assert.Equal(t, int64(0), h.countEscrows(fx.OrderID), "duplicate must not create escrow")
}

func explainCandidateQuery(t *testing.T, h *discoveryTestHarness, ctx context.Context) []string {
	t.Helper()
	var planLines []string
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
			SELECT id FROM payments
			WHERE status = $1
			  AND created_at < NOW() - $2::interval
			  AND expired_at > NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		`, "pending", "600 seconds", 100)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				return err
			}
			planLines = append(planLines, line)
		}
		return rows.Err()
	}))
	return planLines
}
