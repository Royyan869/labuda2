package entity

// ActionType represents the type of moderation action.
// Used by DomainAction for enforcement execution tracking.
type ActionType string

const (
	ActionTypeWarnSeller       ActionType = "warn_seller"
	ActionTypeReduceVisibility ActionType = "reduce_visibility"
	ActionTypeHideForSale      ActionType = "hide_for_sale"
	ActionTypeRemoveForSale    ActionType = "remove_for_sale"
	ActionTypePauseAuction     ActionType = "pause_auction"
	ActionTypeRemoveAuction    ActionType = "remove_auction"
	ActionTypeRestrictSeller   ActionType = "restrict_seller"
	ActionTypeSuspendForSales  ActionType = "suspend_for_sales"
	ActionTypeSuspendAccount   ActionType = "suspend_account"
	ActionTypeContentRemove    ActionType = "content_remove"
	ActionTypeCommentRemove    ActionType = "comment_remove"
)

// ExecutionStatus represents the execution status of a domain action.
type ExecutionStatus string

const (
	ExecutionStatusPending    ExecutionStatus = "pending"
	ExecutionStatusSucceeded  ExecutionStatus = "succeeded"
	ExecutionStatusFailed     ExecutionStatus = "failed"
	ExecutionStatusRolledBack ExecutionStatus = "rolled_back"
)


