// DOMAIN: PAYMENT METHOD — ADMIN CONFIG (PASS_18W)
//
// Admin governance for the canonical payment_methods table introduced in
// PASS_18V. This handler is NOT money authority by itself — it only ever
// writes to payment_methods. The actual buyer payment fee/gross amount is
// still computed exclusively by CorePaymentHandler.CreatePayment at
// payment-creation time, reading this table fresh. An edit here changes
// nothing about any order or payment that already exists.
package http

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	"github.com/labuda/backend/internal/commerce/paymentmethod/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// AdminPaymentMethodHandler handles admin CRUD + preview for canonical
// payment methods.
//
// GET  /admin/payment-methods          — requires finance.payment_method.view
// GET  /admin/payment-methods/:code    — requires finance.payment_method.view
// PUT  /admin/payment-methods/:code    — requires finance.payment_method.manage
// POST /admin/payment-methods/:code/preview — requires finance.payment_method.view
type AdminPaymentMethodHandler struct {
	repo             *repository.PaymentMethodRepository
	db               *db.DB
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
}

// NewAdminPaymentMethodHandler constructs the handler.
func NewAdminPaymentMethodHandler(
	repo *repository.PaymentMethodRepository,
	database *db.DB,
	log *zap.Logger,
) *AdminPaymentMethodHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminPaymentMethodHandler{
		repo:             repo,
		db:               database,
		log:              log,
		adminAuditLogger: audit.NewAdminAuditLoggerDB(database.Pool()),
	}
}

// ============================================================================
// WIRE DTOs
// ============================================================================

// methodResponse is the wire shape for a single method, shared by list/get/update.
type methodResponse struct {
	MethodCode       string   `json:"method_code"`
	DisplayName      string   `json:"display_name"`
	Enabled          bool     `json:"enabled"`
	FeeType          string   `json:"fee_type"`
	FlatAmountRupiah int64    `json:"flat_amount_rupiah"`
	PercentBps       int64    `json:"percent_bps"`
	MinFeeRupiah     *int64   `json:"min_fee_rupiah,omitempty"`
	MaxFeeRupiah     *int64   `json:"max_fee_rupiah,omitempty"`
	MidtransChannels []string `json:"midtrans_channels"`
	SortOrder        int      `json:"sort_order"`

	// PASS_19A rate-source verification metadata.
	RateSource         string     `json:"rate_source"`
	RateSourceNote     string     `json:"rate_source_note,omitempty"`
	MerchantVerifiedAt *time.Time `json:"merchant_verified_at,omitempty"`
}

func toMethodResponse(m entity.Method) methodResponse {
	resp := methodResponse{
		MethodCode:         m.Code,
		DisplayName:        m.DisplayName,
		Enabled:            m.Enabled,
		FeeType:            string(m.FeeType),
		FlatAmountRupiah:   m.FlatAmount.Int64(),
		PercentBps:         m.PercentBps,
		MidtransChannels:   m.MidtransChannels,
		SortOrder:          m.SortOrder,
		RateSource:         string(m.RateSource),
		RateSourceNote:     m.RateSourceNote,
		MerchantVerifiedAt: m.MerchantVerifiedAt,
	}
	if m.MinFee != nil {
		v := m.MinFee.Int64()
		resp.MinFeeRupiah = &v
	}
	if m.MaxFee != nil {
		v := m.MaxFee.Int64()
		resp.MaxFeeRupiah = &v
	}
	return resp
}

// updateMethodRequest is the PUT request body. method_code is deliberately
// absent — it comes from the URL and is immutable (PASS_18V canonical
// method-code doctrine).
type updateMethodRequest struct {
	DisplayName      string   `json:"display_name" binding:"required"`
	Enabled          bool     `json:"enabled"`
	FeeType          string   `json:"fee_type" binding:"required"`
	FlatAmountRupiah int64    `json:"flat_amount_rupiah"`
	PercentBps       int64    `json:"percent_bps"`
	MinFeeRupiah     *int64   `json:"min_fee_rupiah"`
	MaxFeeRupiah     *int64   `json:"max_fee_rupiah"`
	MidtransChannels []string `json:"midtrans_channels"`
	SortOrder        int      `json:"sort_order"`

	// RateSource is required: an admin must always state whether they
	// believe the config they are saving is still the unverified public
	// baseline, an owner-confirmed merchant rate, or a manual override
	// (PASS_19A) — it is never inferred silently on write.
	RateSource     string `json:"rate_source" binding:"required"`
	RateSourceNote string `json:"rate_source_note"`
}

