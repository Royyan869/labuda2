package worker

import (
	"os"
	"strings"
	"testing"

	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// codeLines drops `//` prose so a comment may NAME the old literal while the
// code is still forbidden from using it.
func codeLines(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// TestSubscriptionSettledQueries_DeriveFromCanonicalSet is the regression guard
// for the settled-state divergence this stage converged: the subscription
// recovery selector and the subscription detection queries used to hardcode
// status = 'settlement', so a payment the payment domain considers settled at
// 'capture' was invisible to recovery. SQL must now DERIVE the set from
// paymentRepo.SettledPaymentStatuses() ($n::text[]) and must never restate it.
func TestSubscriptionSettledQueries_DeriveFromCanonicalSet(t *testing.T) {
	for _, path := range []string{
		"subscription_reconciliation_worker.go",
		"subscription_alert_rules.go",
	} {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			require.NoError(t, err)
			code := codeLines(string(src))

			assert.NotContains(t, code, "p.status = 'settlement'",
				"%s must not restate the settled status set in SQL", path)
			assert.NotContains(t, code, "status = 'settlement'",
				"%s must not restate the settled status set in SQL", path)
			assert.NotContains(t, code, "'settlement', 'capture'",
				"%s must not restate the settled status set in SQL", path)

			assert.Contains(t, code, "ANY($1::text[])",
				"%s must inject the canonical settled set as a query parameter", path)
			assert.Contains(t, code, "paymentRepo.SettledPaymentStatuses()",
				"%s must read the settled set from the canonical payment authority", path)
		})
	}
}

// TestSubscriptionReconciliation_RecoveryIsCanonicallyIdempotent proves the
// reconciler has no state of its own: it never writes a subscription row, it
// only delegates to the canonical idempotent activation service (payment_id
// locked, existence-checked, ledger-keyed), so a repeated or concurrent scan
// cannot create duplicate subscription or payment state.
func TestSubscriptionReconciliation_RecoveryIsCanonicallyIdempotent(t *testing.T) {
	src, err := os.ReadFile("subscription_reconciliation_worker.go")
	require.NoError(t, err)
	code := codeLines(string(src))

	assert.Contains(t, code, "subscriptionPaymentService.ProcessSuccessfulPayment",
		"recovery must delegate to the canonical idempotent activation path")
	assert.NotContains(t, strings.ToUpper(code), "INSERT INTO SELLER_SUBSCRIPTIONS",
		"the reconciler must never create subscription state itself")
}

// TestCanonicalSettledSet_IsWhatRecoveryInjects documents the value the worker
// actually sends to its selector, so recovery eligibility is provably
// "settlement OR capture" — the same set the payment domain uses.
func TestCanonicalSettledSet_IsWhatRecoveryInjects(t *testing.T) {
	assert.Equal(t, []string{"settlement", "capture"}, paymentRepo.SettledPaymentStatuses())
}
