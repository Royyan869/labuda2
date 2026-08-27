package http

import (
	"bufio"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	subscriptionApp "github.com/labuda/backend/internal/commerce/subscription/application"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	paymentRepository "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTx struct{}

func (t *testTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *testTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (t *testTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *testTx) Commit(context.Context) error   { return nil }
func (t *testTx) Rollback(context.Context) error { return nil }

type testSubscriptionPaymentRepo struct {
	findPendingFn func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error)
	createFn      func(context.Context, db.Tx, paymentRepository.CreatePaymentInput) (*paymentRepository.Payment, error)
	updateFn      func(context.Context, db.Tx, uuid.UUID, string) error

	findCalls   int
	createCalls int
	updateCalls int
}

func (r *testSubscriptionPaymentRepo) FindPendingSubscriptionPayment(ctx context.Context, tx db.Tx, userID uuid.UUID) (*paymentRepository.Payment, error) {
	r.findCalls++
	if r.findPendingFn != nil {
		return r.findPendingFn(ctx, tx, userID)
	}
	return nil, nil
}

func (r *testSubscriptionPaymentRepo) CreatePayment(ctx context.Context, tx db.Tx, input paymentRepository.CreatePaymentInput) (*paymentRepository.Payment, error) {
	r.createCalls++
	if r.createFn != nil {
		return r.createFn(ctx, tx, input)
	}
	return &paymentRepository.Payment{
		ID:              uuid.New(),
		UserID:          input.UserID,
		PaymentNumber:   input.PaymentNumber,
		MidtransOrderID: input.MidtransOrderID,
		GrossAmount:     input.GrossAmount,
		Status:          paymentRepository.PaymentStatusPending,
		ReferenceType:   input.ReferenceType,
		ReferenceID:     input.ReferenceID,
		ExpiredAt:       input.ExpiredAt,
	}, nil
}

func (r *testSubscriptionPaymentRepo) FindLatestSubscriptionPayment(ctx context.Context, tx db.Tx, userID uuid.UUID) (*paymentRepository.Payment, error) {
	return nil, nil
}

func (r *testSubscriptionPaymentRepo) UpdatePaymentURL(ctx context.Context, tx db.Tx, paymentID uuid.UUID, paymentURL string) error {
	r.updateCalls++
	if r.updateFn != nil {
		return r.updateFn(ctx, tx, paymentID, paymentURL)
	}
	return nil
}

type testSnapClient struct {
	calls int
	req   *midtrans.SnapRequest
	resp  *midtrans.SnapResponse
	err   error
}

func (c *testSnapClient) CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error) {
	c.calls++
	c.req = req
	if c.err != nil {
		return nil, c.err
	}
	if c.resp != nil {
		return c.resp, nil
	}
	return &midtrans.SnapResponse{RedirectURL: "https://midtrans.example/redirect"}, nil
}

func (c *testSnapClient) GetTransactionStatus(orderID string) (*midtrans.NotificationPayload, error) {
	return nil, nil
}

type testSubscriptionRepo struct {
	config *subscriptionEntity.SellerSubscriptionConfig
	latest *subscriptionEntity.SellerSubscription
}

func (r *testSubscriptionRepo) InsertTx(context.Context, db.Tx, *subscriptionEntity.SellerSubscription) error {
	return nil
}

func (r *testSubscriptionRepo) UpdateStatusTx(context.Context, db.Tx, uuid.UUID, subscriptionEntity.Status, subscriptionEntity.Status) error {
	return nil
}

func (r *testSubscriptionRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) GetLatestByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) GetLatestByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.latest, nil
}

func (r *testSubscriptionRepo) GetActiveByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) FetchActiveExpiredBatch(context.Context, db.Tx, time.Time, int) ([]*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) FetchActiveExpiredBatchIDs(context.Context, db.Tx, time.Time, int) ([]uuid.UUID, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) ExistsActiveByUserID(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *testSubscriptionRepo) GetActiveConfig(context.Context, db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error) {
	return r.config, nil
}

func (r *testSubscriptionRepo) UpdateConfigTx(context.Context, db.Tx, uuid.UUID, int64, int, int, bool) error {
	return nil
}

