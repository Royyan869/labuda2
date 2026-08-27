package worker

import (
	"context"
	"testing"
	"time"

	"github.com/labuda/backend/internal/finance/entity"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// K-3 FIX PROOF: reconciliation's own severity taxonomy must map to a
// matching AlertSeverity granularity (MEDIUM->Medium, HIGH->High,
// CRITICAL->Critical) instead of collapsing MEDIUM and HIGH both into
// "High", which made every non-passed reconciliation result indistinguishable
// in the admin alert UI.
func TestReconcileSeverityToAlertSeverity_PreservesGranularity(t *testing.T) {
	cases := []struct {
		reconcile entity.ReconcileSeverity
		alert     alertentity.AlertSeverity
	}{
		{entity.SeverityReconcileMedium, alertentity.SeverityMedium},
		{entity.SeverityReconcileHigh, alertentity.SeverityHigh},
		{entity.SeverityReconcileCritical, alertentity.SeverityCritical},
		{entity.SeverityReconcileLow, alertentity.SeverityLow},
	}

	for _, tc := range cases {
		got := reconcileSeverityToAlertSeverity(tc.reconcile)
		assert.Equal(t, tc.alert, got, "reconcile severity %q must map to alert severity %q", tc.reconcile, tc.alert)
	}

	// MEDIUM and HIGH must no longer collapse to the same alert severity —
	// that was the entire point of K-3.
	assert.NotEqual(t,
		reconcileSeverityToAlertSeverity(entity.SeverityReconcileMedium),
		reconcileSeverityToAlertSeverity(entity.SeverityReconcileHigh),
		"MEDIUM and HIGH reconciliation severities must be distinguishable in the resulting alert severity",
	)
}

// TestHandleIssues_UsesSeverityMapping_NotJustEscalationBoolean drives the
// real handleIssues -> createAlert path (not just the pure mapping function)
// to prove the alert actually created carries the mapped severity.
func TestHandleIssues_UsesSeverityMapping_NotJustEscalationBoolean(t *testing.T) {
	ctx := context.Background()
	checkedAt := time.Now()

	newWorker := func() (*ReconciliationWorkerV2, *MockAlertService) {
		alerts := NewMockAlertService()
		w := NewReconciliationWorkerV2(nil, zap.NewNop(), nil, alerts, ReconciliationConfigV2{
			EnableAlerting: true,
		})
		return w, alerts
	}

	lastAlertSeverity := func(alerts *MockAlertService) alertentity.AlertSeverity {
		created := alerts.GetAlertsCreated()
		require.NotEmpty(t, created)
		return created[len(created)-1].Severity
	}

	w, alerts := newWorker()
	mediumResult := entity.NewReconciliationResult(checkedAt, 2, 1, entity.SeverityReconcileMedium, entity.ReconcileDetails{})
	w.handleIssues(ctx, mediumResult, []ReconciliationIssue{{Type: "account_mismatch", Severity: entity.SeverityReconcileMedium}})
	assert.Equal(t, alertentity.SeverityMedium, lastAlertSeverity(alerts), "a MEDIUM reconciliation result must alert at Medium, not High")

	w, alerts = newWorker()
	highResult := entity.NewReconciliationResult(checkedAt, 2, 1, entity.SeverityReconcileHigh, entity.ReconcileDetails{})
	w.handleIssues(ctx, highResult, []ReconciliationIssue{{Type: "account_mismatch", Severity: entity.SeverityReconcileHigh}})
	assert.Equal(t, alertentity.SeverityHigh, lastAlertSeverity(alerts), "a HIGH reconciliation result must alert at High, not Critical")

	w, alerts = newWorker()
	criticalResult := entity.NewReconciliationResult(checkedAt, 2, 1, entity.SeverityReconcileCritical, entity.ReconcileDetails{})
	w.handleIssues(ctx, criticalResult, []ReconciliationIssue{{Type: "critical_account", Severity: entity.SeverityReconcileCritical}})
	assert.Equal(t, alertentity.SeverityCritical, lastAlertSeverity(alerts))
}
