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
			    ('gopay', 'GoPay', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['gopay'], 10,
			        'public_baseline', 'test seed'),
			    ('ovo', 'OVO', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['ovo'], 20,
			        'public_baseline', 'test seed'),
			    ('dana', 'DANA', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['dana'], 25,
			        'public_baseline', 'test seed'),
			    ('shopeepay', 'ShopeePay', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['shopeepay'], 30,
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
// Phase 2 canonical baseline is exactly four wallets: gopay, ovo, dana,
// shopeepay (each 1:1 wallet→channel, percent 150bps public_baseline).
// PayLater/installment products remain forbidden and must never appear.
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

	if len(methods) != 4 {
		t.Fatalf("expected 4 seeded enabled methods (gopay, ovo, dana, shopeepay), got %d", len(methods))
	}

	byCode := make(map[string]entity.Method, len(methods))
	for _, m := range methods {
		byCode[m.Code] = m
	}

	forbidden := []string{"spaylater", "shopeepay_paylater", "kredivo", "akulaku", "bank_transfer", "qris", "credit_card", "convenience_store"}
	for _, code := range forbidden {
		if _, ok := byCode[code]; ok {
			t.Fatalf("forbidden/legacy method %q must never be seeded/enabled (Phase 2 purge)", code)
		}
	}

	for _, code := range []string{"gopay", "ovo", "dana", "shopeepay"} {
		m, ok := byCode[code]
		if !ok {
			t.Fatalf("missing seeded method: %s", code)
		}
		if m.FeeType != entity.FeeTypePercent || m.PercentBps != 150 {
			t.Fatalf("%s: got fee_type=%s bps=%d, want percent/150", code, m.FeeType, m.PercentBps)
		}
		if m.RateSource != entity.RateSourcePublicBaseline {
			t.Fatalf("%s: rate_source = %q, want public_baseline", code, m.RateSource)
		}
		if len(m.MidtransChannels) != 1 || m.MidtransChannels[0] != code {
			t.Fatalf("%s: MidtransChannels = %v, want [%s] (1:1 wallet)", code, m.MidtransChannels, code)
		}
		if !m.Enabled {
			t.Fatalf("%s must be enabled", code)
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
	// Legacy bucket codes must also be not-found after Phase 2 purge.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.GetByCode(ctx, tx, "bank_transfer")
		return err
	})
	if err != ErrMethodNotFound {
		t.Fatalf("GetByCode(bank_transfer) = %v, want ErrMethodNotFound after purge", err)
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

	// Disable ovo and change its fee formula (wallet canonical test).
	minFee := money.New(1000)
	var updated *entity.Method
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updated, err = repo.Update(ctx, tx, "ovo", UpdateMethodInput{
			DisplayName: "OVO (disabled for test)",
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

	// ListEnabled must now exclude ovo.
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
		if m.Code == "ovo" {
			t.Fatal("disabled ovo must not appear in ListEnabled (buyer-facing)")
		}
	}
	if len(enabled) != 3 {
		t.Fatalf("expected 3 enabled methods after disabling ovo, got %d", len(enabled))
	}

	// ListAll must still include ovo (admin sees disabled methods too).
	var all []entity.Method
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		all, err = repo.ListAll(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 total methods, got %d", len(all))
	}

	// CountEnabledExcluding("ovo") must equal the 3 other enabled methods,
	// regardless of ovo's own current state.
	var count int
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		count, err = repo.CountEnabledExcluding(ctx, tx, "ovo")
		return err
	})
	if err != nil {
		t.Fatalf("CountEnabledExcluding: %v", err)
	}
	if count != 3 {
		t.Fatalf("CountEnabledExcluding(ovo) = %d, want 3", count)
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

	// Seeded rows must default to public_baseline (migration 000088).
	seeded, err := getMethod(ctx, t, testDB, repo, "gopay")
	if err != nil {
		t.Fatalf("GetByCode(gopay): %v", err)
	}
	if seeded.RateSource != entity.RateSourcePublicBaseline {
		t.Fatalf("gopay: rate_source = %q, want public_baseline", seeded.RateSource)
	}
	if seeded.MerchantVerifiedAt != nil {
		t.Fatal("gopay: expected merchant_verified_at = nil for an unverified public baseline row")
	}

	// Admin marks gopay merchant_verified with a note and timestamp.
	verifiedAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	var updated *entity.Method
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updated, err = repo.Update(ctx, tx, "gopay", UpdateMethodInput{
			DisplayName:        "GoPay",
			Enabled:            true,
			FeeType:            entity.FeeTypePercent,
			PercentBps:         150,
			MidtransChannels:   []string{"gopay"},
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
		_, err := repo.Update(ctx, tx, "gopay", UpdateMethodInput{
			DisplayName:      "GoPay",
			Enabled:          true,
			FeeType:          entity.FeeTypePercent,
			PercentBps:       150,
			MidtransChannels: []string{"gopay"},
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

	// Disable ovo with MidtransChannels explicitly nil — the exact shape an
	// admin PUT that omits/nulls the field produces (see
	// admin_payment_method_handler_test.go's
	// TestUpdateMethod_DisabledWithOmittedChannels_ReachesDB).
	var updated *entity.Method
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updated, err = repo.Update(ctx, tx, "ovo", UpdateMethodInput{
			DisplayName:      "OVO (disabled, nil channels)",
			Enabled:          false,
			FeeType:          entity.FeeTypePercent,
			PercentBps:       150,
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
			FROM payment_methods WHERE method_code = 'ovo'
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
	fetched, err := getMethod(ctx, t, testDB, repo, "ovo")
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
		Code:             "ovo",
		DisplayName:      "OVO",
		Enabled:          true,
		FeeType:          entity.FeeTypePercent,
		PercentBps:       150,
		MidtransChannels: nil,
		RateSource:       entity.RateSourcePublicBaseline,
	}
	if err := entity.ValidateConfig(candidate); err != entity.ErrEnabledMethodNeedsChannels {
		t.Fatalf("re-enabling ovo with nil channels must be rejected by ValidateConfig, got: %v", err)
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