func (r updateMethodRequest) toEntity(code string) entity.Method {
	m := entity.Method{
		Code:             code,
		DisplayName:      r.DisplayName,
		Enabled:          r.Enabled,
		FeeType:          entity.FeeType(r.FeeType),
		FlatAmount:       money.New(r.FlatAmountRupiah),
		PercentBps:       r.PercentBps,
		MidtransChannels: r.MidtransChannels,
		SortOrder:        r.SortOrder,
		RateSource:       entity.RateSource(r.RateSource),
		RateSourceNote:   r.RateSourceNote,
	}
	if r.MinFeeRupiah != nil {
		v := money.New(*r.MinFeeRupiah)
		m.MinFee = &v
	}
	if r.MaxFeeRupiah != nil {
		v := money.New(*r.MaxFeeRupiah)
		m.MaxFee = &v
	}
	return m
}

// previewRequest is the POST .../preview request body — a hypothetical fee
// config plus a sample base amount. Pure computation; never touches the DB
// or the saved row, so an admin can preview edits before saving them.
type previewRequest struct {
	FeeType          string `json:"fee_type" binding:"required"`
	FlatAmountRupiah int64  `json:"flat_amount_rupiah"`
	PercentBps       int64  `json:"percent_bps"`
	MinFeeRupiah     *int64 `json:"min_fee_rupiah"`
	MaxFeeRupiah     *int64 `json:"max_fee_rupiah"`
	BaseAmountRupiah int64  `json:"base_amount_rupiah" binding:"required,min=1"`
}

// ============================================================================
// GET /admin/payment-methods
// ============================================================================

// ListMethods returns every method (enabled and disabled) with full config.
func (h *AdminPaymentMethodHandler) ListMethods(c *gin.Context) {
	ctx := c.Request.Context()

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapFinancePaymentMethodView.String()) {
		response.Forbidden(c, "finance.payment_method.view capability required")
		return
	}

	var methods []entity.Method
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		methods, err = h.repo.ListAll(ctx, tx)
		return err
	})
	if err != nil {
		h.log.Error("Failed to list payment methods", zap.Error(err))
		response.InternalServerError(c, "Failed to list payment methods")
		return
	}

	out := make([]methodResponse, 0, len(methods))
	for _, m := range methods {
		out = append(out, toMethodResponse(m))
	}

	response.Success(c, gin.H{"methods": out, "count": len(out)})
}

// ============================================================================
// GET /admin/payment-methods/:code
// ============================================================================

// GetMethod returns a single method's full config.
func (h *AdminPaymentMethodHandler) GetMethod(c *gin.Context) {
	ctx := c.Request.Context()

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapFinancePaymentMethodView.String()) {
		response.Forbidden(c, "finance.payment_method.view capability required")
		return
	}

	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "method code is required")
		return
	}

	var m *entity.Method
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		m, err = h.repo.GetByCode(ctx, tx, code)
		return err
	})
	if err != nil {
		if err == repository.ErrMethodNotFound {
			response.NotFound(c, "Payment method not found")
			return
		}
		h.log.Error("Failed to get payment method", zap.String("method_code", code), zap.Error(err))
		response.InternalServerError(c, "Failed to get payment method")
		return
	}

	response.Success(c, gin.H{"method": toMethodResponse(*m)})
}

// ============================================================================
// PUT /admin/payment-methods/:code
// ============================================================================