type testOnboardingUserRepo struct {
	user    *userEntity.User
	profile *userEntity.UserProfile
}

func (r *testOnboardingUserRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*userEntity.User, error) {
	return r.user, nil
}

func (r *testOnboardingUserRepo) GetProfileByID(context.Context, db.Tx, uuid.UUID) (*userEntity.UserProfile, error) {
	return r.profile, nil
}

type testOnboardingSellerRepo struct {
	profile *sellerEntity.SellerProfile
}

func (r *testOnboardingSellerRepo) GetByUserID(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return r.profile, nil
}

type testOnboardingAddressRepo struct {
	addresses []*addressEntity.Address
}

func (r *testOnboardingAddressRepo) GetByUserIDFiltered(context.Context, db.Tx, uuid.UUID, string) ([]*addressEntity.Address, error) {
	return r.addresses, nil
}

func newTestSubscriptionInitiateHandler(t *testing.T, paymentRepo subscriptionPaymentRepository, snapClient snapTransactionClient, subRepo subscriptionRepo.SellerSubscriptionRepository) *SellerHandler {
	t.Helper()

	userID := uuid.New()
	username := "seller-user"
	bio := "bio"
	phone := "+628123456789"

	onboardingService := subscriptionApp.NewSellerOnboardingService(
		&testOnboardingUserRepo{
			user: &userEntity.User{
				ID:            userID,
				PhoneNumber:   &phone,
				EmailVerified: true,
			},
			profile: &userEntity.UserProfile{
				UserID:   userID,
				Username: &username,
				Bio:      &bio,
			},
		},
		&testOnboardingSellerRepo{
			profile: &sellerEntity.SellerProfile{
				ID:        uuid.New(),
				UserID:    userID,
				StoreName: "Hukoi",
				Tier:      sellerEntity.TierBasic,
			},
		},
		&testOnboardingAddressRepo{
			addresses: []*addressEntity.Address{
				{
					ID:            uuid.New(),
					UserID:        userID,
					Purpose:       addressEntity.AddressPurposeSender,
					RecipientName: "Royyan",
					Phone:         phone,
				},
			},
		},
	)

	return &SellerHandler{
		onboardingService: onboardingService,
		paymentRepo:       paymentRepo,
		midtransClient:    snapClient,
		subRepo:           subRepo,
		frontendURL:       "https://frontend.example",
	}
}

func newTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/seller/subscription/initiate", nil)
	return c
}

func TestInitiateSubscriptionPaymentTx_ReturnsLookupError(t *testing.T) {
	userID := uuid.New()
	expectedErr := errors.New("deadlock inside pending lookup")
	paymentRepo := &testSubscriptionPaymentRepo{
		findPendingFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return nil, expectedErr
		},
	}
	snapClient := &testSnapClient{}
	subRepo := &testSubscriptionRepo{
		config: &subscriptionEntity.SellerSubscriptionConfig{
			ID:                  uuid.New(),
			YearlyFeeRupiah:     150000,
			DurationDays:        365,
			RenewalReminderDays: 7,
			Enabled:             true,
		},
	}
	handler := newTestSubscriptionInitiateHandler(t, paymentRepo, snapClient, subRepo)

	resp, err := handler.initiateSubscriptionPaymentTx(newTestGinContext(), context.Background(), &testTx{}, userID)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "find pending subscription payment")
	assert.ErrorIs(t, err, expectedErr)
	assert.Zero(t, paymentRepo.createCalls)
	assert.Zero(t, paymentRepo.updateCalls)
	assert.Zero(t, snapClient.calls)
}

