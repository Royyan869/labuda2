// Package capability provides the canonical capability string constants for the
// fine-grained authorization system.
//
// CAPABILITY NAMING CONVENTION:
//
//	{cluster}.{resource}.{action}
//
// RULES:
// - All capabilities are explicit string constants
// - No hierarchy or inheritance
// - No implied capabilities
// - Each capability must be explicitly granted
//
// DESIGN PHILOSOPHY:
// - MINIMAL: Only essential capabilities for current domain needs
// - EXPLICIT: No magic strings, all capabilities defined here
// - AUDITABLE: Every capability check is traceable to a constant
// - STRING-BASED: No types, no generics, simple string comparison
//
// DO NOT:
// - Add hierarchy/inheritance
// - Add implied capability logic
// - Add generic permission framework
// - Add policy engine
package capability

// Capability represents a fine-grained permission string.
// Format: {cluster}.{resource}.{action}
type Capability string

// String returns the capability string.
func (c Capability) String() string {
	return string(c)
}

// ============================================================
// FINANCE CLUSTER
// ============================================================

const (
	// CapFinanceWithdrawRead - Can view withdrawal requests
	CapFinanceWithdrawRead Capability = "finance.withdraw.read"

	// CapFinanceWithdrawReview - Can review/approve/reject withdrawal requests
	CapFinanceWithdrawReview Capability = "finance.withdraw.review"

	// CapFinanceDisputeResolve - Can resolve financial disputes
	CapFinanceDisputeResolve Capability = "finance.dispute.resolve"

	// CapFinanceRefundGatewayInitiate - Can manually initiate gateway refund for a refund record
	// Feature-gated via ENABLE_GATEWAY_REFUND_PHASE2 env var
	CapFinanceRefundGatewayInitiate Capability = "finance.refund.gateway.initiate"

	// CapFinancePaymentMethodView - Can view canonical payment methods and
	// their fee config (admin list/detail/preview). PASS_18W.
	CapFinancePaymentMethodView Capability = "finance.payment_method.view"

	// CapFinancePaymentMethodManage - Can edit a payment method's fee
	// formula, enabled status, display name, sort order, and Midtrans
	// channel mapping. Separate from CapFinancePaymentMethodView so an
	// operator can be granted read-only visibility without edit power.
	// Editing only affects payments created after the change — see
	// paymentmethod/infrastructure/repository.PaymentMethodRepository.Update.
	// PASS_18W.
	CapFinancePaymentMethodManage Capability = "finance.payment_method.manage"
)

// ============================================================
// GOVERNANCE CLUSTER
// ============================================================

const (
	// CapGovernanceDashboardView - Can view admin dashboard
	CapGovernanceDashboardView Capability = "governance.dashboard.view"

	// CapGovernanceAlertRead - Can view platform alerts (system health, SLA, fraud signals)
	CapGovernanceAlertRead Capability = "governance.alert.read"

	// CapGovernanceAlertResolve - Can acknowledge, resolve, or mark false-positive on alerts
	CapGovernanceAlertResolve Capability = "governance.alert.resolve"

	// CapGovernanceUserRead - Can view user details and lists
	CapGovernanceUserRead Capability = "governance.user.read"

	// CapGovernanceUserSuspend - Can suspend user accounts
	CapGovernanceUserSuspend Capability = "governance.user.suspend"

	// CapGovernanceUserBan - Can ban user accounts
	CapGovernanceUserBan Capability = "governance.user.ban"

	// CapGovernanceUserActivate - Can activate suspended user accounts.
	// NOTE: Cannot activate banned accounts — use CapGovernanceUserUnban.
	CapGovernanceUserActivate Capability = "governance.user.activate"

	// CapGovernanceUserUnban - Can explicitly unban a banned user account.
	// This is a separate, higher-privilege capability from activate.
	// Ban reversal requires explicit intent, not generic activation.
	CapGovernanceUserUnban Capability = "governance.user.unban"

	// CapGovernanceRoleAssign - Can assign roles to users
	CapGovernanceRoleAssign Capability = "governance.role.assign"

	// CapGovernanceCapabilityAssign - Can grant/revoke capabilities
	CapGovernanceCapabilityAssign Capability = "governance.capability.assign"

	// CapGovernanceAuditRead - Can view audit logs
	CapGovernanceAuditRead Capability = "governance.audit.read"

	// CapGovernanceAuctionCancel - Can emergency-cancel any seller's auction
	// under governance authority (unreachable/abusive seller, trust-and-safety
	// stop). Separate from seller-facing auction cancel; does not grant any
	// seller/selling authority.
	CapGovernanceAuctionCancel Capability = "governance.auction.cancel"
)