// UpdateMethod validates and persists an admin edit to a method's config.
//
// SAFETY: this only ever writes to the payment_methods row. It never
// touches orders/payments — see the package doc comment.
func (h *AdminPaymentMethodHandler) UpdateMethod(c *gin.Context) {
	ctx := c.Request.Context()

	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "method code is required")
		return
	}

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapFinancePaymentMethodManage.String()) {
		response.Forbidden(c, "finance.payment_method.manage capability required")
		return
	}

	var req updateMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	candidate := req.toEntity(code)
	if err := entity.ValidateConfig(candidate); err != nil {
		response.BadRequest(c, "Invalid payment method config: "+err.Error())
		return
	}

	var (
		before   *entity.Method
		after    *entity.Method
		notFound bool
		conflict string
	)

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		before, err = h.repo.GetByCode(ctx, tx, code)
		if err != nil {
			if err == repository.ErrMethodNotFound {
				notFound = true
				return nil
			}
			return err
		}

		// GUARD: disabling every method would make checkout impossible.
		// Reject rather than silently degrading — no override flag in this
		// pass (see PASS_18W remaining debt).
		if !req.Enabled {
			enabledElsewhere, err := h.repo.CountEnabledExcluding(ctx, tx, code)
			if err != nil {
				return err
			}
			if enabledElsewhere == 0 {
				conflict = "cannot disable the last enabled payment method; checkout would have no available method"
				return nil
			}
		}

		// PASS_19A: a fee-formula edit can never silently stay labeled
		// public_baseline, and merchant_verified_at is server-derived, never
		// client-supplied — see entity.ReconcileRateSource /
		// entity.ResolveMerchantVerifiedAt doc comments.
		reconciled := entity.ReconcileRateSource(candidate, *before)
		merchantVerifiedAt := entity.ResolveMerchantVerifiedAt(reconciled, *before, time.Now())

		after, err = h.repo.Update(ctx, tx, code, repository.UpdateMethodInput{
			DisplayName:        req.DisplayName,
			Enabled:            req.Enabled,
			FeeType:            entity.FeeType(req.FeeType),
			FlatAmount:         money.New(req.FlatAmountRupiah),
			PercentBps:         req.PercentBps,
			MinFee:             candidate.MinFee,
			MaxFee:             candidate.MaxFee,
			MidtransChannels:   req.MidtransChannels,
			SortOrder:          req.SortOrder,
			RateSource:         reconciled.RateSource,
			RateSourceNote:     reconciled.RateSourceNote,
			MerchantVerifiedAt: merchantVerifiedAt,
		})
		return err
	})

	if err != nil {
		h.log.Error("Failed to update payment method",
			zap.String("method_code", code),
			zap.String("actor_id", actor.ID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to update payment method")
		return
	}
	if notFound {
		response.NotFound(c, "Payment method not found")
		return
	}
	if conflict != "" {
		response.Conflict(c, conflict)
		return
	}

	h.adminAuditLogger.LogSafe(ctx, actor.ID,
		"payment_method_updated", "payment_method", actor.ID,
		map[string]interface{}{
			"method_code": code,
			"before":      toMethodResponse(*before),
			"after":       toMethodResponse(*after),
		},
	)

	response.Success(c, gin.H{
		"method":  toMethodResponse(*after),
		"message": "Payment method updated. Only payments created from now on use this config.",
	})
}

// ============================================================================
// POST /admin/payment-methods/:code/preview
// ============================================================================

// PreviewFee simulates the fee/gross for a (possibly unsaved) fee config
// against a sample base amount. Pure computation — never reads or writes
// the DB, so it cannot be runtime authority and cannot corrupt anything.
func (h *AdminPaymentMethodHandler) PreviewFee(c *gin.Context) {
	ctx := c.Request.Context()

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapFinancePaymentMethodView.String()) {
		response.Forbidden(c, "finance.payment_method.view capability required")
		return
	}

	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "method code is required")
		return
	}

	var req previewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Build a hypothetical Method for simulation only. Enabled=false, a
	// placeholder display name, and a placeholder RateSource skip the
	// display-name/channel/rate-source checks in ValidateConfig — those are
	// irrelevant to a fee preview.
	candidate := entity.Method{
		Code:        code,
		DisplayName: "preview",
		Enabled:     false,
		FeeType:     entity.FeeType(req.FeeType),
		FlatAmount:  money.New(req.FlatAmountRupiah),
		PercentBps:  req.PercentBps,
		RateSource:  entity.RateSourcePublicBaseline,
	}
	if req.MinFeeRupiah != nil {
		v := money.New(*req.MinFeeRupiah)
		candidate.MinFee = &v
	}
	if req.MaxFeeRupiah != nil {
		v := money.New(*req.MaxFeeRupiah)
		candidate.MaxFee = &v
	}

	if err := entity.ValidateConfig(candidate); err != nil {
		response.BadRequest(c, "Invalid fee config: "+err.Error())
		return
	}

	baseAmount := money.New(req.BaseAmountRupiah)
	fee, err := entity.CalculateFee(baseAmount, candidate)
	if err != nil {
		response.BadRequest(c, "Failed to calculate preview: "+err.Error())
		return
	}
	gross := baseAmount.Add(fee)

	unclamped, _ := unclampedFee(baseAmount, candidate)
	clamped := !unclamped.Equal(fee)

	response.Success(c, gin.H{
		"method_code":              code,
		"base_amount_rupiah":       baseAmount.Int64(),
		"buyer_payment_fee_rupiah": fee.Int64(),
		"gross_amount_rupiah":      gross.Int64(),
		"clamped":                  clamped,
		"formula":                  formulaExplanation(candidate),
	})
}

// unclampedFee recomputes the fee without min/max clamping, so the handler
// can tell the admin whether a clamp actually applied.
func unclampedFee(baseAmount money.Money, m entity.Method) (money.Money, error) {
	m.MinFee = nil
	m.MaxFee = nil
	return entity.CalculateFee(baseAmount, m)
}

// formulaExplanation renders a short human-readable description of m's fee
// formula for the admin preview UI.
func formulaExplanation(m entity.Method) string {
	switch m.FeeType {
	case entity.FeeTypeFlat:
		return "flat Rp" + moneyString(m.FlatAmount)
	case entity.FeeTypePercent:
		return "ceil(base * " + bpsString(m.PercentBps) + ")"
	case entity.FeeTypePercentPlusFlat:
		return "ceil(base * " + bpsString(m.PercentBps) + ") + Rp" + moneyString(m.FlatAmount)
	default:
		return "unknown fee_type"
	}
}

func moneyString(m money.Money) string {
	return strconv.FormatInt(m.Int64(), 10)
}

func bpsString(bps int64) string {
	return strconv.FormatInt(bps, 10) + " bps"
}
