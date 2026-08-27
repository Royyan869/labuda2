package serverboot

import (
	"os"
	"strings"
	"testing"
)

// TestCreatePayment_PersistsCanonicalPaymentMethodOnPayment proves the buyer's
// selected method is persisted on the payment record, not on an orders shadow
// field.
func TestCreatePayment_PersistsCanonicalPaymentMethodOnPayment(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	funcStart := strings.Index(code, "func (h *CorePaymentHandler) CreatePayment(")
	if funcStart < 0 {
		t.Fatal("CreatePayment handler not found in dependencies.go")
	}
	rest := code[funcStart:]
	nextFunc := strings.Index(rest[len("func (h *CorePaymentHandler) CreatePayment("):], "\nfunc ")
	body := rest
	if nextFunc >= 0 {
		body = rest[:nextFunc+len("func (h *CorePaymentHandler) CreatePayment(")]
	}

	methodResolved := strings.Index(body, "h.paymentMethodRepo.GetByCode(")
	if methodResolved < 0 {
		t.Fatal("MISSING: CreatePayment must resolve the buyer-selected method via paymentMethodRepo.GetByCode")
	}

	createInput := strings.Index(body, "PaymentMethodCode: &methodCode,")
	if createInput < 0 {
		t.Fatal("MISSING: CreatePayment must persist the canonical method on the payment record")
	}

	paymentInsert := strings.Index(body, "h.paymentRepo.CreatePayment(")
	if paymentInsert < 0 {
		t.Fatal("paymentRepo.CreatePayment call not found in CreatePayment")
	}

	if methodResolved >= paymentInsert {
		t.Errorf("REGRESSION: method resolution (pos %d) must happen BEFORE the payment INSERT (pos %d) - gross_amount depends on it", methodResolved, paymentInsert)
	}
	if createInput <= paymentInsert {
		t.Errorf("REGRESSION: payment method persistence (pos %d) must happen in the CreatePayment input before the payment INSERT call site (pos %d)", createInput, paymentInsert)
	}

	if !strings.Contains(code, `json:"payment_method_code" binding:"required"`) {
		t.Fatal("MISSING: CreatePaymentRequest must require payment_method_code from the client")
	}

	reqStart := strings.Index(code, "type CreatePaymentRequest struct {")
	reqEnd := strings.Index(code[reqStart:], "\n}")
	reqBody := code[reqStart : reqStart+reqEnd]
	if strings.Contains(reqBody, "GrossAmount") {
		t.Fatal("REGRESSION: CreatePaymentRequest must not accept a client-supplied GrossAmount field")
	}
}

// TestCreatePayment_RejectsDisabledMethod proves CreatePayment checks
// method.Enabled and rejects before the payment INSERT.
func TestCreatePayment_RejectsDisabledMethod(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	funcStart := strings.Index(code, "func (h *CorePaymentHandler) CreatePayment(")
	if funcStart < 0 {
		t.Fatal("CreatePayment handler not found in dependencies.go")
	}
	rest := code[funcStart:]
	nextFunc := strings.Index(rest[len("func (h *CorePaymentHandler) CreatePayment("):], "\nfunc ")
	body := rest
	if nextFunc >= 0 {
		body = rest[:nextFunc+len("func (h *CorePaymentHandler) CreatePayment(")]
	}

	disabledGuard := strings.Index(body, "!method.Enabled")
	if disabledGuard < 0 {
		t.Fatal("MISSING: CreatePayment must reject a disabled method (!method.Enabled)")
	}

	paymentInsert := strings.Index(body, "h.paymentRepo.CreatePayment(")
	if paymentInsert < 0 {
		t.Fatal("paymentRepo.CreatePayment call not found in CreatePayment")
	}
	if disabledGuard >= paymentInsert {
		t.Errorf("REGRESSION: disabled-method guard (pos %d) must appear BEFORE the payment INSERT (pos %d)", disabledGuard, paymentInsert)
	}
}