func TestInitiateSubscriptionPaymentTx_ReturnsExistingPendingPaymentWhenURLExists(t *testing.T) {
	userID := uuid.New()
	paymentURL := "https://midtrans.example/existing"
	existingPayment := &paymentRepository.Payment{
		ID:              uuid.New(),
		UserID:          userID,
		PaymentNumber:   "PAY-SUB-1",
		MidtransOrderID: "LAB-SUB-1",
		GrossAmount:     money.New(150000),
		Status:          paymentRepository.PaymentStatusPending,
		ReferenceType:   paymentRepository.ReferenceTypeSubscription,
		ExpiredAt:       time.Now().Add(24 * time.Hour),
	}
	existingPayment.PaymentURL = &paymentURL

	paymentRepo := &testSubscriptionPaymentRepo{
		findPendingFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return existingPayment, nil
		},
	}
	snapClient := &testSnapClient{}
	subRepo := &testSubscriptionRepo{
		config: &subscriptionEntity.SellerSubscriptionConfig{
			ID:                  uuid.New(),
			YearlyFeeRupiah:     150000,
			DurationDays:        365,
			RenewalReminderDays: 7,
			Enabled:             true,
		},
	}
	handler := newTestSubscriptionInitiateHandler(t, paymentRepo, snapClient, subRepo)

	resp, err := handler.initiateSubscriptionPaymentTx(newTestGinContext(), context.Background(), &testTx{}, userID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, existingPayment.ID, resp.PaymentID)
	assert.Equal(t, paymentURL, resp.PaymentURL)
	assert.Equal(t, existingPayment.GrossAmount.Int64(), resp.GrossAmount)
	assert.Equal(t, existingPayment.ExpiredAt.Format(time.RFC3339), resp.ExpiredAt)
	assert.Zero(t, paymentRepo.createCalls)
	assert.Zero(t, paymentRepo.updateCalls)
	assert.Zero(t, snapClient.calls)
}

func TestInitiateSubscriptionPaymentTx_ReusesExistingPendingPaymentWithoutURL(t *testing.T) {
	userID := uuid.New()
	existingPayment := &paymentRepository.Payment{
		ID:              uuid.New(),
		UserID:          userID,
		PaymentNumber:   "PAY-SUB-2",
		MidtransOrderID: "LAB-SUB-2",
		GrossAmount:     money.New(150000),
		Status:          paymentRepository.PaymentStatusPending,
		ReferenceType:   paymentRepository.ReferenceTypeSubscription,
		ExpiredAt:       time.Now().Add(24 * time.Hour),
	}

	paymentRepo := &testSubscriptionPaymentRepo{
		findPendingFn: func(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
			return existingPayment, nil
		},
	}
	snapClient := &testSnapClient{
		resp: &midtrans.SnapResponse{
			RedirectURL: "https://midtrans.example/reused",
		},
	}
	subRepo := &testSubscriptionRepo{
		config: &subscriptionEntity.SellerSubscriptionConfig{
			ID:                  uuid.New(),
			YearlyFeeRupiah:     150000,
			DurationDays:        365,
			RenewalReminderDays: 7,
			Enabled:             true,
		},
	}
	handler := newTestSubscriptionInitiateHandler(t, paymentRepo, snapClient, subRepo)

	resp, err := handler.initiateSubscriptionPaymentTx(newTestGinContext(), context.Background(), &testTx{}, userID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, existingPayment.ID, resp.PaymentID)
	assert.Equal(t, "https://midtrans.example/reused", resp.PaymentURL)
	assert.Equal(t, existingPayment.GrossAmount.Int64(), resp.GrossAmount)
	assert.Equal(t, existingPayment.ExpiredAt.Format(time.RFC3339), resp.ExpiredAt)
	assert.Zero(t, paymentRepo.createCalls)
	assert.Equal(t, 1, paymentRepo.updateCalls)
	assert.Equal(t, 1, snapClient.calls)
	require.NotNil(t, snapClient.req)
	assert.Equal(t, existingPayment.MidtransOrderID, snapClient.req.TransactionDetails.OrderID)
	// PASS_18N: yearly_fee_rupiah is a Rupiah integer — the Snap gross amount must
	// equal GrossAmount exactly, with NO /100 scaling in either direction.
	assert.InDelta(t, float64(existingPayment.GrossAmount.Int64()), snapClient.req.TransactionDetails.GrossAmount, 0.0001)
}

