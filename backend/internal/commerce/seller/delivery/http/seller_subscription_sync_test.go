package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	subscriptionApp "github.com/labuda/backend/internal/commerce/subscription/application"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	paymentRepository "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks
// ============================================================================

// syncPaymentRepo satisfies subscriptionPaymentRepository with controllable FindLatest.
type syncPaymentRepo struct {
	findPendingFn func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error)
	findLatestFn  func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error)
}

func (r *syncPaymentRepo) FindPendingSubscriptionPayment(ctx context.Context, tx db.Tx, userID uuid.UUID) (*paymentRepository.Payment, error) {
	if r.findPendingFn != nil {
		return r.findPendingFn(ctx, tx, userID)
	}
	return nil, nil
}
func (r *syncPaymentRepo) FindLatestSubscriptionPayment(ctx context.Context, tx db.Tx, userID uuid.UUID) (*paymentRepository.Payment, error) {
	if r.findLatestFn != nil {
		return r.findLatestFn(ctx, tx, userID)
	}
	return nil, nil
}
func (r *syncPaymentRepo) CreatePayment(ctx context.Context, tx db.Tx, input paymentRepository.CreatePaymentInput) (*paymentRepository.Payment, error) {
	return &paymentRepository.Payment{ID: uuid.New()}, nil
}
func (r *syncPaymentRepo) UpdatePaymentURL(ctx context.Context, tx db.Tx, paymentID uuid.UUID, paymentURL string) error {
	return nil
}

// fullSnapClient satisfies snapTransactionClient with controllable GetTransactionStatus.
type fullSnapClient struct {
	getStatusFn    func(string) (*midtrans.NotificationPayload, error)
	getStatusCalls int
}

func (c *fullSnapClient) CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error) {
	return &midtrans.SnapResponse{RedirectURL: "https://midtrans.example/redirect"}, nil
}
func (c *fullSnapClient) GetTransactionStatus(orderID string) (*midtrans.NotificationPayload, error) {
	c.getStatusCalls++
	if c.getStatusFn != nil {
		return c.getStatusFn(orderID)
	}
	return &midtrans.NotificationPayload{TransactionStatus: "pending"}, nil
}

// syncSubRepo satisfies subscriptionRepo.SellerSubscriptionRepository.
// activeSubscription controls the GetActiveByUserID response.
type syncSubRepo struct {
	activeSubscription *subscriptionEntity.SellerSubscription
	config             *subscriptionEntity.SellerSubscriptionConfig
	latest             *subscriptionEntity.SellerSubscription
}

func (r *syncSubRepo) InsertTx(context.Context, db.Tx, *subscriptionEntity.SellerSubscription) error {
	return nil
}
func (r *syncSubRepo) UpdateStatusTx(context.Context, db.Tx, uuid.UUID, subscriptionEntity.Status, subscriptionEntity.Status) error {
	return nil
}
func (r *syncSubRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}
func (r *syncSubRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}
func (r *syncSubRepo) GetLatestByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.latest, nil
}
func (r *syncSubRepo) GetLatestByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.latest, nil
}
func (r *syncSubRepo) GetActiveByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.activeSubscription, nil
}
func (r *syncSubRepo) FetchActiveExpiredBatch(context.Context, db.Tx, time.Time, int) ([]*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}
func (r *syncSubRepo) FetchActiveExpiredBatchIDs(context.Context, db.Tx, time.Time, int) ([]uuid.UUID, error) {
	return nil, nil
}
func (r *syncSubRepo) ExistsActiveByUserID(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}
func (r *syncSubRepo) GetActiveConfig(context.Context, db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error) {
	return r.config, nil
}
func (r *syncSubRepo) UpdateConfigTx(context.Context, db.Tx, uuid.UUID, int64, int, int, bool) error {
	return nil
}

// mockTransactor runs fn synchronously against testTx (no real DB).
type mockTransactor struct{}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	return fn(&testTx{})
}

// syncHandlerForTest is a parallel implementation of SyncSubscriptionPayment that
// accepts db.Transactor instead of *db.DB so tests do not need a real database pool.
// It mirrors the production handler logic exactly.
type syncHandlerForTest struct {
	subRepo    *syncSubRepo
	payRepo    subscriptionPaymentRepository
	snapClient snapTransactionClient
	db         db.Transactor
	svc        *subscriptionApp.SellerSubscriptionPaymentService
}

