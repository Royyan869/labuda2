package monitoring

import (
	"os"
	"strings"
	"testing"

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

// TestMonitoringSettledQueries_DeriveFromCanonicalSet keeps the operational
// metrics (orphaned subscription payments, conversion rate) on the same settled
// definition as the payment domain and the recovery worker: 'settlement' alone
// used to be restated here, so a settled capture payment could be counted as
// unconverted while being settled everywhere else.
func TestMonitoringSettledQueries_DeriveFromCanonicalSet(t *testing.T) {
	src, err := os.ReadFile("monitoring_service.go")
	require.NoError(t, err)
	code := codeLines(string(src))

	assert.NotContains(t, code, "p.status = 'settlement'",
		"monitoring must not restate the settled status set in SQL")
	assert.NotContains(t, code, "status = 'settlement'",
		"monitoring must not restate the settled status set in SQL")
	assert.NotContains(t, strings.ToUpper(code), strings.ToUpper("count(*) filter (where status = 'settlement')"),
		"the conversion-rate denominator must use the canonical settled set")

	assert.Contains(t, code, "paymentRepo.SettledPaymentStatuses()",
		"monitoring must read the settled set from the canonical payment authority")
	assert.Contains(t, code, "status::text = ANY($1::text[])",
		"monitoring queries must inject the canonical settled set as a parameter")
}
