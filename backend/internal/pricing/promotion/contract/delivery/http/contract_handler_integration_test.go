//go:build integration

// PHASE 4A — CANONICAL CONTRACT HTTP SURFACE (REAL DB)
//
// Proves the HTTP layer of the canonical contract surface end to end:
//   - POST   /promotions/contracts           (create, 201; ineligible seller 403)
//   - GET    /promotions/contracts           (owner list only)
//   - GET    /promotions/contracts/:id       (owner detail; cross-seller 403)
//   - POST   /promotions/contracts/:id/pause / resume / finalize
//     (owner OK; cross-seller 403 CONTRACT_NOT_OWNED; double pause 409)
//
// The route middleware (RequireActiveAccount / RequireSellerMiddleware) is NOT
// part of this test — it is exercised separately by the shared route tests.
// Here we prove the handler + canonical service + real eligibility gate.
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	financeapp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/identity/auth"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	configrepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	contractapp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	contracthttp "github.com/labuda/backend/internal/pricing/promotion/contract/delivery/http"
	deliveryRepoImpl "github.com/labuda/backend/internal/pricing/promotion/delivery/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

type httpEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		Message   string `json:"message"`
		Contract  *struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"contract"`
		Contracts []struct {
			ID string `json:"id"`
		} `json:"contracts"`
		Count int `json:"count"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type phase4AHTTPHarness struct {
	engine *gin.Engine
	tdb    *testdb.TestDB
}

func newPhase4AHTTPHarness(t *testing.T) *phase4AHTTPHarness {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	_, err := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool())).EnsureSystemAccounts(ctx)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES
			('promotion_cpm', 7500, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_min_daily_budget', 10000, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_delivery_enabled', NULL, 'disabled', NULL, EXTRACT(epoch FROM now())::bigint)
		ON CONFLICT (key) DO UPDATE
			SET value_numeric = EXCLUDED.value_numeric,
			    value_text = EXCLUDED.value_text,
			    updated_at = EXTRACT(epoch FROM now())::bigint
	`)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	cfgSvc := configapp.NewConfigService(configrepo.NewPlatformConfigRepository())
	deliveryRepo := deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool()))
	gate := contractapp.NewRoleCheckerSellerEligibilityGate(auth.NewRoleCheckerDB(db.NewFromPool(tdb.Pool()), nil))
	contractSvc := contractapp.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		gate,
		deliveryRepo,
	)
	handler := contracthttp.NewContractHandler(contractSvc, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Acting-user middleware: mirrors UserLookupMiddleware's "user_id" context
	// key; the actor is selected per request via the X-Actor-ID header.
	r.Use(func(c *gin.Context) {
		raw := c.GetHeader("X-Actor-ID")
		if raw != "" {
			if id, err := uuid.Parse(raw); err == nil {
				c.Set("user_id", id)
			}
		}
		c.Next()
	})
	group := r.Group("/api/v1/promotions/contracts")
	{
		group.POST("", handler.CreateContract)
		group.GET("", handler.ListContracts)
		group.GET("/:id", handler.GetContract)
		group.POST("/:id/pause", handler.PauseContract)
		group.POST("/:id/resume", handler.ResumeContract)
		group.POST("/:id/finalize", handler.FinalizeContract)
	}
	return &phase4AHTTPHarness{engine: r, tdb: tdb}
}

func (h *phase4AHTTPHarness) seedSeller(t *testing.T, eligible bool, fundRupiah int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	seller := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, email_verified_at, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), NOW(), 'user')
	`, seller, "fb-p4ah-"+seller.String()[:8], seller.String()+"@p4ah.local")
	require.NoError(t, err)
	if eligible {
		_, err = h.tdb.Pool().Exec(ctx, `
			INSERT INTO seller_profiles (id, user_id, store_name, tier, status, created_at, updated_at)
			VALUES ($1, $2, 'P4AH Store', 'basic', 'active', NOW(), NOW())
		`, uuid.New(), seller)
		require.NoError(t, err)
		now := time.Now().UTC()
		_, err = h.tdb.Pool().Exec(ctx, `
			INSERT INTO seller_subscriptions (
				id, user_id, status, started_at, expires_at,
				duration_days, amount_paid, currency, payment_id, created_at, updated_at
			)
			VALUES ($1, $2, 'active', $3, $4, 365, 0, 'IDR', $5, NOW(), NOW())
		`, uuid.New(), seller, now.Add(-24*time.Hour), now.Add(24*time.Hour), uuid.New())
		require.NoError(t, err)
	}
	if fundRupiah > 0 {
		financeSvc := financeapp.NewFinanceService()
		require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
			return financeSvc.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), seller, fundRupiah)
		}))
	}
	return seller
}

