package serverboot

import (
	"os"
	"strings"
	"testing"
)

// TestSnapBuilderAuthorityResidue proves the duplicate Snap builder is gone:
// there is exactly ONE Snap creation authority (integration/payment/application
// SnapService), and serverboot no longer builds Snap requests itself.
func TestSnapBuilderAuthorityResidue(t *testing.T) {
	if _, err := os.Stat("midtrans_snap_builder.go"); err == nil {
		t.Fatal("serverboot/midtrans_snap_builder.go must be purged; Snap creation is canonical")
	}
	if _, err := os.Stat("midtrans_snap_builder_test.go"); err == nil {
		t.Fatal("serverboot/midtrans_snap_builder_test.go must be purged with its owner")
	}

	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)
	for _, forbidden := range []string{
		"buildSnapRequest(",
		"SnapBuilderInput",
		"type MidtransGateway interface",
		"midtransClient:",
	} {
		if strings.Contains(code, forbidden) {
			t.Fatalf("serverboot must not own Snap construction (%q); use paymentApp.SnapService", forbidden)
		}
	}
	if !strings.Contains(code, "paymentApp.NewSnapService(") {
		t.Fatal("serverboot must construct the canonical SnapService")
	}
}
