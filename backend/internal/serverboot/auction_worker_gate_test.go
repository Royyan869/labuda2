package serverboot

import (
	"os"
	"testing"

	"go.uber.org/zap/zaptest"
)

// TestAuctionStartWorker_DefaultOn verifies the start worker is default-ON.
func TestAuctionStartWorker_DefaultOn(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Unsetenv("DISABLE_AUCTION_START_WORKER")

	if !workerEnabled("AUCTION_START_WORKER", true, log) {
		t.Fatal("AUCTION_START_WORKER should be enabled by default")
	}
}

// TestAuctionStartWorker_DisabledByEnv verifies explicit disable works.
func TestAuctionStartWorker_DisabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_AUCTION_START_WORKER", "true")
	defer os.Unsetenv("DISABLE_AUCTION_START_WORKER")

	if workerEnabled("AUCTION_START_WORKER", true, log) {
		t.Fatal("AUCTION_START_WORKER should be disabled when DISABLE_AUCTION_START_WORKER=true")
	}
}

// TestAuctionStartWorker_EnabledByEnv verifies explicit enable works.
func TestAuctionStartWorker_EnabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_AUCTION_START_WORKER", "false")
	defer os.Unsetenv("DISABLE_AUCTION_START_WORKER")

	if !workerEnabled("AUCTION_START_WORKER", true, log) {
		t.Fatal("AUCTION_START_WORKER should be enabled when DISABLE_AUCTION_START_WORKER=false")
	}
}

// TestAuctionEndWorker_DefaultOn verifies the end worker is default-ON.
func TestAuctionEndWorker_DefaultOn(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Unsetenv("DISABLE_AUCTION_END_WORKER")

	if !workerEnabled("AUCTION_END_WORKER", true, log) {
		t.Fatal("AUCTION_END_WORKER should be enabled by default")
	}
}

// TestAuctionEndWorker_DisabledByEnv verifies explicit disable works.
func TestAuctionEndWorker_DisabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_AUCTION_END_WORKER", "true")
	defer os.Unsetenv("DISABLE_AUCTION_END_WORKER")

	if workerEnabled("AUCTION_END_WORKER", true, log) {
		t.Fatal("AUCTION_END_WORKER should be disabled when DISABLE_AUCTION_END_WORKER=true")
	}
}

// TestAuctionEndWorker_EnabledByEnv verifies explicit enable works.
func TestAuctionEndWorker_EnabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_AUCTION_END_WORKER", "false")
	defer os.Unsetenv("DISABLE_AUCTION_END_WORKER")

	if !workerEnabled("AUCTION_END_WORKER", true, log) {
		t.Fatal("AUCTION_END_WORKER should be enabled when DISABLE_AUCTION_END_WORKER=false")
	}
}

// TestAuctionSettlementWorker_DefaultOn verifies the settlement worker is default-ON.
// RUNTIME_PROVEN B81 2026-05-26.
func TestAuctionSettlementWorker_DefaultOn(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Unsetenv("DISABLE_AUCTION_SETTLEMENT_WORKER")

	if !workerEnabled("AUCTION_SETTLEMENT_WORKER", true, log) {
		t.Fatal("AUCTION_SETTLEMENT_WORKER should be enabled by default")
	}
}

// TestAuctionSettlementWorker_EnabledByEnv verifies explicit enable works.
func TestAuctionSettlementWorker_EnabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_AUCTION_SETTLEMENT_WORKER", "false")
	defer os.Unsetenv("DISABLE_AUCTION_SETTLEMENT_WORKER")

	if !workerEnabled("AUCTION_SETTLEMENT_WORKER", true, log) {
		t.Fatal("AUCTION_SETTLEMENT_WORKER should be enabled when DISABLE_AUCTION_SETTLEMENT_WORKER=false")
	}
}

// TestAuctionSettlementWorker_DisabledByEnv verifies explicit disable works.
func TestAuctionSettlementWorker_DisabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_AUCTION_SETTLEMENT_WORKER", "true")
	defer os.Unsetenv("DISABLE_AUCTION_SETTLEMENT_WORKER")

	if workerEnabled("AUCTION_SETTLEMENT_WORKER", true, log) {
		t.Fatal("AUCTION_SETTLEMENT_WORKER should be disabled when DISABLE_AUCTION_SETTLEMENT_WORKER=true")
	}
}

// TestAuctionWorkers_NotInDangerousRegistry verifies auction workers do not
// require ACK_DANGEROUS_* keys (they are safe to enable without operator ack).
func TestAuctionWorkers_NotInDangerousRegistry(t *testing.T) {
	for _, name := range []string{"AUCTION_START_WORKER", "AUCTION_END_WORKER", "AUCTION_SETTLEMENT_WORKER"} {
		_, err := CheckDangerousDormantGuard(name)
		if err != nil {
			t.Errorf("%s should not be in dangerous registry, got error: %v", name, err)
		}
	}
}


