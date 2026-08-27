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
)


