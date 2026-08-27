package constants

const (
	// SubscriptionDurationDays is the fixed subscription duration in days.
	// This is a system-controlled constant and cannot be overridden by users or admins.
	// All subscriptions must use this value - no exceptions.
	SubscriptionDurationDays = 365

	// RenewalWindowDays is the number of days before expiry when renewal is allowed.
	// Renewal is only allowed within 30 days of subscription expiry.
	RenewalWindowDays = 30
)
