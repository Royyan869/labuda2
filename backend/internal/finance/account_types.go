package finance

// AccountClass represents the economic classification of a ledger account.
// This determines how debit/credit entries affect the account balance.
//
// CANONICAL SIGN ARCHITECTURE (Option A — account-aware normal-balance semantics):
//
//	Asset / Expense:  DR increases balance, CR decreases balance
//	Liability / Revenue / Equity:  CR increases balance, DR decreases balance
//
// All economic balances are stored as positive numbers (balance >= 0).
// CHECK(balance >= 0) remains valid for all account classes.
type AccountClass int

const (
	ClassAsset     AccountClass = iota // normal balance = Debit
	ClassLiability                     // normal balance = Credit
	ClassRevenue                       // normal balance = Credit
	ClassExpense                       // normal balance = Debit
	ClassEquity                        // normal balance = Credit
)

// AccountClassOf returns the canonical economic class for an account type string.
func AccountClassOf(accountType string) AccountClass {
	switch accountType {
	// Liabilities: escrow obligations, payables, staging, credits owed
	case AccountGatewayClearing, AccountEscrow, AccountSellerPayable,
		AccountWithdrawalPending, AccountWithdrawalCommitted,
		AccountBuyerRefundable, AccountUserServiceCredit,
		AccountPromoteBalance, AccountPromotionAllocation:
		return ClassLiability
	// Revenue: platform income
	case AccountPlatformRevenue, AccountAdRevenue:
		return ClassRevenue
	// Assets: external bank cash
	case AccountPlatformBank:
		return ClassAsset
	// Platform-owned benefit absorption (Labuda Coins K funding): the platform
	// consumes its own granted usage rights. Expense-like, i.e. debit-normal —
	// DR increases the absorbed benefit, CR releases it.
	case AccountPlatformCoinBenefit:
		return ClassExpense
	// Synthetic reserve: backs clearing obligations (consumed at capture, replenished at settlement)
	case AccountBankSettlement:
		return ClassLiability
	default:
		return ClassAsset // safe default for unknown account types
	}
}

// DebitIncreasesBalance returns true if a debit entry (positive Amount)
// increases the economic balance for this account class.
func (c AccountClass) DebitIncreasesBalance() bool {
	return c == ClassAsset || c == ClassExpense
}

const (
	// System account types
	AccountGatewayClearing     = "GATEWAY_CLEARING"
	AccountEscrow              = "ESCROW"
	AccountSellerPayable       = "SELLER_PAYABLE"
	AccountPlatformRevenue     = "PLATFORM_REVENUE"
	AccountBankSettlement      = "BANK_SETTLEMENT"
	AccountWithdrawalPending   = "WITHDRAWAL_PENDING"
	AccountWithdrawalCommitted = "WITHDRAWAL_COMMITTED" // Admin-approved withdrawals ready for payout
	AccountPlatformBank        = "PLATFORM_BANK"

	// PLATFORM_COIN_BENEFIT is the canonical counterpart of platform-funded
	// Labuda Coins (K) funding.
	//
	// Labuda Coins are platform-owned usage rights / loyalty benefits: not
	// money, not user wallet, not seller money, not a Labuda payable to users,
	// not withdrawable/transferable/cash-redeemable, and coin consumption moves
	// no cash. Honoring K is therefore a platform-owned benefit absorption (a
	// cost) booked as a debit-normal account — never PLATFORM_BANK (cash) and
	// never PLATFORM_REVENUE (income).
	AccountPlatformCoinBenefit = "PLATFORM_COIN_BENEFIT"

	// User account types
	AccountUserServiceCredit = "USER_SERVICE_CREDIT"
	AccountBuyerRefundable   = "BUYER_REFUNDABLE" // Holds refunds due to buyer
	AccountAdRevenue         = "AD_REVENUE"

	// Promotion account types (PROMOTION_FINANCIAL_FOUNDATION)
	// PROMOTE_BALANCE is a seller-owned, non-withdrawable platform usage balance
	// funded through the canonical payment flow. It is NOT cash and NOT usable
	// for checkout; only promotion allocation consumes it and finalization
	// release returns to it.
	// PROMOTION_ALLOCATION is the per-promotion allocation account scoped by
	// (seller, holder=promotion contract). It is NOT a second wallet: it is an
	// ordinary ledger account inside the immutable double-entry ledger.
	AccountPromoteBalance      = "PROMOTE_BALANCE"
	AccountPromotionAllocation = "PROMOTION_ALLOCATION"
)
