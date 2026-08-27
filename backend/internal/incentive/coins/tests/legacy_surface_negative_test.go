package tests

import (
	"reflect"
	"testing"

	auditapp "github.com/labuda/backend/internal/governance/audit/application"
	coinsapp "github.com/labuda/backend/internal/incentive/coins/application"
)

func TestLegacySpendAuthorityIsAbsent(t *testing.T) {
	coinsType := reflect.TypeOf(&coinsapp.CoinsService{})
	for _, method := range []string{"SpendCoins", "SpendCoinsTx"} {
		if _, ok := coinsType.MethodByName(method); ok {
			t.Fatalf("legacy coins spend method %s must remain absent", method)
		}
	}

	auditType := reflect.TypeOf(&auditapp.AuditService{})
	if _, ok := auditType.MethodByName("CoinsSpent"); ok {
		t.Fatalf("legacy audit method CoinsSpent must remain absent")
	}
}
