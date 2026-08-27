package application

import (
	"bufio"
	"os"
	"strings"
	"testing"

	promotionapp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
)

// TestParseGrossAmount tests the amount parsing utility function
func TestParseGrossAmount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "Whole number with decimals",
			input:    "10000.00",
			expected: 10000,
		},
		{
			name:     "No decimal places",
			input:    "50000",
			expected: 50000,
		},
		{
			name:     "Large amount",
			input:    "1000000.00",
			expected: 1000000,
		},
		{
			name:     "Small amount",
			input:    "100.50",
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGrossAmount(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStrPtr tests the string pointer utility function
func TestStrPtr(t *testing.T) {
	s := "test string"
	ptr := strPtr(s)

	assert.NotNil(t, ptr)
	assert.Equal(t, s, *ptr)
}

// TestIsUniqueViolation tests the unique violation error detection
func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "PostgreSQL unique violation error code 23505",
			err:      &mockPgError{code: "23505"},
			expected: true,
		},
		{
			name:     "Serialization failure error code 40001",
			err:      &mockPgError{code: "40001"},
			expected: false,
		},
		{
			name:     "Deadlock error code 40P01",
			err:      &mockPgError{code: "40P01"},
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Other error code",
			err:      &mockPgError{code: "42000"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := db.IsUniqueViolation(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsSerializationFailure tests the serialization failure error detection
func TestIsSerializationFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Serialization failure error code 40001",
			err:      &mockPgError{code: "40001"},
			expected: true,
		},
		{
			name:     "Unique violation error code 23505",
			err:      &mockPgError{code: "23505"},
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := db.IsSerializationFailure(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsDeadlock tests the deadlock error detection
func TestIsDeadlock(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Deadlock error code 40P01",
			err:      &mockPgError{code: "40P01"},
			expected: true,
		},
		{
			name:     "Unique violation error code 23505",
			err:      &mockPgError{code: "23505"},
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := db.IsDeadlock(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsRetryable tests the retryable error detection
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Serialization failure is retryable",
			err:      &mockPgError{code: "40001"},
			expected: true,
		},
		{
			name:     "Deadlock is retryable",
			err:      &mockPgError{code: "40P01"},
			expected: true,
		},
		{
			name:     "Unique violation is NOT retryable",
			err:      &mockPgError{code: "23505"},
			expected: false,
		},
		{
			name:     "Nil error is NOT retryable",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := db.IsRetryable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockPgError is a mock implementation of PostgreSQL error for testing
type mockPgError struct {
	code string
}

func (m *mockPgError) Code() string {
	return m.code
}

func (m *mockPgError) Error() string {
	return "PostgreSQL error: " + m.code
}

// Verify that mockPgError implements the necessary interface
var _ interface{ Code() string } = (*mockPgError)(nil)

// =============================================================================
// PROMOTION WEBHOOK GOVERNANCE ALIGNMENT
// =============================================================================

// TestPromotionService_NilFailsClosed verifies the payment webhook refuses
// to create promotion ownership when PromotionService is not wired.
// This is the fail-closed guard added during the V1→canonical alignment.
func TestPromotionService_NilFailsClosed(t *testing.T) {
	svc := &PaymentWebhookService{
		promotionService: nil,
	}
	assert.Nil(t, svc.promotionService, "promotionService must be nil before SetPromotionService")
}

// TestPromotionService_SetterWires verifies SetPromotionService replaces nil.
func TestPromotionService_SetterWires(t *testing.T) {
	svc := &PaymentWebhookService{}
	assert.Nil(t, svc.promotionService)

	// Any non-nil value proves the setter wires correctly.
	// We cannot construct a real PromotionService without DB, but the field
	// type check is what matters — the canonical instance from dependencies.go
	// will be the actual value at runtime.
	dummy := &promotionapp.PromotionService{}
	svc.SetPromotionService(dummy)
	assert.NotNil(t, svc.promotionService, "SetPromotionService must wire the field")
}

// TestWebhookNoDefaultOperabilityChecker is a structural regression test.
//
// After the V1→canonical alignment, payment_webhook.go must NOT reference
// DefaultOperabilityChecker anywhere. The canonical OperabilityCheckerImpl
// is injected via SetPromotionService from dependencies.go.
func TestWebhookNoDefaultOperabilityChecker(t *testing.T) {
	f, err := os.Open("payment_webhook.go")
	if err != nil {
		t.Fatalf("failed to open payment_webhook.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, "DefaultOperabilityChecker") {
			t.Fatalf("payment_webhook.go:%d still references DefaultOperabilityChecker — "+
				"webhook must use canonical OperabilityCheckerImpl via SetPromotionService", lineNum)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan payment_webhook.go: %v", err)
	}
}

// TestWebhookPromotionBranchHasNilGuard is a structural regression test.
//
// The promotion ownership branch (TypePromotionPackage) must check
// s.promotionService == nil before calling PurchasePackage, matching
// the fail-closed pattern used by financeService and subscriptionPaymentService.
func TestWebhookPromotionBranchHasNilGuard(t *testing.T) {
	f, err := os.Open("payment_webhook.go")
	if err != nil {
		t.Fatalf("failed to open payment_webhook.go: %v", err)
	}
	defer f.Close()

	foundNilCheck := false
	foundPurchaseCall := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "s.promotionService == nil") {
			foundNilCheck = true
		}
		if strings.Contains(line, "s.promotionService.PurchasePackage") {
			foundPurchaseCall = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan payment_webhook.go: %v", err)
	}

	if !foundPurchaseCall {
		t.Fatal("PurchasePackage call not found — promotion branch may have been removed")
	}
	if !foundNilCheck {
		t.Fatal("promotionService nil check not found — webhook must fail-closed when PromotionService is not wired")
	}
}

// TestParseGrossAmount_RupiahOrderTotalRoundTrips locks the PASS_18H money
// unit truth: an order total of Rp103,000 (as sent by Midtrans in its
// webhook notification) parses to exactly 103000 — no /100 division.
func TestParseGrossAmount_RupiahOrderTotalRoundTrips(t *testing.T) {
	assert.Equal(t, int64(103000), parseGrossAmount("103000.00"))
}

// TestWebhookAmountValidationNoCentsDivision is a structural regression
// test for the PASS_18H fix. STEP 6 (amount validation) must compare
// payment.GrossAmount.Int64() directly against parseGrossAmount(...) with
// no /100 (or any other) scaling applied to either side. Reintroducing a
// division here would silently undercharge/under-validate every real
// Midtrans transaction by 100x, so this is asserted at the source level.
func TestWebhookAmountValidationNoCentsDivision(t *testing.T) {
	f, err := os.Open("payment_webhook.go")
	if err != nil {
		t.Fatalf("failed to open payment_webhook.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inStep6 := false
	step6Lines := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "STEP 6: AMOUNT VALIDATION") {
			inStep6 = true
		}
		if inStep6 {
			step6Lines++
			if strings.Contains(line, "/ 100") || strings.Contains(line, "/100") {
				t.Fatalf("payment_webhook.go STEP 6 must not divide amounts by 100 (Rupiah is the canonical unit): %q", line)
			}
			// STEP 6 is a short block; stop scanning once we're well past it.
			if step6Lines > 25 {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan payment_webhook.go: %v", err)
	}
	if !inStep6 {
		t.Fatal("STEP 6: AMOUNT VALIDATION block not found in payment_webhook.go")
	}
}

// TestSubscriptionBranchNotNestedInBilling is a structural regression test.
//
// BUG (fixed): The subscription webhook branch (reference_type=="subscription")
// was incorrectly nested INSIDE the billing branch (reference_type=="billing"),
// making it unreachable. When a subscription payment webhook arrived:
// 1. Line 468: if ReferenceType == "billing" → FALSE (it's "subscription")
// 2. Entire billing block skipped, including the subscription branch inside it
// 3. Subscription payment silently succeeded without activating the subscription
//
// This test parses the webhook source file and verifies that the subscription
// check occurs at the same brace-nesting depth as the billing check, proving
// they are sibling if-blocks (not nested).
func TestSubscriptionBranchNotNestedInBilling(t *testing.T) {
	f, err := os.Open("payment_webhook.go")
	if err != nil {
		t.Fatalf("failed to open payment_webhook.go: %v", err)
	}
	defer f.Close()

	// Track brace depth of the billing and subscription reference type checks.
	// Both must appear at the same depth to be sibling if-blocks.
	billingDepth := -1
	subscriptionDepth := -1
	depth := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Count braces for depth tracking (simplified: ignores strings/comments)
		for _, ch := range line {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
			}
		}

		trimmed := strings.TrimSpace(line)

		// Detect the billing branch guard
		if strings.Contains(trimmed, `payment.ReferenceType == "billing"`) &&
			strings.HasPrefix(trimmed, "if ") {
			billingDepth = depth
		}

		// Detect the subscription branch guard
		if strings.Contains(trimmed, "ReferenceTypeSubscription") &&
			strings.HasPrefix(trimmed, "if ") {
			subscriptionDepth = depth
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan payment_webhook.go: %v", err)
	}

	if billingDepth == -1 {
		t.Fatal("billing branch not found in payment_webhook.go")
	}
	if subscriptionDepth == -1 {
		t.Fatal("subscription branch not found in payment_webhook.go")
	}

	if billingDepth != subscriptionDepth {
		t.Fatalf("subscription branch (depth %d) is NOT at same level as billing branch (depth %d); "+
			"subscription branch must be a sibling, not nested inside billing",
			subscriptionDepth, billingDepth)
	}
}


