package finance_test

import (
	"testing"

	"github.com/labuda/backend/internal/finance"
	ledgerintf "github.com/labuda/backend/internal/finance/repository"
)

// TestCanonicalAccountTypeConstants is the static guard introduced by TASK 40.
//
// It locks two invariants in place:
//
//  1. The canonical account-type strings stored in financial_accounts.account_type
//     are UPPERCASE enum-style values. The system_account_bootstrap inserts these
//     exact strings; any divergence at a lookup site silently breaks
//     GetSystemAccountID.
//
//  2. The historical duplicate constant block in package
//     internal/finance/repository (which previously held lowercase strings and
//     blocked the release ledger flow) is now an alias of package finance and
//     resolves to the same canonical values. Re-introducing a lowercase
//     literal there will fail this test.
//
// If you need to add a new account type, add it to account_types.go AND extend
// this test — do NOT bypass the canonical source.
func TestCanonicalAccountTypeConstants(t *testing.T) {
	cases := []struct {
		name  string
		got   string
		want  string
		alias string // value as exported by internal/finance/repository
	}{
		{"GatewayClearing", finance.AccountGatewayClearing, "GATEWAY_CLEARING", ledgerintf.AccountGatewayClearing},
		// "Escrow" is contest-legacy residue (TASK 40 ADDENDUM). The case is
		// retained here only to mirror the canonical-source constant; when
		// the contest cleanup batch removes finance.AccountEscrow the
		// constant, this line, and the alias must all go together. The
		// assertion does NOT preserve contest behavior — it locks string
		// value parity for as long as the symbol exists.
		{"Escrow", finance.AccountEscrow, "ESCROW", ledgerintf.AccountEscrow},
		{"SellerPayable", finance.AccountSellerPayable, "SELLER_PAYABLE", ledgerintf.AccountSellerPayable},
		{"PlatformRevenue", finance.AccountPlatformRevenue, "PLATFORM_REVENUE", ledgerintf.AccountPlatformRevenue},
		{"BankSettlement", finance.AccountBankSettlement, "BANK_SETTLEMENT", ledgerintf.AccountBankSettlement},
		{"BuyerRefundable", finance.AccountBuyerRefundable, "BUYER_REFUNDABLE", ledgerintf.AccountBuyerRefundable},
		// Constants defined ONLY in package finance (no alias in the
		// repository constants block — only exercise the canonical side).
		{"WithdrawalPending", finance.AccountWithdrawalPending, "WITHDRAWAL_PENDING", finance.AccountWithdrawalPending},
		{"WithdrawalCommitted", finance.AccountWithdrawalCommitted, "WITHDRAWAL_COMMITTED", finance.AccountWithdrawalCommitted},
		{"PlatformBank", finance.AccountPlatformBank, "PLATFORM_BANK", finance.AccountPlatformBank},
		{"UserServiceCredit", finance.AccountUserServiceCredit, "USER_SERVICE_CREDIT", finance.AccountUserServiceCredit},
		{"AdRevenue", finance.AccountAdRevenue, "AD_REVENUE", finance.AccountAdRevenue},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Fatalf("finance.Account%s = %q, want canonical uppercase %q",
					c.name, c.got, c.want)
			}
			if c.alias != c.want {
				t.Fatalf("ledgerintf.Account%s = %q, must alias canonical %q (lowercase strings forbidden)",
					c.name, c.alias, c.want)
			}
		})
	}
}


