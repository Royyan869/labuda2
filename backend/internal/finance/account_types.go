package finance

const (
	// System account types
	AccountGatewayClearing = "GATEWAY_CLEARING"
	AccountEscrow          = "ESCROW"
	AccountSellerPayable   = "SELLER_PAYABLE"
	AccountPlatformRevenue = "PLATFORM_REVENUE"
	AccountBankSettlement  = "BANK_SETTLEMENT"
	AccountWithdrawalPending   = "WITHDRAWAL_PENDING"
	AccountWithdrawalCommitted = "WITHDRAWAL_COMMITTED" // Admin-approved withdrawals ready for payout
	AccountPlatformBank    = "PLATFORM_BANK"

	// User account types
	AccountUserServiceCredit = "USER_SERVICE_CREDIT"
	AccountBuyerRefundable   = "BUYER_REFUNDABLE"  // Holds refunds due to buyer
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


