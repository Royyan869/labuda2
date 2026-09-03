package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
)

// Decision Contract from Backend
//
// Backend is the SINGLE SOURCE OF TRUTH for all business decisions.
// Frontend MUST NOT derive state or allowed actions from other fields.
//
// Use decision.allowed_actions to determine which UI actions to show.
// Use decision.state for authoritative business state.
// Use decision.display for UI rendering hints (labels, badges, warnings).

// AllowedAction represents all possible actions that can be taken on an order.
// This enum is the single source of truth for valid actions.
type AllowedAction string

const (
	// ActionMarkShipped - Seller can mark order as shipped
	ActionMarkShipped AllowedAction = "mark_shipped"
	// ActionComplete - Buyer can complete/confirm the order (B4A: single-click from shipped)
	ActionComplete AllowedAction = "complete"
	// ActionRequestRefund - Buyer can request a refund
	ActionRequestRefund AllowedAction = "request_refund"
	// ActionOpenDispute - Either party can open a dispute
	ActionOpenDispute AllowedAction = "open_dispute"
	// ActionCancel - Buyer can cancel the order
	ActionCancel AllowedAction = "cancel"
	// ActionUpdateTracking - Seller can update tracking number
	ActionUpdateTracking AllowedAction = "update_tracking"
	// ActionExtendConfirmation - Buyer can extend confirmation deadline
	ActionExtendConfirmation AllowedAction = "extend_confirmation"
	// ActionProvideEvidence - User can upload evidence for dispute
	ActionProvideEvidence AllowedAction = "provide_evidence"
	// ActionAcceptRefund - Seller can accept refund request
	ActionAcceptRefund AllowedAction = "accept_refund"
	// ActionRejectRefund - Seller can reject refund request
	ActionRejectRefund AllowedAction = "reject_refund"
	// ActionPay - Buyer can resume or continue payment
	ActionPay AllowedAction = "pay"
	// ActionNone - No action available
	ActionNone AllowedAction = "none"
)

// String returns the string representation of the action
func (a AllowedAction) String() string {
	return string(a)
}

// =============================================================================
// MULTI-ACTION COMMAND ENGINE V3 - Production Hardened
// =============================================================================
// Backend defines ALL behavior. Frontend renders from action definitions.
//
// Action structure:
// - type: enum of the action
// - label_key: localization key for UI
// - enabled: whether action is currently available
// - blocked: why action is disabled (with resolution)
// - endpoint: API endpoint to call
// - method: HTTP method
// - requires_idempotency: whether action requires idempotency key (always true for mutations)
// - financial: whether this action affects money (requires ledger validation)
// - input_schema: structured input definition with validation
//
// Decision structure:
// - state: authoritative business state
// - version: decision version (for optimistic concurrency)
// - decision_version: increments when decision structure changes
// - primary_action: main CTA
// - secondary_actions: alternative actions
// - display: UI hints (badges, warnings)
//
// Action Request structure:
// - idempotency_key: required for all mutation actions
// - decision_version: client's decision version for optimistic concurrency
// - input: validated against input_schema
//
// Action Result structure:
// - success: boolean
// - next_state: new order state after action
// - refresh_targets: what UI elements to refresh
// - error: structured error with type, code, message_key
// =============================================================================

// ActionBlockedReason explains why an action is not available.
type ActionBlockedReason struct {
	Action           AllowedAction  `json:"action"`                      // The blocked action
	MessageKey       string         `json:"message_key"`                 // Localization key for error message
	Reason           string         `json:"reason,omitempty"`            // Human-readable reason (fallback)
	Code             string         `json:"code"`                        // Machine-readable reason code
	ResolutionAction *AllowedAction `json:"resolution_action,omitempty"` // Action to resolve the block
	ResolutionLabel  *string        `json:"resolution_label,omitempty"`  // Label for resolution button
}

// InputFieldType defines the type of input field.
type InputFieldType string

const (
	InputTypeText     InputFieldType = "text"
	InputTypeTextarea InputFieldType = "textarea"
	InputTypeFile     InputFieldType = "file"
	InputTypeNumber   InputFieldType = "number"
	InputTypeSelect   InputFieldType = "select"
	InputTypeDate     InputFieldType = "date"
)

// InputFieldValidation defines validation rules for an input field.
type InputFieldValidation struct {
	Required  *bool    `json:"required,omitempty"`
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
	Min       *int64   `json:"min,omitempty"`
	Max       *int64   `json:"max,omitempty"`
	Pattern   *string  `json:"pattern,omitempty"`
	Options   []string `json:"options,omitempty"` // For select type
}

// InputFieldDefinition defines a single input field in the schema.
type InputFieldDefinition struct {
	Key         string                `json:"key"`                   // Field key for request body
	LabelKey    string                `json:"label_key"`             // Localization key for label
	Type        InputFieldType        `json:"type"`                  // Field type
	Placeholder *string               `json:"placeholder,omitempty"` // Placeholder text
	Validation  *InputFieldValidation `json:"validation,omitempty"`  // Validation rules
	Default     interface{}           `json:"default,omitempty"`     // Default value
}

// InputSchema defines the structured input for an action.
type InputSchema struct {
	Fields []InputFieldDefinition `json:"fields"` // Array of field definitions
}

// RefreshTarget defines what UI elements to refresh after action.
type RefreshTarget string

const (
	RefreshTargetOrder        RefreshTarget = "order"
	RefreshTargetDecision     RefreshTarget = "decision"
	RefreshTargetPayments     RefreshTarget = "payments"
	RefreshTargetMessages     RefreshTarget = "messages"
	RefreshTargetTimeline     RefreshTarget = "timeline"
	RefreshTargetParticipants RefreshTarget = "participants"
)

