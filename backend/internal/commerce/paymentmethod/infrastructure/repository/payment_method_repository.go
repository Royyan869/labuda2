// DOMAIN: PAYMENT METHOD
// Repository for the canonical payment_methods table: buyer-facing reads
// (ListEnabled, GetByCode) and admin-facing config management (ListAll,
// Update, CountEnabledExcluding — PASS_18W).

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ErrMethodNotFound is returned when a method_code does not exist in the
// canonical table.
var ErrMethodNotFound = errors.New("payment method not found")

// PaymentMethodRepository reads and (PASS_18W) administers canonical payment
// methods and their fee formulas. method_code is immutable — every write
// path here is scoped to a single existing code and never creates/renames
// rows, matching the "canonical method code" doctrine from PASS_18V.
type PaymentMethodRepository struct{}

// NewPaymentMethodRepository creates a new PaymentMethodRepository.
func NewPaymentMethodRepository() *PaymentMethodRepository {
	return &PaymentMethodRepository{}
}

const selectColumns = `
	method_code, display_name, enabled, fee_type,
	flat_amount_rupiah, percent_bps, min_fee_rupiah, max_fee_rupiah,
	midtrans_channels, sort_order,
	rate_source, rate_source_note, merchant_verified_at
`

// ListEnabled returns all enabled methods ordered by sort_order.
func (r *PaymentMethodRepository) ListEnabled(ctx context.Context, tx db.Tx) ([]entity.Method, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+selectColumns+`
		FROM payment_methods
		WHERE enabled = true
		ORDER BY sort_order ASC, method_code ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled payment methods: %w", err)
	}
	defer rows.Close()

	var methods []entity.Method
	for rows.Next() {
		m, err := scanMethod(rows)
		if err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return methods, nil
}

// GetByCode returns the method with the given code, regardless of enabled
// status. Returns ErrMethodNotFound if no such code exists. Callers MUST
// check m.Enabled themselves before accepting the method for a payment.
func (r *PaymentMethodRepository) GetByCode(ctx context.Context, tx db.Tx, code string) (*entity.Method, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM payment_methods
		WHERE method_code = $1
	`, code)

	m, err := scanMethod(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMethodNotFound
		}
		return nil, fmt.Errorf("get payment method by code: %w", err)
	}
	return &m, nil
}

// ListAll returns every method regardless of enabled status, for admin
// config screens (buyers only ever see ListEnabled).
func (r *PaymentMethodRepository) ListAll(ctx context.Context, tx db.Tx) ([]entity.Method, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+selectColumns+`
		FROM payment_methods
		ORDER BY sort_order ASC, method_code ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list all payment methods: %w", err)
	}
	defer rows.Close()

	var methods []entity.Method
	for rows.Next() {
		m, err := scanMethod(rows)
		if err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return methods, nil
}

// CountEnabledExcluding returns how many methods are enabled, excluding
// excludeCode. Used to guard against disabling the last enabled method
// (checkout would become impossible).
func (r *PaymentMethodRepository) CountEnabledExcluding(ctx context.Context, tx db.Tx, excludeCode string) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM payment_methods
		WHERE enabled = true AND method_code != $1
	`, excludeCode).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count enabled payment methods: %w", err)
	}
	return count, nil
}

// UpdateMethodInput holds every admin-editable field of a payment method.
// method_code is intentionally NOT a field here — it is the immutable path
// parameter identifying which row to update, never part of the mutable
// config (see entity.ValidateConfig's doc comment).
type UpdateMethodInput struct {
	DisplayName      string
	Enabled          bool
	FeeType          entity.FeeType
	FlatAmount       money.Money
	PercentBps       int64
	MinFee           *money.Money
	MaxFee           *money.Money
	MidtransChannels []string
	SortOrder        int

	// RateSource/RateSourceNote/MerchantVerifiedAt (PASS_19A) — see
	// entity.ReconcileRateSource and entity.ResolveMerchantVerifiedAt for how
	// callers must derive these before calling Update.
	RateSource         entity.RateSource
	RateSourceNote     string
	MerchantVerifiedAt *time.Time
}

