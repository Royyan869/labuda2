package serverboot

import (
	"os"
	"testing"
)

// TestCheckDangerousDormantGuard_AllRegistered verifies every worker in the
// dangerousDormantWorkers registry is checked correctly.
func TestCheckDangerousDormantGuard_AllRegistered(t *testing.T) {
	// Sanity: the registry should contain exactly 2 entries.
	// Batch 71: MODERATION_EVENT_HANDLER promoted to default-ON (safe enablement).
	if got := len(dangerousDormantWorkers); got != 2 {
		t.Fatalf("dangerousDormantWorkers has %d entries, want 2", got)
	}

	for name, wantPrereq := range dangerousDormantWorkers {
		t.Run(name+"_blocked_without_ack", func(t *testing.T) {
			// Ensure the ACK env is NOT set.
			ackKey := "ACK_DANGEROUS_" + name
			os.Unsetenv(ackKey)

			prereq, err := CheckDangerousDormantGuard(name)
			if err == nil {
				t.Fatalf("expected error for %s without ack, got nil", name)
			}
			if prereq != wantPrereq {
				t.Errorf("prerequisite = %q, want %q", prereq, wantPrereq)
			}
		})

		t.Run(name+"_allowed_with_ack", func(t *testing.T) {
			ackKey := "ACK_DANGEROUS_" + name
			os.Setenv(ackKey, "true")
			defer os.Unsetenv(ackKey)

			prereq, err := CheckDangerousDormantGuard(name)
			if err != nil {
				t.Fatalf("expected no error for %s with ack, got: %v", name, err)
			}
			if prereq != wantPrereq {
				t.Errorf("prerequisite = %q, want %q", prereq, wantPrereq)
			}
		})
	}
}

// TestCheckDangerousDormantGuard_UnknownWorker verifies that workers NOT in
// the dangerous registry pass through without error (no false positives).
func TestCheckDangerousDormantGuard_UnknownWorker(t *testing.T) {
	prereq, err := CheckDangerousDormantGuard("SOME_SAFE_WORKER")
	if err != nil {
		t.Fatalf("unknown worker should pass, got error: %v", err)
	}
	if prereq != "" {
		t.Errorf("prerequisite should be empty for unknown worker, got %q", prereq)
	}
}

// TestCheckDangerousDormantGuard_IdempotencyCleanupWorkerNotDangerous verifies
// that IDEMPOTENCY_CLEANUP_WORKER is NOT in the dangerous dormant registry.
// It only deletes response-cache rows (idempotency_records) — no money risk.
func TestCheckDangerousDormantGuard_IdempotencyCleanupWorkerNotDangerous(t *testing.T) {
	prereq, err := CheckDangerousDormantGuard("IDEMPOTENCY_CLEANUP_WORKER")
	if err != nil {
		t.Fatalf("IDEMPOTENCY_CLEANUP_WORKER should not be in dangerous registry, got error: %v", err)
	}
	if prereq != "" {
		t.Errorf("prerequisite should be empty, got %q", prereq)
	}
}

// TestCheckDangerousDormantGuard_SellerMetricsWorkerNotDangerous verifies
// that SELLER_METRICS_WORKER is NOT in the dangerous dormant registry.
// It only writes seller_monthly_metrics snapshots — no money, tier, or authority mutation.
func TestCheckDangerousDormantGuard_SellerMetricsWorkerNotDangerous(t *testing.T) {
	prereq, err := CheckDangerousDormantGuard("SELLER_METRICS_WORKER")
	if err != nil {
		t.Fatalf("SELLER_METRICS_WORKER should not be in dangerous registry, got error: %v", err)
	}
	if prereq != "" {
		t.Errorf("prerequisite should be empty, got %q", prereq)
	}
}

// TestCheckDangerousDormantGuard_AckValueMustBeExactTrue verifies partial or
// wrong ack values are rejected.
func TestCheckDangerousDormantGuard_AckValueMustBeExactTrue(t *testing.T) {
	const name = "PAYMENT_EXPIRY_WORKER"
	ackKey := "ACK_DANGEROUS_" + name

	badValues := []string{"yes", "1", "TRUE", "True", "on", ""}
	for _, val := range badValues {
		os.Setenv(ackKey, val)
		_, err := CheckDangerousDormantGuard(name)
		if err == nil {
			t.Errorf("ack value %q should be rejected, but got nil error", val)
		}
		os.Unsetenv(ackKey)
	}

	// Only exact "true" (lowercase) is accepted.
	os.Setenv(ackKey, "true")
	defer os.Unsetenv(ackKey)
	_, err := CheckDangerousDormantGuard(name)
	if err != nil {
		t.Errorf("ack value \"true\" should be accepted, got: %v", err)
	}
}


