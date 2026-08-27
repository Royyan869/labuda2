package repository

import (
	"os"
	"strings"
	"testing"
)

// TestUpdate_OnlyTouchesPaymentMethodsTable is a source-level regression
// guard (PASS_18W) proving PaymentMethodRepository.Update's SQL can only
// ever mutate the payment_methods table. This is the structural half of
// "existing payment/order amounts are not mutated by config change" — the
// other half (Payment.GrossAmount for an already-created payment stays
// exactly as computed at creation time) follows from CorePaymentHandler.
// CreatePayment reading payment_methods fresh and snapshotting the result
// onto the payment/order rows, which it never re-reads or re-writes after
// creation.
func TestUpdate_OnlyTouchesPaymentMethodsTable(t *testing.T) {
	src, err := os.ReadFile("payment_method_repository.go")
	if err != nil {
		t.Fatalf("read payment_method_repository.go: %v", err)
	}
	code := string(src)

	funcStart := strings.Index(code, "func (r *PaymentMethodRepository) Update(")
	if funcStart < 0 {
		t.Fatal("Update method not found in payment_method_repository.go")
	}
	rest := code[funcStart:]
	nextFunc := strings.Index(rest[len("func (r *PaymentMethodRepository) Update("):], "\nfunc ")
	body := rest
	if nextFunc >= 0 {
		body = rest[:nextFunc+len("func (r *PaymentMethodRepository) Update(")]
	}

	if !strings.Contains(body, "UPDATE payment_methods") {
		t.Fatal("MISSING: Update must issue an UPDATE payment_methods statement")
	}
	forbidden := []string{
		"UPDATE orders", "INSERT INTO orders", "DELETE FROM orders",
		"UPDATE payments", "INSERT INTO payments", "DELETE FROM payments",
	}
	for _, f := range forbidden {
		if strings.Contains(body, f) {
			t.Fatalf("REGRESSION: Update must never touch orders/payments tables, found %q", f)
		}
	}
}

// TestUpdate_NeverSendsNilChannelsToSQL (PASS_19C) is a source-level
// regression guard proving Update coalesces a nil MidtransChannels to an
// empty slice before it ever reaches the SQL parameter list. Without this,
// a nil Go slice pgx-encodes as SQL NULL, which the midtrans_channels NOT
// NULL column (migration 000006) rejects — reachable whenever a caller
// passes nil for a disabled method (a legitimate business state per
// entity.ValidateConfig's disabled-with-no-channels rule).
func TestUpdate_NeverSendsNilChannelsToSQL(t *testing.T) {
	src, err := os.ReadFile("payment_method_repository.go")
	if err != nil {
		t.Fatalf("read payment_method_repository.go: %v", err)
	}
	code := string(src)

	funcStart := strings.Index(code, "func (r *PaymentMethodRepository) Update(")
	if funcStart < 0 {
		t.Fatal("Update method not found in payment_method_repository.go")
	}
	rest := code[funcStart:]
	nextFunc := strings.Index(rest[len("func (r *PaymentMethodRepository) Update("):], "\nfunc ")
	body := rest
	if nextFunc >= 0 {
		body = rest[:nextFunc+len("func (r *PaymentMethodRepository) Update(")]
	}

	if !strings.Contains(body, "channels == nil") {
		t.Fatal("REGRESSION: Update must guard against a nil MidtransChannels before the SQL call (PASS_19C)")
	}
	if strings.Contains(body, "input.MidtransChannels,\n") {
		t.Fatal("REGRESSION: Update must pass the nil-coalesced local variable to SQL, not input.MidtransChannels directly")
	}
}
