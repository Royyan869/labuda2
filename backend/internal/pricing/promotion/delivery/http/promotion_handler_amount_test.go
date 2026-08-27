package http

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestPurchasePackage_GrossAmountNoCentsScaling is a structural regression
// test for the PASS_18N fix. PromotionPackage.PriceAmount is a Rupiah
// integer (the "price_cents" column/field name was a lie — this codebase
// never converted it, so packages were being charged at whatever raw value
// admins entered). Reintroducing a "/100" or "*100" scaling in
// PurchasePackage would either overcharge or undercharge buyers 100x
// relative to the displayed package price. Asserted at the source level,
// matching the convention used by TestWebhookAmountValidationNoCentsDivision
// in payment_webhook_test.go and TestSellerHandler_SubscriptionAmountsNoCentsDivision
// in seller_subscription_initiate_test.go.
func TestPurchasePackage_GrossAmountNoCentsScaling(t *testing.T) {
	f, err := os.Open("promotion_handler.go")
	if err != nil {
		t.Fatalf("failed to open promotion_handler.go: %v", err)
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
			if strings.Contains(line, "func (h *PromotionHandler) PurchasePackage(") {
				inFunc = true
				foundFunc = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		if strings.Contains(line, "/ 100") || strings.Contains(line, "/100") ||
			strings.Contains(line, "* 100") || strings.Contains(line, "*100") {
			t.Fatalf("promotion_handler.go:%d inside PurchasePackage must not scale amounts by 100 (PriceAmount is a Rupiah integer): %q", lineNum, line)
		}

		if braceDepth <= 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan promotion_handler.go: %v", err)
	}
	if !foundFunc {
		t.Fatal("PurchasePackage function not found — promotion purchase logic may have moved")
	}
}
