package serverboot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paymentWireContractFixture is the SINGLE cross-language definition of the
// payment wire contract. It lives with the mobile test suite that consumes it
// and is read from here so the backend half proves the canonical handler still
// emits exactly those keys. There is deliberately no second copy of the shape.
//
// Dart half:
//
//	apps/mobile/test/domains/finance/transaction/payment/data/dto/payment_wire_contract_test.dart
const paymentWireContractFixture = "../../../apps/mobile/test/fixtures/payment_wire_contract.json"

type paymentWireBlock struct {
	Handler       string                     `json:"handler"`
	ForbiddenKeys []string                   `json:"forbidden_response_keys"`
	Response      map[string]json.RawMessage `json:"response"`
}

type paymentWireFixture struct {
	CreatePaymentRequest map[string]json.RawMessage `json:"create_payment_request"`
	CreatePayment        paymentWireBlock           `json:"create_payment"`
	ListPaymentMethods   paymentWireBlock           `json:"list_payment_methods"`
	GetPayment           paymentWireBlock           `json:"get_payment"`
}

func loadPaymentWireFixture(t *testing.T) paymentWireFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(paymentWireContractFixture))
	if err != nil {
		t.Fatalf("read payment wire fixture %s: %v", paymentWireContractFixture, err)
	}

	var fixture paymentWireFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode payment wire fixture: %v", err)
	}
	if len(fixture.CreatePayment.Response) == 0 ||
		len(fixture.ListPaymentMethods.Response) == 0 ||
		len(fixture.GetPayment.Response) == 0 {
		t.Fatal("payment wire fixture must declare all three response blocks")
	}
	return fixture
}

// ginHKeysInFunc returns every top-level key of every `gin.H{...}` literal
// inside the function whose signature starts with signature.
//
// Only depth-1 keys are collected, so nested literals (e.g. the inline
// func() *string closures used for nullable response fields) cannot leak keys
// into the set. String literals are skipped while scanning, so braces inside
// strings never affect brace depth.
func ginHKeysInFunc(t *testing.T, src, signature string) map[string]bool {
	t.Helper()

	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("function %q not found in dependencies.go", signature)
	}
	body := src[start:]
	if next := strings.Index(body[len(signature):], "\nfunc "); next >= 0 {
		body = body[:len(signature)+next]
	}

	keys := make(map[string]bool)
	const marker = "gin.H{"

	for offset := 0; ; {
		idx := strings.Index(body[offset:], marker)
		if idx < 0 {
			break
		}
		i := offset + idx + len(marker)
		depth := 1

		for i < len(body) && depth > 0 {
			ch := body[i]
			switch {
			case ch == '"':
				// Read the whole string literal; capture it if it sits at
				// depth 1 and is immediately followed by a key separator.
				j := i + 1
				for j < len(body) {
					if body[j] == '\\' {
						j += 2
						continue
					}
					if body[j] == '"' {
						break
					}
					j++
				}
				if j >= len(body) {
					i = j
					break
				}
				literal := body[i+1 : j]
				rest := strings.TrimLeft(body[j+1:], " \t")
				if depth == 1 && strings.HasPrefix(rest, ":") {
					keys[literal] = true
				}
				i = j + 1
			case ch == '{':
				depth++
				i++
			case ch == '}':
				depth--
				i++
			default:
				i++
			}
		}
		offset = i
	}

	if len(keys) == 0 {
		t.Fatalf("no gin.H keys found in %q", signature)
	}
	return keys
}

// TestPaymentWireContract_BackendEmitsFixtureKeys proves the canonical payment
// handlers still emit every response key the shared fixture (and therefore the
// mobile parsers) relies on, and none of the keys that were purged.
//
// This fails if the backend renames payment_id, drops payment_url, or
// re-introduces a dropped/renamed key such as net_amount or coin_discount.
func TestPaymentWireContract_BackendEmitsFixtureKeys(t *testing.T) {
	fixture := loadPaymentWireFixture(t)

	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	if len(fixture.CreatePaymentRequest) == 0 {
		t.Fatal("fixture must declare the canonical create-payment request body")
	}

	blocks := []struct {
		name string
		wire paymentWireBlock
	}{
		{"create_payment", fixture.CreatePayment},
		{"list_payment_methods", fixture.ListPaymentMethods},
		{"get_payment", fixture.GetPayment},
	}

	for _, block := range blocks {
		emitted := ginHKeysInFunc(t, code, block.wire.Handler)

		// Positive proof: every declared key must actually be emitted.
		declared := make([]string, 0, len(block.wire.Response))
		for key := range block.wire.Response {
			declared = append(declared, key)
		}
		for _, key := range declared {
			if !emitted[key] {
				t.Errorf("%s: fixture declares %q but %s never emits it (wire contract drift)",
					block.name, key, block.wire.Handler)
			}
		}

		// Negative proof: purged / stale keys must not come back.
		for _, forbidden := range block.wire.ForbiddenKeys {
			if emitted[forbidden] {
				t.Errorf("%s: %s must not emit %q (purged or renamed wire key)",
					block.name, block.wire.Handler, forbidden)
			}
		}
	}

	// The request body is backend-bound by CreatePaymentRequest; assert the
	// canonical keys are the ones the struct declares.
	for _, key := range []string{"order_id", "payment_method_code", "price_snapshot_id"} {
		if _, ok := fixture.CreatePaymentRequest[key]; !ok {
			t.Errorf("fixture request body is missing canonical key %q", key)
		}
	}
	if _, ok := fixture.CreatePaymentRequest["coin_discount"]; ok {
		t.Error("fixture request body must not declare the stale key coin_discount")
	}
	// CANONICAL PAY-B: K is fixed at Order creation (pricing_tokens.coins_used);
	// POST /payments must NOT accept a client coin authority.
	if _, ok := fixture.CreatePaymentRequest["coins_to_use"]; ok {
		t.Error("fixture request body must not declare coins_to_use — payment derives K from the pricing token")
	}

	for _, key := range []string{`json:"order_id"`, `json:"payment_method_code"`, `json:"price_snapshot_id"`} {
		if !strings.Contains(code, key) {
			t.Errorf("CreatePaymentRequest binding is missing %s", key)
		}
	}
	if strings.Contains(code, `json:"coin_discount"`) {
		t.Error("CreatePaymentRequest must not bind the outdated json key coin_discount")
	}
	if strings.Contains(code, `json:"coins_to_use"`) {
		t.Error("CreatePaymentRequest must not bind coins_to_use — payment derives K from the pricing token")
	}
}

// TestPaymentWireContract_NoDroppedMoneyColumnOnPaymentReads pins the reason
// net_amount must stay absent: migration 000037 dropped payments.net_amount, so
// no payment response may re-emit it.
func TestPaymentWireContract_NoDroppedMoneyColumnOnPaymentReads(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	for _, signature := range []string{
		"func (h *CorePaymentHandler) CreatePayment(",
		"func (h *CorePaymentHandler) GetPayment(",
		"func (h *CorePaymentHandler) ListPaymentMethods(",
	} {
		emitted := ginHKeysInFunc(t, code, signature)
		if emitted["net_amount"] {
			t.Errorf("%s must not emit net_amount (payments.net_amount dropped in migration 000037)", signature)
		}
		if emitted["coin_discount"] {
			t.Errorf("%s must not emit coin_discount (renamed to coins_to_use in migration 000036)", signature)
		}
	}

	if !strings.Contains(code, `"coins_to_use"`) {
		t.Fatal("payment responses must expose the canonical coins_to_use key")
	}
}