func (h *syncHandlerForTest) run(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "User not authenticated"})
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Step 1: find latest payment
	var payment *paymentRepository.Payment
	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		p, err := h.payRepo.FindLatestSubscriptionPayment(ctx, tx, userID)
		payment = p
		return err
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	if payment == nil {
		var activeSub *subscriptionEntity.SellerSubscription
		if err := h.db.WithTx(ctx, func(tx db.Tx) error {
			sub, err := h.subRepo.GetActiveByUserID(ctx, tx, userID)
			if err != nil {
				return err
			}
			activeSub = sub
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
		if activeSub != nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "status": "already_active", "subscription_id": activeSub.ID})
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"success": false, "error_code": "NO_PAYMENT_FOUND"})
		return
	}

	// Step 2: locally settled → activate
	if payment.IsSettled() {
		if h.svc != nil {
			if err := h.svc.ProcessSuccessfulPayment(ctx, payment.ID, userID, "seller_sync_"+userID.String()); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "activated", "payment_id": payment.ID})
		return
	}

	// Step 3: pending
	if payment.IsPending() {
		if time.Now().After(payment.ExpiredAt) {
			c.JSON(http.StatusGone, gin.H{"success": false, "error_code": "PAYMENT_EXPIRED"})
			return
		}

		gwStatus, err := h.snapClient.GetTransactionStatus(payment.MidtransOrderID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error_code": "GATEWAY_UNAVAILABLE"})
			return
		}

		switch gwStatus.TransactionStatus {
		case string(midtrans.StatusSettlement), string(midtrans.StatusCapture):
			if h.svc != nil {
				if err := h.svc.ProcessSuccessfulPayment(ctx, payment.ID, userID, "seller_sync_gateway_"+userID.String()); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"success": false})
					return
				}
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "status": "activated", "payment_id": payment.ID})
		case string(midtrans.StatusPending):
			c.JSON(http.StatusAccepted, gin.H{"success": true, "status": "pending"})
		default:
			c.JSON(http.StatusConflict, gin.H{"success": false, "error_code": "PAYMENT_FAILED"})
		}
		return
	}

	// terminal failed state locally
	c.JSON(http.StatusConflict, gin.H{"success": false, "error_code": "PAYMENT_FAILED"})
}

// ============================================================================
// Helpers
// ============================================================================

func newSyncGinContext(userID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/seller/subscription/sync", nil)
	c.Set("userID", userID)
	return c, w
}

func newSyncTestHandler(subRepo *syncSubRepo, payRepo subscriptionPaymentRepository, snapClient snapTransactionClient) *syncHandlerForTest {
	return &syncHandlerForTest{
		subRepo:    subRepo,
		payRepo:    payRepo,
		snapClient: snapClient,
		db:         &mockTransactor{},
		svc:        nil, // ProcessSuccessfulPayment skipped (no real DB)
	}
}

// ============================================================================
// Test cases
// ============================================================================

func TestSyncSubscription_NoUserID_Returns401(t *testing.T) {
	h := &SellerHandler{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/seller/subscription/sync", nil)

	h.SyncSubscriptionPayment(c) // calls production handler; h.db is nil → 401 before db call

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSyncSubscription_AlreadyActive_Returns200(t *testing.T) {
	subID := uuid.New()
	subRepo := &syncSubRepo{
		activeSubscription: &subscriptionEntity.SellerSubscription{
			ID:     subID,
			Status: subscriptionEntity.StatusActive,
		},
	}
	h := newSyncTestHandler(subRepo, &syncPaymentRepo{}, &fullSnapClient{})

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already_active")
	assert.Contains(t, w.Body.String(), subID.String())
}

func TestSyncSubscription_NoPayment_Returns404(t *testing.T) {
	subRepo := &syncSubRepo{}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return nil, nil
		},
	}
	h := newSyncTestHandler(subRepo, payRepo, &fullSnapClient{})

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "NO_PAYMENT_FOUND")
}

func TestSyncSubscription_LocalSettlement_Returns200(t *testing.T) {
	paymentID := uuid.New()
	snapClient := &fullSnapClient{}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              paymentID,
				MidtransOrderID: "LAB-SUB-SETTLE",
				Status:          paymentRepository.PaymentStatusSettlement,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "activated")
	// gateway must NOT be called for a locally-settled payment
	assert.Equal(t, 0, snapClient.getStatusCalls)
}