func (h *phase4AHTTPHarness) do(t *testing.T, method, path string, actor uuid.UUID, body string) (int, httpEnvelope) {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if actor != uuid.Nil {
		req.Header.Set("X-Actor-ID", actor.String())
	}
	w := httptest.NewRecorder()
	h.engine.ServeHTTP(w, req)
	var env httpEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env
}

func TestPhase4A_ContractHTTP_Lifecycle_RealDB(t *testing.T) {
	h := newPhase4AHTTPHarness(t)

	sellerA := h.seedSeller(t, true, 100_000)
	sellerB := h.seedSeller(t, true, 100_000)

	// Create (owner A): 201 + active contract.
	code, env := h.do(t, http.MethodPost, "/api/v1/promotions/contracts", sellerA,
		`{"kind":"internal","budget_rupiah":50000,"duration_days":3}`)
	require.Equal(t, http.StatusCreated, code, "create should be 201: %+v", env.Error)
	require.NotNil(t, env.Data.Contract)
	require.Equal(t, "active", env.Data.Contract.Status)
	contractID := env.Data.Contract.ID

	// Invalid kind -> 400.
	code, env = h.do(t, http.MethodPost, "/api/v1/promotions/contracts", sellerA,
		`{"kind":"bogus","budget_rupiah":50000,"duration_days":3}`)
	require.Equal(t, http.StatusBadRequest, code)

	// Ineligible seller (real gate) -> 403.
	ghost := h.seedSeller(t, false, 0)
	code, env = h.do(t, http.MethodPost, "/api/v1/promotions/contracts", ghost,
		`{"kind":"internal","budget_rupiah":50000,"duration_days":3}`)
	require.Equal(t, http.StatusForbidden, code, "ineligible seller must be rejected")

	// Owner list / detail.
	code, env = h.do(t, http.MethodGet, "/api/v1/promotions/contracts", sellerA, "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 1, env.Data.Count)

	code, env = h.do(t, http.MethodGet, "/api/v1/promotions/contracts/"+contractID, sellerA, "")
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, env.Data.Contract)

	// Cross-seller reads are denied.
	code, env = h.do(t, http.MethodGet, "/api/v1/promotions/contracts/"+contractID, sellerB, "")
	require.Equal(t, http.StatusForbidden, code)
	require.NotNil(t, env.Error)
	require.Equal(t, "CONTRACT_NOT_OWNED", env.Error.Code)

	// Cross-seller lifecycle mutations are denied.
	for _, action := range []string{"pause", "resume", "finalize"} {
		code, env = h.do(t, http.MethodPost, "/api/v1/promotions/contracts/"+contractID+"/"+action, sellerB, "")
		require.Equal(t, http.StatusForbidden, code, "%s by non-owner must be forbidden", action)
		require.Equal(t, "CONTRACT_NOT_OWNED", env.Error.Code)
	}

	// Owner lifecycle: pause -> double pause 409 -> resume -> finalize.
	code, _ = h.do(t, http.MethodPost, "/api/v1/promotions/contracts/"+contractID+"/pause", sellerA, "")
	require.Equal(t, http.StatusOK, code)
	code, env = h.do(t, http.MethodPost, "/api/v1/promotions/contracts/"+contractID+"/pause", sellerA, "")
	require.Equal(t, http.StatusConflict, code)
	require.Equal(t, "PROMOTION_ALREADY_PAUSED", env.Error.Code)

	code, _ = h.do(t, http.MethodPost, "/api/v1/promotions/contracts/"+contractID+"/resume", sellerA, "")
	require.Equal(t, http.StatusOK, code)
	code, _ = h.do(t, http.MethodPost, "/api/v1/promotions/contracts/"+contractID+"/finalize", sellerA, "")
	require.Equal(t, http.StatusOK, code)

	code, env = h.do(t, http.MethodGet, "/api/v1/promotions/contracts/"+contractID, sellerA, "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "finalized", env.Data.Contract.Status)
}
