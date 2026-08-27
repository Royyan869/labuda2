package repository

import (
	"os"
	"strings"
	"testing"
)

func TestRefundHistoryRepositorySourceContract(t *testing.T) {
	src, err := os.ReadFile("refund_repository_impl.go")
	if err != nil {
		t.Fatalf("read refund_repository_impl.go: %v", err)
	}
	text := string(src)

	for _, want := range []string{
		"func (r *RefundRepositoryImpl) ListByOrderID(",
		"ORDER BY created_at DESC, id DESC",
		"(created_at, id) < ($2, $3)",
		"r.ListEvidence(ctx, tx, refund.ID)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("refund_repository_impl.go missing %q", want)
		}
	}
}