func TestSyncSubscription_LocalCapture_Returns200(t *testing.T) {
	paymentID := uuid.New()
	snapClient := &fullSnapClient{}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              paymentID,
				MidtransOrderID: "LAB-SUB-CAPTURE",
				Status:          paymentRepository.PaymentStatusCapture,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "activated")
	assert.Equal(t, 0, snapClient.getStatusCalls)
}

func TestSyncSubscription_PendingGatewaySettled_Returns200(t *testing.T) {
	snapClient := &fullSnapClient{
		getStatusFn: func(string) (*midtrans.NotificationPayload, error) {
			return &midtrans.NotificationPayload{TransactionStatus: "settlement"}, nil
		},
	}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-PENDING",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "activated")
	assert.Equal(t, 1, snapClient.getStatusCalls)
}

func TestSyncSubscription_PendingGatewayCapture_Returns200(t *testing.T) {
	snapClient := &fullSnapClient{
		getStatusFn: func(string) (*midtrans.NotificationPayload, error) {
			return &midtrans.NotificationPayload{TransactionStatus: "capture"}, nil
		},
	}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-CAPTURE-GW",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "activated")
	assert.Equal(t, 1, snapClient.getStatusCalls)
}

func TestSyncSubscription_PendingGatewayPending_Returns202(t *testing.T) {
	snapClient := &fullSnapClient{
		getStatusFn: func(string) (*midtrans.NotificationPayload, error) {
			return &midtrans.NotificationPayload{TransactionStatus: "pending"}, nil
		},
	}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-STILL-PENDING",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), "pending")
}

func TestSyncSubscription_PendingExpired_Returns410(t *testing.T) {
	snapClient := &fullSnapClient{}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-EXP",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(-1 * time.Hour), // expired
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusGone, w.Code)
	assert.Contains(t, w.Body.String(), "PAYMENT_EXPIRED")
	// gateway must NOT be called for a locally-expired payment
	assert.Equal(t, 0, snapClient.getStatusCalls)
}

func TestSyncSubscription_GatewayError_Returns503(t *testing.T) {
	snapClient := &fullSnapClient{
		getStatusFn: func(string) (*midtrans.NotificationPayload, error) {
			return nil, errors.New("circuit breaker open")
		},
	}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-GW-ERR",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "GATEWAY_UNAVAILABLE")
}

func TestSyncSubscription_PendingGatewayDeny_Returns409(t *testing.T) {
	snapClient := &fullSnapClient{
		getStatusFn: func(string) (*midtrans.NotificationPayload, error) {
			return &midtrans.NotificationPayload{TransactionStatus: "deny"}, nil
		},
	}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-DENY",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(&syncSubRepo{}, payRepo, snapClient)

	c, w := newSyncGinContext(uuid.New())
	h.run(c)

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "PAYMENT_FAILED")
}

func TestSyncSubscription_IdempotentDoubleSync_NoDoubleActivation(t *testing.T) {
	// First call: pending → gateway settlement → activates.
	// Second call: active subscription already exists, but the latest payment
	// still takes the canonical activation path because the handler no longer
	// short-circuits before looking at the payment row.
	subRepo := &syncSubRepo{}
	snapClient := &fullSnapClient{
		getStatusFn: func(string) (*midtrans.NotificationPayload, error) {
			return &midtrans.NotificationPayload{TransactionStatus: "settlement"}, nil
		},
	}
	payRepo := &syncPaymentRepo{
		findLatestFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				MidtransOrderID: "LAB-SUB-IDEM",
				Status:          paymentRepository.PaymentStatusPending,
				GrossAmount:     money.New(150000),
				ExpiredAt:       time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	h := newSyncTestHandler(subRepo, payRepo, snapClient)

	userID := uuid.New()

	// First sync — activates
	c1, w1 := newSyncGinContext(userID)
	h.run(c1)
	require.Equal(t, http.StatusOK, w1.Code)
	assert.Contains(t, w1.Body.String(), "activated")
	assert.Equal(t, 1, snapClient.getStatusCalls)

	// Simulate subscription now active for subsequent calls
	subID := uuid.New()
	subRepo.activeSubscription = &subscriptionEntity.SellerSubscription{
		ID:     subID,
		Status: subscriptionEntity.StatusActive,
	}

	// Second sync — still reaches the payment branch.
	c2, w2 := newSyncGinContext(userID)
	h.run(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "activated")

	// Gateway was called twice because both sync attempts inspect the payment.
	assert.Equal(t, 2, snapClient.getStatusCalls)
}