// Update writes input onto the method identified by code and returns the
// saved row. Callers MUST run entity.ValidateConfig (and any
// last-enabled-method guard) before calling this — Update performs no
// validation of its own beyond the DB's CHECK constraints, matching the
// "backend validates, repository persists" split used elsewhere in this
// codebase (see OrderRepository.UpdatePaymentSelectionTx for the same
// pattern). Returns ErrMethodNotFound if code does not exist.
//
// SAFETY: this only ever updates the payment_methods row itself. It never
// touches orders or payments — a config edit affects payment creation from
// this point forward only; Payment.GrossAmount for already-created payments
// is never recalculated (see CorePaymentHandler.CreatePayment, which reads
// this table fresh at creation time and snapshots the result onto the
// payment/order rows).
func (r *PaymentMethodRepository) Update(ctx context.Context, tx db.Tx, code string, input UpdateMethodInput) (*entity.Method, error) {
	var minFee, maxFee *int64
	if input.MinFee != nil {
		v := input.MinFee.Int64()
		minFee = &v
	}
	if input.MaxFee != nil {
		v := input.MaxFee.Int64()
		maxFee = &v
	}

	// midtrans_channels is a NOT NULL DB column (migration 000006); pgx
	// encodes a nil Go slice as SQL NULL, which that constraint rejects. A
	// nil MidtransChannels is a legitimate input here — entity.ValidateConfig
	// allows a disabled method to have no channels, and callers naturally
	// construct that as nil (e.g. an admin PUT that omits/nulls the field
	// entirely) — so this repository, as the last step before the DB, must
	// not trust every caller to remember to pass []string{} instead of nil
	// (PASS_19C: backend fails safely independent of caller/frontend
	// behavior). This never changes what gets validated: ValidateConfig runs
	// on the caller's original value before Update is ever invoked.
	channels := input.MidtransChannels
	if channels == nil {
		channels = []string{}
	}

	row := tx.QueryRow(ctx, `
		UPDATE payment_methods
		SET display_name = $2,
		    enabled = $3,
		    fee_type = $4,
		    flat_amount_rupiah = $5,
		    percent_bps = $6,
		    min_fee_rupiah = $7,
		    max_fee_rupiah = $8,
		    midtrans_channels = $9,
		    sort_order = $10,
		    rate_source = $11,
		    rate_source_note = $12,
		    merchant_verified_at = $13,
		    updated_at = NOW()
		WHERE method_code = $1
		RETURNING `+selectColumns+`
	`,
		code,
		input.DisplayName,
		input.Enabled,
		string(input.FeeType),
		input.FlatAmount.Int64(),
		input.PercentBps,
		minFee,
		maxFee,
		channels,
		input.SortOrder,
		string(input.RateSource),
		input.RateSourceNote,
		input.MerchantVerifiedAt,
	)

	m, err := scanMethod(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMethodNotFound
		}
		return nil, fmt.Errorf("update payment method: %w", err)
	}
	return &m, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMethod(row scanner) (entity.Method, error) {
	var (
		code, displayName, feeType string
		enabled                    bool
		flatAmount, percentBps     int64
		minFee, maxFee             *int64
		midtransChannels           []string
		sortOrder                  int
		rateSource                 string
		rateSourceNote             *string
		merchantVerifiedAt         *time.Time
	)
	if err := row.Scan(
		&code, &displayName, &enabled, &feeType,
		&flatAmount, &percentBps, &minFee, &maxFee,
		&midtransChannels, &sortOrder,
		&rateSource, &rateSourceNote, &merchantVerifiedAt,
	); err != nil {
		return entity.Method{}, err
	}

	out := entity.Method{
		Code:               code,
		DisplayName:        displayName,
		Enabled:            enabled,
		FeeType:            entity.FeeType(feeType),
		FlatAmount:         money.New(flatAmount),
		PercentBps:         percentBps,
		MidtransChannels:   midtransChannels,
		SortOrder:          sortOrder,
		RateSource:         entity.RateSource(rateSource),
		MerchantVerifiedAt: merchantVerifiedAt,
	}
	if rateSourceNote != nil {
		out.RateSourceNote = *rateSourceNote
	}
	if minFee != nil {
		v := money.New(*minFee)
		out.MinFee = &v
	}
	if maxFee != nil {
		v := money.New(*maxFee)
		out.MaxFee = &v
	}
	return out, nil
}