// Action represents a single executable action on an order.
// Frontend renders buttons directly from this structure - no business logic in UI.
type Action struct {
	Type                AllowedAction        `json:"type"`                   // Action type enum
	LabelKey            string               `json:"label_key"`              // Localization key for button label
	Enabled             bool                 `json:"enabled"`                // Whether the action is currently enabled
	Blocked             *ActionBlockedReason `json:"blocked,omitempty"`      // Why blocked (if disabled)
	Endpoint            string               `json:"endpoint"`               // API endpoint to call
	Method              string               `json:"method"`                 // HTTP method (POST, PATCH, etc.)
	RequiresIdempotency bool                 `json:"requires_idempotency"`   // Whether action requires idempotency key
	Financial           bool                 `json:"financial"`              // Whether action affects money (ledger validation)
	InputSchema         *InputSchema         `json:"input_schema,omitempty"` // Structured input definition
	// DEPRECATED: Use InputSchema instead
	RequiresInput bool    `json:"requires_input,omitempty"` // Whether action needs user input
	InputHint     *string `json:"input_hint,omitempty"`     // Placeholder/input hint if requires_input
	InputType     *string `json:"input_type,omitempty"`     // Input type: "text", "textarea", "file", etc.
}

// Decision represents the decision contract for an order (V3 - Production Hardened).
type Decision struct {
	State            string        `json:"state"`                       // Authoritative business state (order status)
	Version          string        `json:"version"`                     // Decision contract version
	DecisionVersion  int64         `json:"decision_version"`            // Optimistic concurrency counter
	PrimaryAction    *Action       `json:"primary_action,omitempty"`    // Main call-to-action
	SecondaryActions []Action      `json:"secondary_actions,omitempty"` // Alternative actions
	Display          *DisplayHints `json:"display,omitempty"`           // UI rendering hints
}

// =============================================================================
// ACTION REQUEST AND RESULT - V3 Production Hardened
// ============================================================================

// ErrorType defines the category of error.
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "validation" // Input validation failed
	ErrorTypeBusiness   ErrorType = "business"   // Business rule violation
	ErrorTypeSystem     ErrorType = "system"     // Internal system error
	ErrorTypeAuth       ErrorType = "auth"       // Authorization/permission error
)

// ActionError represents a structured error response.
type ActionError struct {
	Type       ErrorType `json:"type"`        // Error category
	Code       string    `json:"code"`        // Machine-readable error code
	MessageKey string    `json:"message_key"` // Localization key for message
	Message    string    `json:"message"`     // Human-readable fallback message
	Details    *struct {
		Field   *string                `json:"field,omitempty"`   // Field that caused the error (for validation)
		Reason  *string                `json:"reason,omitempty"`  // Detailed reason
		Context map[string]interface{} `json:"context,omitempty"` // Additional context
	} `json:"details,omitempty"`
}

// ActionResultMetadata contains metadata about the result of an action.
type ActionResultMetadata struct {
	Success        bool            `json:"success"`                   // Whether the action succeeded
	NextState      *string         `json:"next_state,omitempty"`      // New order state after action
	RefreshTargets []RefreshTarget `json:"refresh_targets,omitempty"` // UI elements to refresh
	Error          *ActionError    `json:"error,omitempty"`           // Error if success=false
	Data           interface{}     `json:"data,omitempty"`            // Additional data
}

// ActionRequest represents a client action execution request.
type ActionRequest struct {
	IdempotencyKey  string                 `json:"idempotency_key"`  // Required for all mutations
	DecisionVersion int64                  `json:"decision_version"` // Client's decision version for optimistic concurrency
	Input           map[string]interface{} `json:"input,omitempty"`  // Validated against input_schema
}

// NextAction represents the next recommended action with full context.
type NextAction struct {
	Type     AllowedAction        `json:"type"`              // Action type enum
	LabelKey string               `json:"label_key"`         // Localization key for the button label
	Enabled  bool                 `json:"enabled"`           // Whether the action is currently enabled
	Blocked  *ActionBlockedReason `json:"blocked,omitempty"` // Why blocked (if disabled)
}

// DisplayHints contains UI hints for rendering (NON-AUTHORITATIVE).
//
// These are UI hints ONLY. Frontend MUST NOT derive state or
// allowed_actions from these hints. Always use decision.state and
// decision.allowed_actions for logic.
type DisplayHints struct {
	Badge                *string `json:"badge,omitempty"`                  // Warning/info badge text
	BadgeVariant         *string `json:"badge_variant,omitempty"`          // Badge style (info, warning, error, success)
	Warning              *string `json:"warning,omitempty"`                // Warning message
	Info                 *string `json:"info,omitempty"`                   // Info message
	TimeRemainingSeconds *int    `json:"time_remaining_seconds,omitempty"` // Time remaining for action

	// Next Action - Full context object (replaces simple string fields)
	NextAction *NextAction `json:"next_action,omitempty"` // Complete next action with type, label, enabled status
}

