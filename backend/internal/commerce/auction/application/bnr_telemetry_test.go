package application

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// Test 1: RecordBNRCheckFailOpen increments the counter
func TestBNRTelemetry_FailOpen_IncrementsCounter(t *testing.T) {
	before := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)
	RecordBNRCheckFailOpen()
	after := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)

	if after != before+1 {
		t.Errorf("counter = %v, want %v (before=%v)", after, before+1, before)
	}
}

// Test 2: Normal allowed path does NOT increment counter
func TestBNRTelemetry_Allowed_DoesNotIncrement(t *testing.T) {
	c := NewBNRStrikeChecker()
	before := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)

	// evaluate with 0 strikes — normal allowed path
	result := c.evaluate(0, nil, time.Now())
	if !result.Allowed {
		t.Fatal("0 strikes should be allowed")
	}

	after := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)
	if after != before {
		t.Errorf("counter changed during normal allow path: before=%v, after=%v", before, after)
	}
}

// Test 3: Normal blocked path does NOT increment counter
func TestBNRTelemetry_Blocked_DoesNotIncrement(t *testing.T) {
	c := NewBNRStrikeChecker()
	before := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)

	// evaluate with 4 strikes — permanent ban, normal blocked path
	struck := time.Now()
	result := c.evaluate(4, &struck, time.Now())
	if result.Allowed {
		t.Fatal("4 strikes should be blocked")
	}

	after := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)
	if after != before {
		t.Errorf("counter changed during normal block path: before=%v, after=%v", before, after)
	}
}

// Test 4: Multiple fail-opens increment correctly
func TestBNRTelemetry_MultipleFailOpens_IncrementCorrectly(t *testing.T) {
	before := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)
	RecordBNRCheckFailOpen()
	RecordBNRCheckFailOpen()
	RecordBNRCheckFailOpen()
	after := testutil.ToFloat64(bnrRestrictionCheckFailedTotal)

	if after != before+3 {
		t.Errorf("counter = %v, want %v (before=%v)", after, before+3, before)
	}
}


