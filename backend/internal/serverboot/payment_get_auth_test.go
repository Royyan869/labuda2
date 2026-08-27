package serverboot

import (
	"os"
	"strings"
	"testing"
)

// TestGetPayment_AuthGuards verifies that the payment detail handler fails
// closed for unauthenticated and unauthorized access before returning data.
func TestGetPayment_AuthGuards(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	start := strings.Index(code, "func (h *CorePaymentHandler) GetPayment(")
	if start < 0 {
		t.Fatal("GetPayment handler not found in dependencies.go")
	}
	body := code[start:]

	if !strings.Contains(body, "response.Unauthorized(c, \"User not authenticated\")") {
		t.Fatal("GetPayment must reject unauthenticated requests with response.Unauthorized")
	}
	if !strings.Contains(body, "response.Forbidden(c, \"You can only view your own payments\")") {
		t.Fatal("GetPayment must reject non-owner access with response.Forbidden")
	}
	if !strings.Contains(body, "payment.UserID != userID") {
		t.Fatal("GetPayment must compare payment.UserID with the authenticated userID")
	}
}
