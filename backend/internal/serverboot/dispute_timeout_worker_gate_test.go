package serverboot

import (
	"os"
	"testing"

	"go.uber.org/zap/zaptest"
)

// TestDisputeTimeoutWorker_DefaultOn verifies the worker is default-ON.
// Batch 86: worker enabled after DISPUTE_TIMEOUT_CONVERGED audit.
func TestDisputeTimeoutWorker_DefaultOn(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Unsetenv("DISABLE_DISPUTE_TIMEOUT_WORKER")

	if !workerEnabled("DISPUTE_TIMEOUT_WORKER", true, log) {
		t.Fatal("DISPUTE_TIMEOUT_WORKER should be enabled by default")
	}
}

// TestDisputeTimeoutWorker_DisabledByEnv verifies explicit disable works.
func TestDisputeTimeoutWorker_DisabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_DISPUTE_TIMEOUT_WORKER", "true")
	defer os.Unsetenv("DISABLE_DISPUTE_TIMEOUT_WORKER")

	if workerEnabled("DISPUTE_TIMEOUT_WORKER", true, log) {
		t.Fatal("DISPUTE_TIMEOUT_WORKER should be disabled when DISABLE_DISPUTE_TIMEOUT_WORKER=true")
	}
}

// TestDisputeTimeoutWorker_EnabledByEnv verifies explicit enable works.
func TestDisputeTimeoutWorker_EnabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_DISPUTE_TIMEOUT_WORKER", "false")
	defer os.Unsetenv("DISABLE_DISPUTE_TIMEOUT_WORKER")

	if !workerEnabled("DISPUTE_TIMEOUT_WORKER", true, log) {
		t.Fatal("DISPUTE_TIMEOUT_WORKER should be enabled when DISABLE_DISPUTE_TIMEOUT_WORKER=false")
	}
}

// TestDisputeTimeoutWorker_NotInDangerousRegistry verifies this worker does
// not require ACK_DANGEROUS_* (zero money mutation — safe to enable without
// operator ack).
func TestDisputeTimeoutWorker_NotInDangerousRegistry(t *testing.T) {
	_, err := CheckDangerousDormantGuard("DISPUTE_TIMEOUT_WORKER")
	if err != nil {
		t.Fatalf("DISPUTE_TIMEOUT_WORKER should not be in dangerous registry, got error: %v", err)
	}
}


