package application

import (
	"os"
	"strings"
	"testing"
)

func TestRefundHistoryServiceSourceContract(t *testing.T) {
	src, err := os.ReadFile("refund_service.go")
	if err != nil {
		t.Fatalf("read refund_service.go: %v", err)
	}
	text := string(src)

	for _, want := range []string{
		"func (s *RefundService) ListRefundHistoryByOrderID(",
		"limit+1",
		"s.refundRepo.ListByOrderID",
		"nextCursor = &RefundCursor{",
		"HasMore:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("refund_service.go missing %q", want)
		}
	}
}