// ============================================================
// MODERATION CLUSTER
// ============================================================

const (
	// CapModerationCaseRead - Can view moderation cases and reports
	CapModerationCaseRead Capability = "moderation.case.read"

	// CapModerationContentView - Can view reported content
	CapModerationContentView Capability = "moderation.content.view"

	// CapModerationContentRemove - Can remove content
	CapModerationContentRemove Capability = "moderation.content.remove"

	// CapModerationCaseResolve - Can resolve moderation cases
	CapModerationCaseResolve Capability = "moderation.case.resolve"

	// CapModerationEvidenceRead - Can view original hidden moderation evidence
	CapModerationEvidenceRead Capability = "moderation.evidence.read"

	// CapModerationAppealRead - Can view moderation appeals (list/detail/pending).
	// Appeal content is escalation/governance content and requires appeal-specific
	// trust — it is NOT implied by CapModerationCaseRead, even though both are
	// "read" capabilities in the same cluster.
	CapModerationAppealRead Capability = "moderation.appeal.read"

	// CapModerationAppealReview - Can review moderation appeals (decide/resolve)
	CapModerationAppealReview Capability = "moderation.appeal.review"
)

// ============================================================
// PROMOTION CLUSTER
// ============================================================

const (
	// CapPromotionExternalProductReview - Can review external product promotions
	CapPromotionExternalProductReview Capability = "promotion.external_product.review"

	// CapPromotionPackageManage - Can create, update, enable and disable promotion packages
	CapPromotionPackageManage Capability = "promotion.package.manage"

	// CapPromotionCampaignView - Can view active and historical promotion campaigns (instances)
	CapPromotionCampaignView Capability = "promotion.campaign.view"

	// CapPromotionCampaignStop - Can force-stop a running promotion campaign
	CapPromotionCampaignStop Capability = "promotion.campaign.stop"
)

// ============================================================
// SELLER CLUSTER
// ============================================================

const (
	// CapSellerVerificationReview - Can review seller verification requests
	CapSellerVerificationReview Capability = "seller.verification.review"

	// CapSellerSubscriptionRecover - Can manually recover a settled subscription payment
	// that has no corresponding seller_subscriptions row (webhook miss scenario).
	// Gates: POST /admin/seller-subscriptions/recover/:payment_id
	CapSellerSubscriptionRecover Capability = "seller.subscription.recover"
)

// ============================================================
// ORDER CLUSTER
// ============================================================

const (
	// CapOrderRead - Can view all orders (admin)
	CapOrderRead Capability = "order.read"
)

// ============================================================
// CONFIG CLUSTER
// ============================================================

const (
	// CapConfigView - Can view platform configuration
	CapConfigView Capability = "config.view"

	// CapConfigUpdateGeneral - Can update general platform configuration
	CapConfigUpdateGeneral Capability = "config.update.general"

	// CapConfigUpdateFinancial - Can update financial platform configuration
	// Financial configs include: for_sale_commission_percent, auction_commission_percent,
	// min_withdrawal, max_withdrawal, withdrawal_threshold.
	CapConfigUpdateFinancial Capability = "config.update.financial"
)

// ============================================================
// SUPPORT CLUSTER
// ============================================================

