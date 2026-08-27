//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// reseedCanonicalPaymentMethods (PASS_19B) re-inserts the migration
// 000006/000007 seed rows via ON CONFLICT DO NOTHING.
//
// testdb.TestDB truncates every table (including payment_methods) after any
// test in this file that passes, and migrations only run once per test
// binary (sync.Once in pkg/testdb) — so a later test in this same package can
// otherwise find the seed rows already wiped out by an earlier test's
// cleanup. Calling this at the top of every test makes each test
// self-contained regardless of run order or a sibling test's truncation.
func reseedCanonicalPaymentMethods(ctx context.Context, t *testing.T, testDB *testdb.TestDB) {
	t.Helper()
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
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
			        'public_baseline', 'test seed'),
			    ('credit_card', 'Kartu Kredit/Debit', true, 'percent_plus_flat', 2000, 290, NULL, NULL,
			        ARRAY['credit_card'], 30,
			        'public_baseline', 'test seed'),
			    ('dana', 'DANA', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['dana'], 25,
			        'public_baseline', 'test seed'),
			    ('convenience_store', 'Indomaret / Alfamart', true, 'flat', 5000, 0, NULL, NULL,
			        ARRAY['alfamart', 'indomaret'], 40,
			        'public_baseline', 'test seed')
			ON CONFLICT (method_code) DO NOTHING
		`)
		return err
	})
	if err != nil {
		t.Fatalf("reseedCanonicalPaymentMethods: %v", err)
	}
}

// TestPaymentMethodRepository_SeedData_DBProven proves the migration
// 000006/000007 seed rows are readable through the canonical repository with
// the correct fee formula and rate_source for each method, against a real
// Postgres instance running the full migration chain.
//
// PASS_19A owner policy: the active baseline is exactly bank_transfer, qris,
// dana, convenience_store, and credit_card (card payment). ShopeePay,
// SPayLater, Kredivo, and Akulaku PayLater are forbidden and must never
// appear.
//
// Requires PostgreSQL (see pkg/testdb) — run with: go test -tags integration
func TestPaymentMethodRepository_SeedData_DBProven(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	reseedCanonicalPaymentMethods(ctx, t, testDB)

	repo := NewPaymentMethodRepository()

	var methods []entity.Method
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		methods, err = repo.ListEnabled(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}

	if len(methods) != 5 {
		t.Fatalf("expected 5 seeded enabled methods (bank_transfer, qris, dana, convenience_store, credit_card), got %d", len(methods))
	}

	byCode := make(map[string]entity.Method, len(methods))
	for _, m := range methods {
		byCode[m.Code] = m
	}

	forbidden := []string{"shopeepay", "spaylater", "kredivo", "akulaku"}
	for _, code := range forbidden {
		if _, ok := byCode[code]; ok {
			t.Fatalf("forbidden method %q must never be seeded/enabled", code)
		}
	}

	bankTransfer, ok := byCode["bank_transfer"]
	if !ok {
		t.Fatal("missing seeded method: bank_transfer")
	}
	if bankTransfer.FeeType != entity.FeeTypeFlat || bankTransfer.FlatAmount.Int64() != 4000 {
		t.Fatalf("bank_transfer: got fee_type=%s flat=%d, want flat/4000", bankTransfer.FeeType, bankTransfer.FlatAmount.Int64())
	}
	if bankTransfer.RateSource != entity.RateSourcePublicBaseline {
		t.Fatalf("bank_transfer: rate_source = %q, want public_baseline", bankTransfer.RateSource)
	}

	qris, ok := byCode["qris"]
	if !ok {
		t.Fatal("missing seeded method: qris")
	}
	if qris.FeeType != entity.FeeTypePercent || qris.PercentBps != 70 {
		t.Fatalf("qris: got fee_type=%s bps=%d, want percent/70", qris.FeeType, qris.PercentBps)
	}
	if qris.MinFee == nil || qris.MinFee.Int64() != 500 {
		t.Fatalf("qris: expected min_fee_rupiah=500")
	}
	if qris.RateSource != entity.RateSourcePublicBaseline {
		t.Fatalf("qris: rate_source = %q, want public_baseline", qris.RateSource)
	}

	dana, ok := byCode["dana"]
	if !ok {
		t.Fatal("missing seeded method: dana")
	}
	if dana.FeeType != entity.FeeTypePercent || dana.PercentBps != 150 {
		t.Fatalf("dana: got fee_type=%s bps=%d, want percent/150", dana.FeeType, dana.PercentBps)
	}
	if dana.RateSource != entity.RateSourcePublicBaseline {
		t.Fatalf("dana: rate_source = %q, want public_baseline", dana.RateSource)
	}
	for _, ch := range dana.MidtransChannels {
		if ch != "dana" {
			t.Fatalf("dana: unexpected midtrans channel %q", ch)
		}
	}

	cstore, ok := byCode["convenience_store"]
	if !ok {
		t.Fatal("missing seeded method: convenience_store")
	}
	if cstore.FeeType != entity.FeeTypeFlat || cstore.FlatAmount.Int64() != 5000 {
		t.Fatalf("convenience_store: got fee_type=%s flat=%d, want flat/5000", cstore.FeeType, cstore.FlatAmount.Int64())
	}
	for _, ch := range cstore.MidtransChannels {
		if ch != "alfamart" && ch != "indomaret" {
			t.Fatalf("convenience_store: unexpected midtrans channel %q (paylater/unsafe channel leaked into seed)", ch)
		}
	}

	creditCard, ok := byCode["credit_card"]
	if !ok {
		t.Fatal("missing seeded method: credit_card (card payment must remain enabled per PASS_19A addendum)")
	}
	if creditCard.FeeType != entity.FeeTypePercentPlusFlat || creditCard.PercentBps != 290 || creditCard.FlatAmount.Int64() != 2000 {
		t.Fatalf("credit_card: got fee_type=%s bps=%d flat=%d, want percent_plus_flat/290/2000",
			creditCard.FeeType, creditCard.PercentBps, creditCard.FlatAmount.Int64())
	}
	if creditCard.RateSource != entity.RateSourcePublicBaseline {
		t.Fatalf("credit_card: rate_source = %q, want public_baseline", creditCard.RateSource)
	}
	// Card payment must map only to the safe card channel — never a
	// paylater/installment product riding along on the same method row.
	for _, ch := range creditCard.MidtransChannels {
		if ch != "credit_card" && ch != "debit_card" {
			t.Fatalf("credit_card: unexpected midtrans channel %q (paylater/unsafe channel leaked into card method)", ch)
		}
	}

	// GetByCode for an unknown code must surface ErrMethodNotFound so the
	// CreatePayment handler can reject it with a clean 400.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetByCode(ctx, tx, "nonexistent_method")
		return err
	})
	if err != ErrMethodNotFound {
		t.Fatalf("GetByCode(unknown) = %v, want ErrMethodNotFound", err)
	}
}

// TestPaymentMethodRepository_AdminUpdate_DBProven (PASS_18W) proves the
// admin write path round-trips correctly against real Postgres:
//   - Update persists every field, including disabling a method.
//   - ListEnabled excludes a disabled method (buyer-facing list).
//   - ListAll still returns the disabled method (admin-facing list).
//   - CountEnabledExcluding correctly counts the OTHER enabled methods.
//
// Requires PostgreSQL — run with: go test -tags integration
func TestPaymentMethodRepository_AdminUpdate_DBProven(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	reseedCanonicalPaymentMethods(ctx, t, testDB)

	repo := NewPaymentMethodRepository()

	// Disable qris and change its fee formula.
	minFee := money.New(1000)
	var updated *entity.Method
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updated, err = repo.Update(ctx, tx, "qris", UpdateMethodInput{
			DisplayName: "QRIS (disabled for test)",
			Enabled:     false,
			FeeType:     entity.FeeTypePercent,
			PercentBps:  100,
			MinFee:      &minFee,
			// allowed: disabled method needs no channels. Empty slice, NOT nil —
			// midtrans_channels is a NOT NULL DB column (migration 000006) and a
			// nil Go slice pgx-encodes as SQL NULL, which that NOT NULL
			// constraint rejects. This is what every real caller sends too: the
			// admin UI always marshals its channels array as `[]`, never omits
			// the field or sends `null` (PASS_19B discovery — see the PASS_19B
			// report's "new P1" for the underlying repository.Update() gap this
			// masks: it does not coalesce a nil MidtransChannels before writing).
			MidtransChannels: []string{},
			SortOrder:        99,
			RateSource:       entity.RateSourceManualOverride,
			RateSourceNote:   "test override",
		})
		return err
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Enabled {
		t.Fatal("expected updated.Enabled = false")
	}
	if updated.PercentBps != 100 {
		t.Fatalf("expected PercentBps = 100, got %d", updated.PercentBps)
	}

	// ListEnabled must now exclude qris.
	var enabled []entity.Method
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		enabled, err = repo.ListEnabled(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}
	for _, m := range enabled {
		if m.Code == "qris" {
			t.Fatal("disabled qris must not appear in ListEnabled (buyer-facing)")
		}
	}
	if len(enabled) != 4 {
		t.Fatalf("expected 4 enabled methods after disabling qris, got %d", len(enabled))
	}

	// ListAll must still include qris (admin sees disabled methods too).
	var all []entity.Method
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		all, err = repo.ListAll(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 total methods, got %d", len(all))
	}

	// CountEnabledExcluding("qris") must equal the 4 other enabled methods,
	// regardless of qris's own current state.
	var count int
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		count, err = repo.CountEnabledExcluding(ctx, tx, "qris")
		return err
	})
	if err != nil {
		t.Fatalf("CountEnabledExcluding: %v", err)
	}
	if count != 4 {
		t.Fatalf("CountEnabledExcluding(qris) = %d, want 4", count)
	}

	// Update on an unknown code must surface ErrMethodNotFound.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.Update(ctx, tx, "nonexistent_method", UpdateMethodInput{DisplayName: "X", FeeType: entity.FeeTypeFlat})
		return err
	})
	if err != ErrMethodNotFound {
		t.Fatalf("Update(unknown) = %v, want ErrMethodNotFound", err)
	}
}

// TestPaymentMethodRepository_RateSource_DBProven (PASS_19A) proves
// rate_source/rate_source_note/merchant_verified_at round-trip through
// Update/GetByCode/ListAll against real Postgres, and that the DB CHECK
// constraint rejects an unknown rate_source value.
//
// Requires PostgreSQL — run with: go test -tags integration
func TestPaymentMethodRepository_RateSource_DBProven(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	reseedCanonicalPaymentMethods(ctx, t, testDB)

	repo := NewPaymentMethodRepository()

	// Seeded rows must default to public_baseline (migration 000007).
	seeded, err := getMethod(ctx, t, testDB, repo, "bank_transfer")
	if err != nil {
		t.Fatalf("GetByCode(bank_transfer): %v", err)
	}
	if seeded.RateSource != entity.RateSourcePublicBaseline {
		t.Fatalf("bank_transfer: rate_source = %q, want public_baseline", seeded.RateSource)
	}
	if seeded.MerchantVerifiedAt != nil {
		t.Fatal("bank_transfer: expected merchant_verified_at = nil for an unverified public baseline row")
	}

	// Admin marks bank_transfer merchant_verified with a note and timestamp.
	verifiedAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	var updated *entity.Method
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updated, err = repo.Update(ctx, tx, "bank_transfer", UpdateMethodInput{
			DisplayName:        "Transfer Bank (Virtual Account)",
			Enabled:            true,
			FeeType:            entity.FeeTypeFlat,
			FlatAmount:         money.New(4000),
			MidtransChannels:   []string{"bca_va"},
			SortOrder:          10,
			RateSource:         entity.RateSourceMerchantVerified,
			RateSourceNote:     "Confirmed against Midtrans merchant dashboard 2026-08-01.",
			MerchantVerifiedAt: &verifiedAt,
		})
		return err
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.RateSource != entity.RateSourceMerchantVerified {
		t.Fatalf("rate_source = %q, want merchant_verified", updated.RateSource)
	}
	if updated.RateSourceNote != "Confirmed against Midtrans merchant dashboard 2026-08-01." {
		t.Fatalf("rate_source_note = %q, want the confirmation note", updated.RateSourceNote)
	}
	if updated.MerchantVerifiedAt == nil || !updated.MerchantVerifiedAt.Equal(verifiedAt) {
		t.Fatalf("merchant_verified_at = %v, want %v", updated.MerchantVerifiedAt, verifiedAt)
	}

	// An unknown rate_source must be rejected by the DB CHECK constraint —
	// defense in depth behind the handler's own entity.ValidateConfig check.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.Update(ctx, tx, "bank_transfer", UpdateMethodInput{
			DisplayName:      "Transfer Bank (Virtual Account)",
			Enabled:          true,
			FeeType:          entity.FeeTypeFlat,
			FlatAmount:       money.New(4000),
			MidtransChannels: []string{"bca_va"},
			SortOrder:        10,
			RateSource:       entity.RateSource("bogus"),
		})
		return err
	})
	if err == nil {
		t.Fatal("expected the rate_source CHECK constraint to reject an unknown value")
	}
}

// TestPaymentMethodRepository_Update_NilChannels_DBProven (PASS_19C) proves
// the fix for the reachable production defect found in PASS_19B: passing a
// nil Go MidtransChannels for a disabled method must not 500. Before the
// fix, pgx encoded a nil slice as SQL NULL, which the midtrans_channels NOT
// NULL column (migration 000006) rejected.
func TestPaymentMethodRepository_Update_NilChannels_DBProven(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	reseedCanonicalPaymentMethods(ctx, t, testDB)

	repo := NewPaymentMethodRepository()

	// Disable qris with MidtransChannels explicitly nil — the exact shape an
	// admin PUT that omits/nulls the field produces (see
	// admin_payment_method_handler_test.go's
	// TestUpdateMethod_DisabledWithOmittedChannels_ReachesDB).
	var updated *entity.Method
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updated, err = repo.Update(ctx, tx, "qris", UpdateMethodInput{
			DisplayName:      "QRIS (disabled, nil channels)",
			Enabled:          false,
			FeeType:          entity.FeeTypePercent,
			PercentBps:       70,
			MidtransChannels: nil,
			SortOrder:        20,
			RateSource:       entity.RateSourceManualOverride,
			RateSourceNote:   "test: nil channels must not 500",
		})
		return err
	})
	if err != nil {
		t.Fatalf("Update with nil MidtransChannels must succeed for a disabled method, got: %v", err)
	}
	if updated.MidtransChannels == nil {
		t.Fatal("scanned-back MidtransChannels must not be nil (DB column must not be NULL)")
	}
	if len(updated.MidtransChannels) != 0 {
		t.Fatalf("expected empty MidtransChannels, got %v", updated.MidtransChannels)
	}

	// Prove the DB column itself is a non-null empty array, not NULL — the
	// actual invariant this pass protects, independent of how pgx happens to
	// scan it back into Go.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var isNull bool
		var length int
		scanErr := tx.QueryRow(ctx, `
			SELECT midtrans_channels IS NULL, COALESCE(array_length(midtrans_channels, 1), 0)
			FROM payment_methods WHERE method_code = 'qris'
		`).Scan(&isNull, &length)
		if scanErr != nil {
			return scanErr
		}
		if isNull {
			t.Fatal("REGRESSION: midtrans_channels is NULL in the DB — nil-coalescing fix regressed")
		}
		if length != 0 {
			t.Fatalf("expected 0-length array, got length %d", length)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verify DB column: %v", err)
	}

	// GetByCode must scan the same non-nil empty slice back.
	fetched, err := getMethod(ctx, t, testDB, repo, "qris")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if len(fetched.MidtransChannels) != 0 {
		t.Fatalf("GetByCode: expected empty MidtransChannels, got %v", fetched.MidtransChannels)
	}

	// Re-enabling the method without channels must still be rejected — this
	// pass fixes the DB-safety net, not the enabled+no-channels business
	// rule, which is (and must remain) entity.ValidateConfig's job, enforced
	// upstream of Update.
	candidate := entity.Method{
		Code:             "qris",
		DisplayName:      "QRIS",
		Enabled:          true,
		FeeType:          entity.FeeTypePercent,
		PercentBps:       70,
		MidtransChannels: nil,
		RateSource:       entity.RateSourcePublicBaseline,
	}
	if err := entity.ValidateConfig(candidate); err != entity.ErrEnabledMethodNeedsChannels {
		t.Fatalf("re-enabling qris with nil channels must be rejected by ValidateConfig, got: %v", err)
	}
}

func getMethod(ctx context.Context, t *testing.T, testDB *testdb.TestDB, repo *PaymentMethodRepository, code string) (*entity.Method, error) {
	t.Helper()
	var m *entity.Method
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		m, err = repo.GetByCode(ctx, tx, code)
		return err
	})
	return m, err
}