// TestInitiateSubscriptionPaymentTx_CreatesNewPaymentWithFullRupiahGrossAmount
// locks the PASS_18N fix: when no pending payment exists, the newly created
// payment row's GrossAmount and the Midtrans Snap charge must both equal the
// full Rupiah config.YearlyFeeRupiah — no /100 division anywhere in this path.
// Before the fix, the Snap charge was config.YearlyFeeRupiah/100 (undercharging
// the buyer 100x) while the stored GrossAmount kept the full value, so the
// webhook's amount-validation step would always fail and the subscription
// would never activate.
func TestInitiateSubscriptionPaymentTx_CreatesNewPaymentWithFullRupiahGrossAmount(t *testing.T) {
	userID := uuid.New()
	const yearlyFee = int64(150000) // Rp 150,000

	var capturedInput paymentRepository.CreatePaymentInput
	paymentRepo := &testSubscriptionPaymentRepo{
		createFn: func(_ context.Context, _ db.Tx, input paymentRepository.CreatePaymentInput) (*paymentRepository.Payment, error) {
			capturedInput = input
			return &paymentRepository.Payment{
				ID:              uuid.New(),
				UserID:          input.UserID,
				PaymentNumber:   input.PaymentNumber,
				MidtransOrderID: input.MidtransOrderID,
				GrossAmount:     input.GrossAmount,
				Status:          paymentRepository.PaymentStatusPending,
				ReferenceType:   input.ReferenceType,
				ReferenceID:     input.ReferenceID,
				ExpiredAt:       input.ExpiredAt,
			}, nil
		},
	}
	snapClient := &testSnapClient{}
	subRepo := &testSubscriptionRepo{
		config: &subscriptionEntity.SellerSubscriptionConfig{
			ID:                  uuid.New(),
			YearlyFeeRupiah:     yearlyFee,
			DurationDays:        365,
			RenewalReminderDays: 7,
			Enabled:             true,
		},
	}
	handler := newTestSubscriptionInitiateHandler(t, paymentRepo, snapClient, subRepo)

	resp, err := handler.initiateSubscriptionPaymentTx(newTestGinContext(), context.Background(), &testTx{}, userID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, paymentRepo.createCalls)
	require.NotNil(t, snapClient.req)

	// The stored payment.GrossAmount must be the full Rupiah yearly fee.
	assert.Equal(t, yearlyFee, capturedInput.GrossAmount.Int64())
	assert.Equal(t, yearlyFee, resp.GrossAmount)

	// The Snap charge sent to Midtrans must equal the SAME Rupiah value —
	// this is the exact invariant the webhook's amount validation depends on.
	assert.InDelta(t, float64(yearlyFee), snapClient.req.TransactionDetails.GrossAmount, 0.0001)
	assert.InDelta(t, float64(yearlyFee), snapClient.req.ItemDetails[0].Price, 0.0001)
}

// TestSellerHandler_SubscriptionAmountsNoCentsDivision is a structural
// regression test for the PASS_18N fix. yearly_fee_rupiah is a Rupiah integer;
// reintroducing a "/ 100" or "/100" division inside initiateSubscriptionPaymentTx
// would silently undercharge every seller subscription 100x at Midtrans
// while the stored payment.GrossAmount kept the full value, causing the
// webhook's amount validation to fail and the subscription to never
// activate. Asserted at the source level, matching the convention used by
// TestWebhookAmountValidationNoCentsDivision in payment_webhook_test.go.
//
// Scoped to initiateSubscriptionPaymentTx only — this file also contains
// unrelated "/100" float-rounding for percentage stats (ConversionRate,
// CancelRate) which must NOT trip this check.
func TestSellerHandler_SubscriptionAmountsNoCentsDivision(t *testing.T) {
	f, err := os.Open("seller_handler.go")
	if err != nil {
		t.Fatalf("failed to open seller_handler.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFunc := false
	foundFunc := false
	braceDepth := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inFunc {
			if strings.Contains(line, "func (h *SellerHandler) initiateSubscriptionPaymentTx") {
				inFunc = true
				foundFunc = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		if strings.Contains(line, "/ 100") || strings.Contains(line, "/100") {
			t.Fatalf("seller_handler.go:%d inside initiateSubscriptionPaymentTx must not divide amounts by 100 (yearly_fee_rupiah is a Rupiah integer): %q", lineNum, line)
		}

		if braceDepth <= 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan seller_handler.go: %v", err)
	}
	if !foundFunc {
		t.Fatal("initiateSubscriptionPaymentTx function not found — subscription payment logic may have moved")
	}
}