const (
	// CapSupportTicketRead - Can view all support tickets
	CapSupportTicketRead Capability = "support.ticket.read"

	// CapSupportTicketRespond - Can respond to support tickets
	CapSupportTicketRespond Capability = "support.ticket.respond"

	// CapSupportTicketClaim - Can claim support tickets
	CapSupportTicketClaim Capability = "support.ticket.claim"

	// CapSupportTicketResolve - Can resolve support tickets
	CapSupportTicketResolve Capability = "support.ticket.resolve"

	// CapSupportAdminAssign - Can reassign tickets to admins
	CapSupportAdminAssign Capability = "support.admin.assign"

	// CapSupportAdminRead - Can view support admin statistics and lists
	CapSupportAdminRead Capability = "support.admin.read"

	// CapSupportTicketEscalate - Can escalate support tickets to disputes
	CapSupportTicketEscalate Capability = "support.ticket.escalate"
)

// ============================================================
// VALIDATION
// ============================================================

// IsValid checks if a string is a valid capability constant.
// This is used to ensure only known capabilities are granted.
func IsValid(cap string) bool {
	switch Capability(cap) {
	case CapFinanceWithdrawRead,
		CapFinanceWithdrawReview,
		CapFinanceDisputeResolve,
		CapFinanceRefundGatewayInitiate,
		CapFinancePaymentMethodView,
		CapFinancePaymentMethodManage,
		CapGovernanceDashboardView,
		CapGovernanceAlertRead,
		CapGovernanceAlertResolve,
		CapGovernanceUserRead,
		CapGovernanceUserSuspend,
		CapGovernanceUserBan,
		CapGovernanceUserActivate,
		CapGovernanceUserUnban,
		CapGovernanceRoleAssign,
		CapGovernanceCapabilityAssign,
		CapGovernanceAuditRead,
		CapModerationCaseRead,
		CapModerationContentView,
		CapModerationContentRemove,
		CapModerationCaseResolve,
		CapModerationEvidenceRead,
		CapModerationAppealRead,
		CapModerationAppealReview,
		CapPromotionExternalProductReview,
		CapPromotionPackageManage,
		CapPromotionCampaignView,
		CapPromotionCampaignStop,
		CapSellerVerificationReview,
		CapSellerSubscriptionRecover,
		CapOrderRead,
		CapConfigView,
		CapConfigUpdateGeneral,
		CapConfigUpdateFinancial,
		CapSupportTicketRead,
		CapSupportTicketRespond,
		CapSupportTicketClaim,
		CapSupportTicketResolve,
		CapSupportAdminAssign,
		CapSupportAdminRead,
		CapSupportTicketEscalate,
		CapGovernanceAuctionCancel:
		return true
	default:
		return false
	}
}

// AllCapabilities returns a list of all defined capabilities.
// Useful for validation and testing.
func AllCapabilities() []Capability {
	return []Capability{
		CapFinanceWithdrawRead,
		CapFinanceWithdrawReview,
		CapFinanceDisputeResolve,
		CapFinanceRefundGatewayInitiate,
		CapFinancePaymentMethodView,
		CapFinancePaymentMethodManage,
		CapGovernanceDashboardView,
		CapGovernanceAlertRead,
		CapGovernanceAlertResolve,
		CapGovernanceUserRead,
		CapGovernanceUserSuspend,
		CapGovernanceUserBan,
		CapGovernanceUserActivate,
		CapGovernanceUserUnban,
		CapGovernanceRoleAssign,
		CapGovernanceCapabilityAssign,
		CapGovernanceAuditRead,
		CapModerationCaseRead,
		CapModerationContentView,
		CapModerationContentRemove,
		CapModerationCaseResolve,
		CapModerationEvidenceRead,
		CapModerationAppealRead,
		CapModerationAppealReview,
		CapPromotionExternalProductReview,
		CapPromotionPackageManage,
		CapPromotionCampaignView,
		CapPromotionCampaignStop,
		CapSellerVerificationReview,
		CapSellerSubscriptionRecover,
		CapOrderRead,
		CapConfigView,
		CapConfigUpdateGeneral,
		CapConfigUpdateFinancial,
		CapSupportTicketRead,
		CapSupportTicketRespond,
		CapSupportTicketClaim,
		CapSupportTicketResolve,
		CapSupportAdminAssign,
		CapSupportAdminRead,
		CapSupportTicketEscalate,
		CapGovernanceAuctionCancel,
	}
}