// TestCreatePayment_UsesCanonicalBuyerBaseSnapshot proves CreatePayment and
// ListPaymentMethods read the immutable total_before_coins snapshot rather than
// the mutable total_payable_amount field.
func TestCreatePayment_UsesCanonicalBuyerBaseSnapshot(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	if !strings.Contains(code, "baseAmount := order.TotalBeforeCoinsAmount") {
		t.Fatal("MISSING: payment flow must read order.TotalBeforeCoinsAmount as the buyer base snapshot")
	}
	if strings.Contains(code, "baseAmount := order.TotalPayableAmount") {
		t.Fatal("REGRESSION: payment flow must not derive the buyer base from order.TotalPayableAmount")
	}
	if !strings.Contains(code, "TotalBeforeCoinsAmount") {
		t.Fatal("MISSING: payment flow source should reference the canonical total_before_coins snapshot")
	}
}

// TestCreatePayment_UsesCanonicalPricingTokenCoinCapSnapshot proves CreatePayment
// and ListPaymentMethods use the order-bound pricing token cap instead of a
// local percentage formula.
func TestCreatePayment_UsesCanonicalPricingTokenCoinCapSnapshot(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	createStart := strings.Index(code, "func (h *CorePaymentHandler) CreatePayment(")
	if createStart < 0 {
		t.Fatal("CreatePayment handler not found in dependencies.go")
	}
	listStart := strings.Index(code, "func (h *CorePaymentHandler) ListPaymentMethods(")
	if listStart < 0 {
		t.Fatal("ListPaymentMethods handler not found in dependencies.go")
	}

	createBody := code[createStart:listStart]
	listBody := code[listStart:]

	for _, body := range []struct {
		name string
		text string
	}{
		{name: "CreatePayment", text: createBody},
		{name: "ListPaymentMethods", text: listBody},
	} {
		if !strings.Contains(body.text, "loadOrderPricingTokenSnapshot(") {
			t.Fatalf("MISSING: %s must load the order-bound pricing token snapshot", body.name)
		}
		if !strings.Contains(body.text, "pricingToken.MaxCoinsAllowed") {
			t.Fatalf("MISSING: %s must validate coins against pricingToken.MaxCoinsAllowed", body.name)
		}
		if strings.Contains(body.text, "MaxPercentBps") {
			t.Fatalf("REGRESSION: %s must not use paymentmethodentity.MaxPercentBps for coin caps", body.name)
		}
	}
}

// TestUpdatePaymentSelection_DoesNotTouchStaleOrderPaymentMethod proves the
// order repository update no longer writes a nonexistent orders.payment_method
// column while still persisting the buyer-facing fee projection.
func TestUpdatePaymentSelection_DoesNotTouchStaleOrderPaymentMethod(t *testing.T) {
	src, err := os.ReadFile("../commerce/order/infrastructure/repository/order_repository.go")
	if err != nil {
		t.Fatalf("read order_repository.go: %v", err)
	}
	code := string(src)

	funcStart := strings.Index(code, "func (r *OrderRepository) UpdatePaymentSelectionTx(")
	if funcStart < 0 {
		t.Fatal("UpdatePaymentSelectionTx not found in order_repository.go")
	}
	body := code[funcStart:]
	nextFunc := strings.Index(body[len("func (r *OrderRepository) UpdatePaymentSelectionTx("):], "\nfunc ")
	if nextFunc >= 0 {
		body = body[:nextFunc+len("func (r *OrderRepository) UpdatePaymentSelectionTx(")]
	}

	if strings.Contains(body, "payment_method =") {
		t.Fatal("REGRESSION: UpdatePaymentSelectionTx must not write the stale orders.payment_method field")
	}
	if strings.Contains(body, "total_before_coins_amount =") {
		t.Fatal("REGRESSION: UpdatePaymentSelectionTx must not write total_before_coins_amount")
	}
	if !strings.Contains(body, "service_fee_amount = $2") {
		t.Fatal("MISSING: UpdatePaymentSelectionTx must still persist the buyer payment fee projection")
	}
	if !strings.Contains(body, "total_payable_amount = $3") {
		t.Fatal("MISSING: UpdatePaymentSelectionTx must still persist the buyer gross projection")
	}
}
