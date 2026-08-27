package worker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/jackc/pgx/v5"
)

// nonShipmentMockTx wraps mockTx with a Query func returning configurable rows.
func newNonShipmentTx(rows [][]any) *mockTx {
	return &mockTx{
		QueryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{rows: rows}, nil
		},
	}
}

func TestSellerNonShipmentRule_Name(t *testing.T) {
	rule := NewSellerNonShipmentRule(nil, nil)
	if rule.Name() != "seller_non_shipment" {
		t.Fatalf("expected name 'seller_non_shipment', got %q", rule.Name())
	}
}

// TestSellerNonShipment_CountThresholdTriggers verifies that >=3 cancelled_timeout orders fire the alert.
func TestSellerNonShipment_CountThresholdTriggers(t *testing.T) {
	sellerID := uuid.New()
	tx := newNonShipmentTx([][]any{
		{sellerID, 4, 6, 10}, // 4 cancelled_timeout, 6 fulfilled, 10 total
	})

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert to trigger for count threshold")
	}
	if finding.AlertType != alertentity.AlertTypeSellerNonShipment {
		t.Fatalf("expected alert type seller_non_shipment, got %q", finding.AlertType)
	}
	if finding.EntityType != "seller" {
		t.Fatalf("expected entity type 'seller', got %q", finding.EntityType)
	}
	if finding.EntityID != sellerID {
		t.Fatalf("expected entity ID %s, got %s", sellerID, finding.EntityID)
	}
	if finding.Metadata["cancelled_timeout_count"] != 4 {
		t.Fatalf("expected cancelled_timeout_count=4, got %v", finding.Metadata["cancelled_timeout_count"])
	}
	if finding.Metadata["window_days"] != SellerNonShipmentWindowDays {
		t.Fatalf("expected window_days=%d, got %v", SellerNonShipmentWindowDays, finding.Metadata["window_days"])
	}
}

// TestSellerNonShipment_RateThresholdTriggers verifies that low fulfillment rate (< 80%) with sufficient volume fires the alert.
func TestSellerNonShipment_RateThresholdTriggers(t *testing.T) {
	sellerID := uuid.New()
	// 1 cancelled_timeout (below count threshold), but 7/12 = 58.3% fulfillment (below 80%)
	tx := newNonShipmentTx([][]any{
		{sellerID, 1, 7, 12},
	})

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert to trigger for rate threshold")
	}
	if finding.AlertType != alertentity.AlertTypeSellerNonShipment {
		t.Fatalf("expected alert type seller_non_shipment, got %q", finding.AlertType)
	}
	rate := finding.Metadata["fulfillment_rate"].(float64)
	if rate >= 80.0 {
		t.Fatalf("expected fulfillment_rate < 80, got %.1f", rate)
	}
}

// TestSellerNonShipment_LowVolumeDoesNotTrigger verifies that a seller with few orders and low rate does not trigger.
func TestSellerNonShipment_LowVolumeDoesNotTrigger(t *testing.T) {
	// Empty results = no sellers matched the HAVING clause
	tx := newNonShipmentTx(nil)

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected {
		t.Fatal("expected no alert for low volume seller")
	}
	if finding != nil {
		t.Fatal("expected nil finding for low volume seller")
	}
}

// TestSellerNonShipment_HealthySellerDoesNotTrigger verifies that a healthy seller does not trigger.
func TestSellerNonShipment_HealthySellerDoesNotTrigger(t *testing.T) {
	// No rows returned = the DB HAVING clause eliminated healthy sellers
	tx := newNonShipmentTx(nil)

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected {
		t.Fatal("expected no alert for healthy seller")
	}
	if finding != nil {
		t.Fatal("expected nil finding for healthy seller")
	}
}

// TestSellerNonShipment_SeverityEscalation verifies that high count escalates severity.
func TestSellerNonShipment_SeverityEscalation(t *testing.T) {
	sellerID := uuid.New()
	// 6 >= 3*2 = escalate to high
	tx := newNonShipmentTx([][]any{
		{sellerID, 6, 4, 10},
	})

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert to trigger")
	}
	if finding.Severity != alertentity.SeverityHigh {
		t.Fatalf("expected severity high for 6 cancelled_timeout, got %q", finding.Severity)
	}
}

// TestSellerNonShipment_DefaultSeverityWarning verifies that moderate count stays warning.
func TestSellerNonShipment_DefaultSeverityWarning(t *testing.T) {
	sellerID := uuid.New()
	// 3 < 3*2 = stays warning
	tx := newNonShipmentTx([][]any{
		{sellerID, 3, 7, 10},
	})

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert to trigger")
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Fatalf("expected severity warning for 3 cancelled_timeout, got %q", finding.Severity)
	}
}

// TestSellerNonShipment_GroupKeyFormat verifies dedup group key structure.
func TestSellerNonShipment_GroupKeyFormat(t *testing.T) {
	sellerID := uuid.New()
	tx := newNonShipmentTx([][]any{
		{sellerID, 5, 5, 10},
	})

	rule := NewSellerNonShipmentRule(nil, nil)
	_, finding, _ := rule.Detect(context.Background(), tx)

	expected := "seller_non_shipment:" + sellerID.String()
	if finding.GroupKey == nil || *finding.GroupKey != expected {
		t.Fatalf("expected group key %q, got %v", expected, finding.GroupKey)
	}
}

// TestSellerNonShipment_MultipleSellersPicksWorst verifies worst offender is alerted.
func TestSellerNonShipment_MultipleSellersPicksWorst(t *testing.T) {
	worst := uuid.New()
	other := uuid.New()
	tx := newNonShipmentTx([][]any{
		{worst, 8, 2, 10}, // highest cancelled_timeout (query ORDER BY DESC)
		{other, 3, 7, 10},
	})

	rule := NewSellerNonShipmentRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert to trigger")
	}
	if finding.EntityID != worst {
		t.Fatalf("expected worst offender %s, got %s", worst, finding.EntityID)
	}
}