// OrderDetailResponse represents the response for GET /api/v1/orders/{id}.
// It includes the order entity with decision contract.
//
// ARCHITECTURAL NOTES:
// - EscrowAmount is the canonical buyer base from the pricing-token snapshot
//   ((P−D)+S); commission C is seller/platform-side and is NOT buyer-funded cash.
// - RefundAmount comes from Ledger service, not stored in Order
// - Discount information comes from Ledger service, not stored in Order
// - Coins discount amount comes from Ledger, only CoinsUsed is stored for display
type OrderDetailResponse struct {
	// Order fields (flattened from Order entity for response)
	ID           uuid.UUID `json:"id"`
	OrderNumber  *string   `json:"order_number,omitempty"`
	BuyerID      uuid.UUID `json:"buyer_id"`
	SellerID     uuid.UUID `json:"seller_id"`
	BuyerName    string    `json:"buyer_name"`
	BuyerAvatar  *string   `json:"buyer_avatar,omitempty"`
	SellerAvatar *string   `json:"seller_avatar,omitempty"`

	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE (additive).
	// These are populated alongside the legacy buyer_name field
	// fields with strict source separation (NEVER COALESCE):
	//   - buyer_username    ← user_profiles.username
	//   - seller_username   ← user_profiles.username  (NEVER store_name)
	//   - seller_farm_name  ← seller_profiles.store_name  (NEVER username)
	//   - seller_avatar_url ← user_profiles.avatar_url
	BuyerUsername   string `json:"buyer_username"`
	SellerUsername  string `json:"seller_username"`
	SellerFarmName  string `json:"seller_farm_name"`
	SellerAvatarURL string `json:"seller_avatar_url"`

	// Source
	SourceType    string     `json:"source_type"`
	SourceID      uuid.UUID  `json:"source_id"`
	NegotiationID *uuid.UUID `json:"negotiation_id,omitempty"`

	// Pricing Snapshot (immutable at order creation)
	Quantity           int   `json:"quantity"`
	UnitPrice          int64 `json:"unit_price"`
	Subtotal           int64 `json:"subtotal"`
	ShippingTotal      int64 `json:"shipping_total"`
	CommissionPercent  int64 `json:"commission_percent"`
	CommissionAmount   int64 `json:"commission_amount"`
	ServiceFeeAmount   int64 `json:"service_fee_amount"`
	TotalPayableAmount int64 `json:"total_payable_amount"`

	// CoinsUsed for display only (discount amount handled by Ledger)
	CoinsUsed int64 `json:"coins_used,omitempty"`

	// Shipping option snapshot
	ShippingSetupID       uuid.UUID `json:"shipping_option_id"`
	ShippingSetupName     string    `json:"shipping_option_name"`
	ShippingTransportType string `json:"shipping_transport_type"`

	// Shipping Readiness Snapshot (for overdue calculation)
	PreparationTimeSnapshot *string `json:"preparation_time_snapshot,omitempty"`
	PreparationNoteSnapshot *string `json:"preparation_note_snapshot,omitempty"`
	ReadyToShipBy           *int64  `json:"ready_to_ship_by,omitempty"`

	// Overdue Display Layer (computed, not persisted)
	OverdueTier *string `json:"overdue_tier,omitempty"` // none, overdue, severely_overdue, critical_overdue
	OverdueDays *int    `json:"overdue_days,omitempty"` // Days past ready_to_ship_by (0 if not overdue)
	IsOverdue   *bool   `json:"is_overdue,omitempty"`   // Convenience boolean

	// Shipping Proof (set when order is marked as shipped)
	ProofType          *string `json:"proof_type,omitempty"`           // "tracking" | "phone" | "manual"
	TrackingNumber     *string `json:"tracking_number,omitempty"`      // tracking number or phone
	ShippingProofMedia *string `json:"shipping_proof_media,omitempty"` // URL to proof image/document
	ShippingNote       *string `json:"shipping_note,omitempty"`        // Optional shipping note

	// Status
	Status        string `json:"status"`
	EscrowStatus  string `json:"escrow_status"` // CACHED from Wallet - may be stale, critical decisions should use Wallet
	AutoReleaseAt *int64 `json:"auto_release_at,omitempty"`

	// Confirmation extension
	ConfirmationExtensionUsed bool   `json:"confirmation_extension_used"`
	ConfirmationExtendedAt    *int64 `json:"confirmation_extended_at,omitempty"`

	// Buyer notes (optional, set by buyer at checkout)
	BuyerNotes *string `json:"buyer_notes,omitempty"`

	// Payment reference
	PaymentID *uuid.UUID `json:"payment_id,omitempty"`
	// PaymentStatus is the status of the active/latest payment for this order.
	// Source: payments table, prioritised settlement > capture > pending > others.
	// Nil when no payment record exists (e.g. order still awaiting initial payment intent).
	PaymentStatus *string `json:"payment_status,omitempty"`

	// Timestamps
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`

	// Decision Contract - Backend is SINGLE SOURCE OF TRUTH for business decisions
	Decision *Decision `json:"decision,omitempty"`

	// Active refund visibility for seller/buyer follow-up actions.
	// Source: refund service lookup in GET /orders/:id handler.
	HasActiveRefund bool                 `json:"has_active_refund"`
	ActiveRefund    *ActiveRefundSummary `json:"active_refund"`

	// Shipping Source + Origin (I1-C1: now persisted)
	ShippingSource *string                        `json:"shipping_source,omitempty"` // "for_sale" or "shipping_quote"
	ShippingOrigin *addressentity.AddressSnapshot `json:"shipping_origin,omitempty"` // Seller farm/warehouse address snapshot

	// Nested objects (optional, populated when available)
	Items               []*OrderItemDTO                `json:"items,omitempty"`
	ShippingDestination *addressentity.AddressSnapshot `json:"shipping_destination,omitempty"`
}

// ActiveRefundSummary is the seller/buyer-safe refund payload surfaced in
// GET /orders/:id for active refund visibility.
type ActiveRefundSummary struct {
	ID              uuid.UUID `json:"id"`
	OrderID         uuid.UUID `json:"order_id"`
	BuyerID         uuid.UUID `json:"buyer_id"`
	SellerID        uuid.UUID `json:"seller_id"`
	Status          string    `json:"status"`
	Reason          string    `json:"reason"`
	Description     *string   `json:"description,omitempty"`
	RequestedAmount int64     `json:"requested_amount"`
	SellerNotes     *string   `json:"seller_notes,omitempty"`
	EvidenceURLs    []string  `json:"evidence_urls,omitempty"`
	CreatedAt       int64     `json:"created_at"`
	UpdatedAt       int64     `json:"updated_at"`
	AdminNotes      *string   `json:"admin_notes,omitempty"`
	ResolvedAt      *int64    `json:"resolved_at,omitempty"`
	GatewayStatus   *string   `json:"gateway_status,omitempty"`
}

// OrderItemDTO represents an order item with snapshot data from order creation time.
// It reflects the OrderItem entity structure for API responses.
type OrderItemDTO struct {
	ID                uuid.UUID `json:"id"`
	OrderID           uuid.UUID `json:"order_id"`
	ProductID         uuid.UUID `json:"product_id"`
	Name              string    `json:"name"`                // Item name snapshot
	UnitPriceSnapshot int64     `json:"unit_price_snapshot"` // Price per unit at order time
	Quantity          int       `json:"quantity"`
	Subtotal          int64     `json:"subtotal"` // Computed: UnitPriceSnapshot * Quantity
}

// =============================================================================
// DECISION V2 BUILDERS - Multi-Action Command Engine
// =============================================================================

const (
	// DecisionContractVersion is the current version of the decision contract format
	DecisionContractVersion = "3.0.0"
)

// NewDecisionV2 creates a new Decision (V2) with state and actions.
// DEPRECATED: Use NewDecisionV3 instead.
func NewDecisionV2(state string) *Decision {
	return &Decision{
		State:            state,
		Version:          DecisionContractVersion,
		DecisionVersion:  0,
		PrimaryAction:    nil,
		SecondaryActions: []Action{},
	}
}

// NewDecisionV3 creates a new Decision (V3) with state and actions.
func NewDecisionV3(state string, decisionVersion int64) *Decision {
	return &Decision{
		State:            state,
		Version:          DecisionContractVersion,
		DecisionVersion:  decisionVersion,
		PrimaryAction:    nil,
		SecondaryActions: []Action{},
	}
}

// WithDecisionVersion sets the decision version for optimistic concurrency.
func (d *Decision) WithDecisionVersion(version int64) *Decision {
	d.DecisionVersion = version
	return d
}

// WithPrimaryAction sets the primary action for the decision.
func (d *Decision) WithPrimaryAction(action *Action) *Decision {
	d.PrimaryAction = action
	return d
}

// WithSecondaryActions adds secondary actions to the decision.
func (d *Decision) WithSecondaryActions(actions ...Action) *Decision {
	d.SecondaryActions = append(d.SecondaryActions, actions...)
	return d
}

// WithDisplayV2 adds display hints to the decision (V2).
func (d *Decision) WithDisplayV2(display *DisplayHints) *Decision {
	d.Display = display
	return d
}

// NewAction creates a new Action with the given parameters.
func NewAction(actionType AllowedAction, labelKey string, endpoint string, method string) *Action {
	return &Action{
		Type:                actionType,
		LabelKey:            labelKey,
		Enabled:             true,
		Endpoint:            endpoint,
		Method:              method,
		RequiresIdempotency: true, // All mutations require idempotency by default
		Financial:           false,
		RequiresInput:       false,
	}
}

// WithIdempotency sets whether the action requires an idempotency key.
func (a *Action) WithIdempotency(requires bool) *Action {
	a.RequiresIdempotency = requires
	return a
}

// WithFinancial marks the action as affecting money (requires ledger validation).
func (a *Action) WithFinancial() *Action {
	a.Financial = true
	return a
}

// WithInputSchema sets the structured input schema for the action.
func (a *Action) WithInputSchema(schema *InputSchema) *Action {
	a.InputSchema = schema
	return a
}

// WithInputField adds a single input field to the action's input schema.
func (a *Action) WithInputField(key, labelKey, fieldType string, required bool) *Action {
	if a.InputSchema == nil {
		a.InputSchema = &InputSchema{Fields: []InputFieldDefinition{}}
	}
	req := required
	a.InputSchema.Fields = append(a.InputSchema.Fields, InputFieldDefinition{
		Key:        key,
		LabelKey:   labelKey,
		Type:       InputFieldType(fieldType),
		Validation: &InputFieldValidation{Required: &req},
	})
	return a
}

// WithInput sets input requirements for the action.
// DEPRECATED: Use WithInputSchema or WithInputField instead.
func (a *Action) WithInput(inputHint string, inputType string) *Action {
	a.RequiresInput = true
	a.InputHint = &inputHint
	a.InputType = &inputType
	return a
}

// WithBlocked sets the action as blocked with a reason.
func (a *Action) WithBlocked(messageKey string, code string, resolution *AllowedAction, resolutionLabel *string) *Action {
	a.Enabled = false
	a.Blocked = &ActionBlockedReason{
		Action:           a.Type,
		MessageKey:       messageKey,
		Code:             code,
		ResolutionAction: resolution,
		ResolutionLabel:  resolutionLabel,
	}
	return a
}

// NewDisplayHints creates a new DisplayHints.
func NewDisplayHints() *DisplayHints {
	return &DisplayHints{}
}

// WithBadge sets the badge and variant.
func (dh *DisplayHints) WithBadge(badge, variant string) *DisplayHints {
	dh.Badge = &badge
	dh.BadgeVariant = &variant
	return dh
}

// WithWarning sets the warning message.
func (dh *DisplayHints) WithWarning(warning string) *DisplayHints {
	dh.Warning = &warning
	return dh
}

// WithInfo sets the info message.
func (dh *DisplayHints) WithInfo(info string) *DisplayHints {
	dh.Info = &info
	return dh
}

// WithTimeRemaining sets the time remaining in seconds.
func (dh *DisplayHints) WithTimeRemaining(seconds int) *DisplayHints {
	dh.TimeRemainingSeconds = &seconds
	return dh
}

// WithNextAction sets the next action with full context (type, label, enabled, blocked reason).
func (dh *DisplayHints) WithNextAction(actionType AllowedAction, labelKey string, enabled bool) *DisplayHints {
	dh.NextAction = &NextAction{
		Type:     actionType,
		LabelKey: labelKey,
		Enabled:  enabled,
	}
	return dh
}

// WithNextActionBlocked sets the next action as blocked with a reason.
func (dh *DisplayHints) WithNextActionBlocked(actionType AllowedAction, labelKey string, reason string, code string) *DisplayHints {
	dh.NextAction = &NextAction{
		Type:     actionType,
		LabelKey: labelKey,
		Enabled:  false,
		Blocked: &ActionBlockedReason{
			Action: actionType,
			Reason: reason,
			Code:   code,
		},
	}
	return dh
}

// OrderToDetailResponse converts an Order entity to OrderDetailResponse with decision contract.
// callerID is used to determine the user's role and compute allowed actions.
// buyerName, buyerAvatar, sellerAvatar are optional participant info (LEGACY).
// orderItems is the list of order items for this order.
//
// Phase 5 Stage 1 — additive seller/farm convergence. Callers should
// use OrderToDetailResponseWithIdentity to populate the new explicit
// fields (buyer_username, seller_username, seller_farm_name,
// seller_avatar_url). This legacy entry point keeps existing callers
// compiling; new callers MUST use the With-variant.
func OrderToDetailResponse(
	order *entity.Order,
	callerID uuid.UUID,
	buyerName, buyerAvatar, sellerAvatar string,
	orderItems []*entity.OrderItem,
) *OrderDetailResponse {
	return OrderToDetailResponseWithIdentity(
		order, callerID,
		buyerName, buyerAvatar, sellerAvatar,
		// New explicit identity fields default to "" — preserves shape.
		"", "", "", "",
		orderItems,
		nil,
		false, nil, // No refund state available from legacy caller
		nil, // No payment status available from legacy caller
		nil, // No payment ID available from legacy caller
		nil, // No payment expiry available from legacy caller
	)
}

// OrderToDetailResponseWithIdentity converts an Order entity to
// OrderDetailResponse with the additive Phase 5 Stage 1 seller/farm
// convergence identity fields populated.
//
// Strict source separation (NEVER COALESCE):
//   - buyerUsername    ← user_profiles.username
//   - sellerUsername   ← user_profiles.username   (NEVER store_name)
//   - sellerFarmName   ← seller_profiles.store_name (NEVER username)
//   - sellerAvatarURL  ← user_profiles.avatar_url
//
// Legacy buyerName/buyerAvatar/sellerAvatar are passed
// through unchanged for backward compatibility.
func OrderToDetailResponseWithIdentity(
	order *entity.Order,
	callerID uuid.UUID,
	buyerName, buyerAvatar, sellerAvatar string,
	buyerUsername, sellerUsername, sellerFarmName, sellerAvatarURL string,
	orderItems []*entity.OrderItem,
	activeRefund *ActiveRefundSummary,
	hasActiveRefund bool,
	activeRefundStatus *string, // nil if no active refund; e.g. "pending_seller_review", "seller_rejected"
	paymentStatus *string, // nil when no payment record exists for the order
	paymentID *uuid.UUID, // nil when no payment record exists for the order
	paymentExpiredAt *time.Time, // nil when no payment record exists for the order
) *OrderDetailResponse {
	// Determine caller's role
	role := "buyer"
	if order.SellerID == callerID {
		role = "seller"
	}

	// Build V2 decision with actions
	decision := buildDecisionV2ForOrder(order, role, hasActiveRefund, activeRefundStatus, paymentStatus, paymentExpiredAt)

	// Convert timestamps
	var autoReleaseAt *int64
	if order.AutoReleaseAt != nil {
		ts := order.AutoReleaseAt.Unix()
		autoReleaseAt = &ts
	}

	var confirmationExtendedAt *int64
	if order.ConfirmationExtendedAt != nil {
		ts := order.ConfirmationExtendedAt.Unix()
		confirmationExtendedAt = &ts
	}

	// Convert shipping destination snapshot
	var shippingDestination *addressentity.AddressSnapshot
	if order.ShippingDestination != nil {
		shippingDestination = order.ShippingDestination
	}

	// Convert avatar strings to pointers
	var buyerAvatarPtr, sellerAvatarPtr *string
	if buyerAvatar != "" {
		buyerAvatarPtr = &buyerAvatar
	}
	if sellerAvatar != "" {
		sellerAvatarPtr = &sellerAvatar
	}

	// Convert order items to DTOs
	var itemsDTO []*OrderItemDTO
	if len(orderItems) > 0 {
		itemsDTO = make([]*OrderItemDTO, len(orderItems))
		for i, item := range orderItems {
			itemsDTO[i] = &OrderItemDTO{
				ID:                item.ID,
				OrderID:           item.OrderID,
				ProductID:         item.ProductID,
				Name:              item.Name,
				UnitPriceSnapshot: item.UnitPriceSnapshot.Int64(),
				Quantity:          item.Quantity,
				Subtotal:          item.Subtotal().Int64(),
			}
		}
	}

	// Convert preparation time snapshot (string field, nullable if empty)
	var preparationTimeSnapshot *string
	if order.PreparationTimeSnapshot != "" && order.PreparationTimeSnapshot != "immediate" {
		preparationTimeSnapshot = &order.PreparationTimeSnapshot
	}

	// Convert ready_to_ship_by to timestamp
	var readyToShipBy *int64
	if order.ReadyToShipBy != nil {
		ts := order.ReadyToShipBy.Unix()
		readyToShipBy = &ts
	}

	// Calculate overdue info
	overdueInfo := order.CalculateOverdueInfo()
	var overdueTier *string
	var overdueDays *int
	var isOverdue *bool
	if overdueInfo.IsOverdue {
		tierStr := string(overdueInfo.Tier)
		overdueTier = &tierStr
		overdueDays = &overdueInfo.DaysOverdue
		isOverdueVal := true
		isOverdue = &isOverdueVal
	}

	return &OrderDetailResponse{
		ID:           order.ID,
		OrderNumber:  order.OrderNumber,
		BuyerID:      order.BuyerID,
		SellerID:     order.SellerID,
		BuyerName:    buyerName,
		BuyerAvatar:  buyerAvatarPtr,
		SellerAvatar: sellerAvatarPtr,
		// Phase 5 Stage 1 additive identity fields (strict separation).
		BuyerUsername:           buyerUsername,
		SellerUsername:          sellerUsername,
		SellerFarmName:          sellerFarmName,
		SellerAvatarURL:         sellerAvatarURL,
		SourceType:              string(order.SourceType),
		SourceID:                order.SourceID,
		NegotiationID:           order.NegotiationID,
		Quantity:                order.Quantity,
		UnitPrice:               order.UnitPrice.Int64(),
		Subtotal:                order.Subtotal.Int64(),
		ShippingTotal:           order.ShippingTotal.Int64(),
		CommissionPercent:       order.CommissionPercent,
		CommissionAmount:        order.CommissionAmount.Int64(),
		ServiceFeeAmount:        order.ServiceFeeAmount.Int64(),
		TotalPayableAmount:      order.TotalPayableAmount.Int64(),
		CoinsUsed:               order.CoinsUsed,
		ShippingSetupID:        getShippingSetupID(order.ShippingSetupID),
		ShippingSetupName:      order.ShippingSetupName,
		ShippingTransportType:   order.ShippingTransportType,
		PreparationTimeSnapshot: preparationTimeSnapshot,
		PreparationNoteSnapshot: order.PreparationNoteSnapshot,
		ReadyToShipBy:           readyToShipBy,
		OverdueTier:             overdueTier,
		OverdueDays:             overdueDays,
		IsOverdue:               isOverdue,
		// Shipping Proof fields
		ProofType:          order.ProofType,
		TrackingNumber:     order.TrackingNumber,
		ShippingProofMedia: order.ShippingProofMedia,
		ShippingNote:       order.ShippingNote,
		// Shipping Source + Origin
		ShippingSource: order.ShippingSource,
		ShippingOrigin: order.ShippingOrigin,
		// Status
		Status:                    string(order.Status),
		EscrowStatus:              string(order.EscrowStatus),
		AutoReleaseAt:             autoReleaseAt,
		ConfirmationExtensionUsed: order.ConfirmationExtensionUsed,
		ConfirmationExtendedAt:    confirmationExtendedAt,
		BuyerNotes:                nil, // Field not yet implemented in entity
		PaymentID:                 paymentID,
		PaymentStatus:             paymentStatus,
		CreatedAt:                 order.CreatedAt.Unix(),
		UpdatedAt:                 order.UpdatedAt.Unix(),
		Decision:                  decision,
		HasActiveRefund:           hasActiveRefund,
		ActiveRefund:              activeRefund,
		Items:                     itemsDTO,
		ShippingDestination:       shippingDestination,
	}
}

// selectPayActionLabelKey chooses the label_key for the executable "pay"
// action on a pending buyer order based on the currently selected payment
// row (if any). The action TYPE and ENDPOINT stay constant (pay,
// POST /api/v1/payments) - only the label communicates whether this is a
// fresh payment, a continuation of an active one, a status check on one
// already settling, or a retry after expiry.
//
// paymentStatus is the raw gateway status string from the payments table
// (nil when no payment row exists yet). paymentExpiredAt is that same
// payment row's expiry, used only to distinguish a still-active pending
// payment from one whose window has lapsed but hasn't been swept yet.
func selectPayActionLabelKey(paymentStatus *string, paymentExpiredAt *time.Time) string {
	if paymentStatus == nil {
		return "action.pay_now"
	}

	switch *paymentStatus {
	case "settlement", "capture":
		// Payment resource already shows success but the order hasn't
		// caught up yet (webhook/order-sync lag) - buyer should check
		// status, not pay again.
		return "action.payment_check_status"
	case "challenge":
		// Fraud-review hold - still resolving, not yet actionable either way.
		return "action.payment_check_status"
	case "pending":
		expired := paymentExpiredAt != nil && paymentExpiredAt.Before(time.Now())
		if expired {
			return "action.pay_again"
		}
		return "action.payment_continue"
	case "deny", "cancel", "expire":
		// Terminal negative outcome on the payment row while the order is
		// still pending - safest path is a fresh payment attempt.
		return "action.pay_again"
	default:
		return "action.pay_now"
	}
}

// buildDisplayHintsForOrder creates display hints for a single order.
// role is either "buyer" or "seller" - determines which user's perspective to use.
// paymentStatus/paymentExpiredAt are the buyer's currently selected payment
// row (nil when none exists yet) - see selectPayActionLabelKey.
func buildDisplayHintsForOrder(order *entity.Order, role string, paymentStatus *string, paymentExpiredAt *time.Time) *DisplayHints {
	hints := &DisplayHints{}

	now := time.Now()

	switch order.Status {
	case entity.StatusPending:
		badge := "Menunggu Pembayaran"
		variant := "warning"
		hints.WithBadge(badge, variant)
		hints.WithInfo("Selesaikan pembayaran sebelum waktu habis")
		// Next action: wait for payment
		if role == "seller" {
			hints.WithNextAction(ActionNone, "action.wait_payment", false)
		} else {
			hints.WithNextAction(ActionPay, selectPayActionLabelKey(paymentStatus, paymentExpiredAt), true)
		}

	case entity.StatusPaid:
		// Check for overdue status first
		overdueInfo := order.CalculateOverdueInfo()
		if overdueInfo.IsOverdue {
			// Use overdue badge and warning
			badge := order.GetOverdueBadgeLabel()
			variant := order.GetOverdueBadgeVariant()
			hints.WithBadge(badge, variant)
			hints.WithWarning(order.GetOverdueWarningMessage())

			// Add info message for buyer based on tier
			if role == "buyer" {
				switch overdueInfo.Tier {
				case entity.OverdueTier1:
					hints.WithInfo("Jika perlu, Anda dapat mengingatkan penjual")
				case entity.OverdueTier2:
					hints.WithInfo("Anda dapat menghubungi penjual atau support")
				case entity.OverdueTier3:
					hints.WithInfo("Disarankan menghubungi support bantuan")
				}
				hints.WithNextAction(ActionNone, "action.wait_shipping", false)
			} else {
				// Seller perspective
				hints.WithInfo("Pesanan ini melewati estimasi siap kirim")
				hints.WithNextAction(ActionMarkShipped, "action.mark_shipped", true)
			}
		} else {
			// Normal paid order (not overdue)
			badge := "Menunggu Pengiriman"
			variant := "info"
			hints.WithBadge(badge, variant)
			if role == "seller" {
				hints.WithNextAction(ActionMarkShipped, "action.mark_shipped", true)
			} else {
				hints.WithNextAction(ActionNone, "action.wait_shipping", false)
			}
		}

	case entity.StatusShipped:
		// B4A: SHIPPED is the buyer decision screen.
		// "Terima Barang" = final acceptance (Complete, financial action).
		// "Ada Masalah" = refund/dispute path.
		badge := "Dalam Pengiriman"
		variant := "success"
		hints.WithBadge(badge, variant)
		if role == "buyer" {
			hints.WithNextAction(ActionComplete, "action.confirm_receipt", true)
		} else {
			hints.WithNextAction(ActionNone, "action.wait_buyer_confirm", false)
		}

		// Add time remaining if auto_release_at is set
		if order.AutoReleaseAt != nil && order.AutoReleaseAt.After(now) {
			remaining := int(time.Until(*order.AutoReleaseAt).Seconds())
			hints.WithTimeRemaining(remaining)
			hints.WithInfo("Otomatis selesai dalam 5 hari")
		}

	case entity.StatusCompleted:
		badge := "Selesai"
		variant := "success"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(ActionNone, "action.order_completed", false)

	case entity.StatusCancelled:
		badge := "Dibatalkan"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(ActionNone, "action.order_cancelled", false)

	case entity.StatusExpired:
		badge := "Kadaluarsa"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(ActionNone, "action.order_expired", false)

	case entity.StatusCancelledTimeout:
		badge := "Dibatalkan (Timeout)"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithInfo("Penjual tidak mengirim dalam batas waktu")
		hints.WithNextAction(ActionNone, "action.order_cancelled_timeout", false)

	case entity.StatusPartiallyRefunded:
		badge := "Refund Sebagian"
		variant := "warning"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(ActionNone, "action.partially_refunded", false)

	case entity.StatusRefunded:
		badge := "Refund"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(ActionNone, "action.refunded", false)

	case entity.StatusDisputeOpen:
		badge := "Dispute Terbuka"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(ActionProvideEvidence, "action.provide_evidence", true)
	}

	return hints
}

// =============================================================================
// DECISION V2 BUILDER - Multi-Action Command Engine
// =============================================================================

// buildDecisionV2ForOrder creates a complete Decision (V2) with primary and secondary actions.
// Frontend will render buttons directly from the action definitions.
//
// hasActiveRefund: true if order has a non-terminal refund request.
// activeRefundStatus: the refund's current status string (nil if no active refund).
// Used to gate refund/dispute CTA coexistence:
//   - No active refund → show both refund + dispute CTAs
//   - Active refund (pending/approved) → hide both (wait for seller decision)
//   - Active refund (rejected) → hide refund CTA, show dispute CTA (escalation path)
//   - Active refund (escalated) → hide both (admin reviewing)
//
// paymentStatus/paymentExpiredAt are the buyer's currently selected payment
// row (nil when none exists yet) - see selectPayActionLabelKey.
func buildDecisionV2ForOrder(order *entity.Order, role string, hasActiveRefund bool, activeRefundStatus *string, paymentStatus *string, paymentExpiredAt *time.Time) *Decision {
	// Compute decision version from order state and timestamps
	// This version changes whenever order state changes, enabling optimistic concurrency
	decisionVersion := order.UpdatedAt.Unix()

	// Start with base decision
	decision := NewDecisionV3(string(order.Status), decisionVersion)

	// Build display hints for UI
	display := buildDisplayHintsForOrder(order, role, paymentStatus, paymentExpiredAt)
	decision.WithDisplayV2(display)

	// Build primary and secondary actions
	var primaryAction *Action
	var secondaryActions []Action

	switch order.Status {
	case entity.StatusPaid:
		if role == "seller" {
			// Seller: Mark Shipped (primary with input)
			primaryAction = NewAction(
				ActionMarkShipped,
				"action.mark_shipped",
				"/api/v1/orders/"+order.ID.String()+"/ship",
				"POST",
			).WithInput("tracking_number", "text")
		} else {
			// Buyer: Check if overdue to provide appropriate actions
			overdueInfo := order.CalculateOverdueInfo()
			if overdueInfo.IsOverdue {
				// Buyer has cancel action for Tier 2+ overdue orders (SLA enforcement)
				if order.IsEligibleForBuyerCancelDueToOverdue() {
					// Primary action: Cancel Order (SLA enforcement)
					primaryAction = NewAction(
						ActionCancel,
						"action.cancel_order_overdue",
						"/api/v1/orders/"+order.ID.String()+"/cancel",
						"POST",
					).WithInput("cancel_reason", "textarea")
				} else {
					primaryAction = nil
				}

				// Secondary action: Contact Seller (chat)
				contactSellerAction := NewAction(
					"contact_seller",
					"action.chat_seller",
					"/chat/direct/"+order.SellerID.String(),
					"POST",
				).WithIdempotency(false) // Chat doesn't require idempotency
				secondaryActions = append(secondaryActions, *contactSellerAction)

				// Secondary action: Contact Support (Tier 2+)
				if overdueInfo.Tier == entity.OverdueTier2 || overdueInfo.Tier == entity.OverdueTier3 {
					contactSupportAction := NewAction(
						"contact_support",
						"action.contact_support",
						"/support/tickets",
						"POST",
					).WithInput("message", "textarea").WithIdempotency(false)
					secondaryActions = append(secondaryActions, *contactSupportAction)
				}
			} else {
				// Buyer: Wait for shipping (no action)
				primaryAction = nil
			}
		}

	case entity.StatusShipped:
		if role == "buyer" {
			// B4A: "Terima Barang" = final acceptance (single click).
			// Calls Complete() directly → releases escrow to seller.
			// Marked as financial action — mobile must show confirmation dialog.
			primaryAction = NewAction(
				ActionComplete,
				"action.confirm_receipt",
				"/api/v1/orders/"+order.ID.String()+"/complete",
				"POST",
			).WithFinancial()

			// Secondary: Extend Confirmation (on day 5, if eligible)
			// BUSINESS RULE: Only show when within 24 hours of auto_release_at (day 5)
			// and escrow is still holding and not yet extended
			if order.EscrowStatus == entity.EscrowStatusHolding &&
				!order.ConfirmationExtensionUsed &&
				order.AutoReleaseAt != nil {
				// Show extend button within 24 hours of auto_release_at (day 5)
				hoursUntilRelease := time.Until(*order.AutoReleaseAt).Hours()
				if hoursUntilRelease > 0 && hoursUntilRelease <= 24 {
					extendAction := NewAction(
						ActionExtendConfirmation,
						"action.extend_confirmation",
						"/api/v1/orders/"+order.ID.String()+"/extend-confirmation",
						"POST",
					)
					secondaryActions = append(secondaryActions, *extendAction)
				}
			}

			// Secondary: Request Refund (if escrow holding and no active refund) - FINANCIAL ACTION
			if order.EscrowStatus == entity.EscrowStatusHolding && !hasActiveRefund {
				refundAction := NewAction(
					ActionRequestRefund,
					"action.request_refund",
					"/api/v1/orders/"+order.ID.String()+"/refund",
					"POST",
				).WithFinancial().WithInput("refund_reason", "textarea")
				secondaryActions = append(secondaryActions, *refundAction)
			}

			// Secondary: Open Dispute
			// Coexistence rule: if active refund exists, only show dispute when
			// refund is seller_rejected (escalation path). Otherwise hide dispute
			// (buyer should wait for seller decision or admin review).
			canOpenDispute := true
			if hasActiveRefund && activeRefundStatus != nil {
				switch *activeRefundStatus {
				case "seller_rejected":
					canOpenDispute = true // Escalation from rejected refund
				default:
					canOpenDispute = false // pending/approved/escalated → wait
				}
			}
			if order.EscrowStatus == entity.EscrowStatusHolding && canOpenDispute {
				disputeAction := NewAction(
					ActionOpenDispute,
					"action.open_dispute",
					"/api/v1/orders/"+order.ID.String()+"/dispute",
					"POST",
				).WithInput("dispute_reason", "textarea")
				secondaryActions = append(secondaryActions, *disputeAction)
			}
		} else if role == "seller" {
			// Seller: Update Tracking (secondary)
			updateTrackingAction := NewAction(
				ActionUpdateTracking,
				"action.update_tracking",
				"/api/v1/orders/"+order.ID.String()+"/tracking",
				"PATCH",
			).WithInput("tracking_number", "text")
			secondaryActions = append(secondaryActions, *updateTrackingAction)
		}

	case entity.StatusDisputeOpen:
		// Both parties: Provide Evidence (primary)
		primaryAction = NewAction(
			ActionProvideEvidence,
			"action.provide_evidence",
			"/api/v1/orders/"+order.ID.String()+"/evidence",
			"POST",
		).WithInput("evidence_files", "file")

	case entity.StatusPending:
		if role == "buyer" {
			// Buyer: Pay (primary) — always calls POST /api/v1/payments with
			// order_id; label_key varies by payment state so the CTA reflects
			// whether this is a fresh payment, a continuation, a status
			// check, or a retry after expiry (see selectPayActionLabelKey).
			primaryAction = NewAction(
				ActionPay,
				selectPayActionLabelKey(paymentStatus, paymentExpiredAt),
				"/api/v1/payments",
				"POST",
			).WithInputField("order_id", "label.order_id", "hidden", true)

			// Secondary: Cancel Order
			cancelAction := NewAction(
				ActionCancel,
				"action.cancel_order",
				"/api/v1/orders/"+order.ID.String()+"/cancel",
				"POST",
			).WithInput("cancel_reason", "textarea")
			secondaryActions = append(secondaryActions, *cancelAction)
		}

	case entity.StatusCompleted, entity.StatusCancelled, entity.StatusExpired, entity.StatusCancelledTimeout:
		// No actions available
		primaryAction = nil
	}

	// TODO: Handle refund review actions when refund service is integrated
	// Special case: Seller refund review actions
	// if hasActiveRefund && role == "seller" {
	// 	// Primary: Accept Refund
	// 	acceptAction := NewAction(...)
	//
	// 	// Secondary: Reject Refund
	// 	rejectAction := NewAction(...)
	//
	// 	primaryAction = acceptAction
	// 	secondaryActions = append([]Action{*rejectAction}, secondaryActions...)
	// }

	// Apply actions to decision
	if primaryAction != nil {
		decision.WithPrimaryAction(primaryAction)
	}
	if len(secondaryActions) > 0 {
		decision.WithSecondaryActions(secondaryActions...)
	}

	return decision
}

// Helper functions for role determination
func roleIsSeller(order *entity.Order) bool {
	// This would be determined by the caller context
	// For now, return false as placeholder
	return false
}

func roleIsBuyer(order *entity.Order) bool {
	// This would be determined by the caller context
	// For now, return false as placeholder
	return false
}

// getShippingSetupID safely dereferences a nullable UUID
func getShippingSetupID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}
