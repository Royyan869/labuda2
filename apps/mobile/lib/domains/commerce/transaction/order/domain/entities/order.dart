import 'package:equatable/equatable.dart';

import 'order_item.dart';
import 'order_pricing.dart';
import 'refund_request.dart';
import 'order_status.dart';
import 'order_source.dart';
import 'shipping_info.dart';
import 'package:labuda/core/common/types/payment_types.dart';
import 'package:labuda/core/common/types/preparation_time.dart';

// =============================================================================
// DECISION CONTRACT V2 - Backend is SINGLE SOURCE OF TRUTH
// =============================================================================
//
// ACTION DEFINITION: Full action metadata from backend
// - type: Action type enum
// - label_key: Localization key for UI
// - enabled: Whether action is currently available
// - blocked: Why action is disabled (with resolution)
// - endpoint: API endpoint to call
// - method: HTTP method (POST, PATCH, etc.)
// - requires_idempotency: Whether action requires idempotency key
// - financial: Whether action affects money (ledger validation)
// - input_schema: Structured input definition with validation
//
// DECISION DEFINITION:
// - state: Authoritative business state (order status)
// - version: Decision contract version
// - decision_version: Optimistic concurrency counter
// - primary_action: Main call-to-action (Action object)
// - secondary_actions: Alternative actions (array of Action)
// - display: UI hints (badges, warnings)
//
// Frontend MUST:
// 1. Loop through primary_action + secondary_actions
// 2. Render buttons based on action metadata
// 3. Execute actions using endpoint + method from backend
// 4. NEVER derive state or actions from other fields
// =============================================================================

/// Action Blocked Reason - explains why an action is not available
class ActionBlockedReason {
  final String action;
  final String messageKey;
  final String? reason;
  final String code;
  final String? resolutionAction;
  final String? resolutionLabel;

  const ActionBlockedReason({
    required this.action,
    required this.messageKey,
    this.reason,
    required this.code,
    this.resolutionAction,
    this.resolutionLabel,
  });

  factory ActionBlockedReason.fromJson(Map<String, dynamic> json) {
    return ActionBlockedReason(
      action: json['action'] as String? ?? '',
      messageKey: json['message_key'] as String? ?? '',
      reason: json['reason'] as String?,
      code: json['code'] as String? ?? '',
      resolutionAction: json['resolution_action'] as String?,
      resolutionLabel: json['resolution_label'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'action': action,
    'message_key': messageKey,
    if (reason != null) 'reason': reason,
    'code': code,
    if (resolutionAction != null) 'resolution_action': resolutionAction,
    if (resolutionLabel != null) 'resolution_label': resolutionLabel,
  };
}

/// Input Field Validation - validation rules for an input field
class InputFieldValidation {
  final bool? required;
  final int? minLength;
  final int? maxLength;
  final int? min;
  final int? max;
  final String? pattern;
  final List<String>? options;

  const InputFieldValidation({
    this.required,
    this.minLength,
    this.maxLength,
    this.min,
    this.max,
    this.pattern,
    this.options,
  });

  factory InputFieldValidation.fromJson(Map<String, dynamic> json) {
    return InputFieldValidation(
      required: json['required'] as bool?,
      minLength: json['min_length'] as int?,
      maxLength: json['max_length'] as int?,
      min: json['min'] as int?,
      max: json['max'] as int?,
      pattern: json['pattern'] as String?,
      options: (json['options'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    if (required != null) 'required': required,
    if (minLength != null) 'min_length': minLength,
    if (maxLength != null) 'max_length': maxLength,
    if (min != null) 'min': min,
    if (max != null) 'max': max,
    if (pattern != null) 'pattern': pattern,
    if (options != null) 'options': options,
  };
}

/// Input Field Definition - defines a single input field in the schema
class InputFieldDefinition {
  final String key;
  final String labelKey;
  final String type;
  final String? placeholder;
  final InputFieldValidation? validation;
  final dynamic defaultValue;

  const InputFieldDefinition({
    required this.key,
    required this.labelKey,
    required this.type,
    this.placeholder,
    this.validation,
    this.defaultValue,
  });

  factory InputFieldDefinition.fromJson(Map<String, dynamic> json) {
    return InputFieldDefinition(
      key: json['key'] as String? ?? '',
      labelKey: json['label_key'] as String? ?? '',
      type: json['type'] as String? ?? 'text',
      placeholder: json['placeholder'] as String?,
      validation: json['validation'] != null
          ? InputFieldValidation.fromJson(
              json['validation'] as Map<String, dynamic>,
            )
          : null,
      defaultValue: json['default'],
    );
  }

  Map<String, dynamic> toJson() => {
    'key': key,
    'label_key': labelKey,
    'type': type,
    if (placeholder != null) 'placeholder': placeholder,
    if (validation != null) 'validation': validation!.toJson(),
    if (defaultValue != null) 'default': defaultValue,
  };
}

/// Input Schema - defines the structured input for an action
class InputSchema {
  final List<InputFieldDefinition> fields;

  const InputSchema({this.fields = const []});

  factory InputSchema.fromJson(Map<String, dynamic> json) {
    return InputSchema(
      fields:
          (json['fields'] as List<dynamic>?)
              ?.map(
                (e) => InputFieldDefinition.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() => {
    'fields': fields.map((f) => f.toJson()).toList(),
  };
}

/// Action - represents a single executable action on an order
/// Frontend renders buttons directly from this structure - no business logic in UI
class Action {
  final String type; // Action type enum (mark_shipped, complete, refund, etc.)
  final String labelKey; // Localization key for button label
  final bool enabled; // Whether the action is currently enabled
  final ActionBlockedReason? blocked; // Why blocked (if disabled)
  final String endpoint; // API endpoint to call
  final String method; // HTTP method (POST, PATCH, etc.)
  final bool requiresIdempotency; // Whether action requires idempotency key
  final bool financial; // Whether action affects money (ledger validation)
  final InputSchema? inputSchema; // Structured input definition

  // Deprecated fields - kept for backward compatibility during migration
  final bool? requiresInput;
  final String? inputHint;
  final String? inputType;

  const Action({
    required this.type,
    required this.labelKey,
    required this.enabled,
    this.blocked,
    required this.endpoint,
    required this.method,
    required this.requiresIdempotency,
    required this.financial,
    this.inputSchema,
    this.requiresInput,
    this.inputHint,
    this.inputType,
  });

  factory Action.fromJson(Map<String, dynamic> json) {
    return Action(
      type: json['type'] as String? ?? '',
      labelKey: json['label_key'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? true,
      blocked: json['blocked'] != null
          ? ActionBlockedReason.fromJson(
              json['blocked'] as Map<String, dynamic>,
            )
          : null,
      endpoint: json['endpoint'] as String? ?? '',
      method: json['method'] as String? ?? 'POST',
      requiresIdempotency: json['requires_idempotency'] as bool? ?? false,
      financial: json['financial'] as bool? ?? false,
      inputSchema: json['input_schema'] != null
          ? InputSchema.fromJson(json['input_schema'] as Map<String, dynamic>)
          : null,
      // Deprecated fields
      requiresInput: json['requires_input'] as bool?,
      inputHint: json['input_hint'] as String?,
      inputType: json['input_type'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'type': type,
    'label_key': labelKey,
    'enabled': enabled,
    if (blocked != null) 'blocked': blocked!.toJson(),
    'endpoint': endpoint,
    'method': method,
    'requires_idempotency': requiresIdempotency,
    'financial': financial,
    if (inputSchema != null) 'input_schema': inputSchema!.toJson(),
    if (requiresInput != null) 'requires_input': requiresInput,
    if (inputHint != null) 'input_hint': inputHint,
    if (inputType != null) 'input_type': inputType,
  };
}

/// Decision Contract V2 from Backend
///
/// Backend is the SINGLE SOURCE OF TRUTH for all business decisions.
/// Frontend MUST NOT derive state or allowed actions from other fields.
///
/// Use decision.primary_action and decision.secondary_actions to render UI.
/// Use decision.state for authoritative business state.
/// Use decision.display for UI rendering hints (labels, badges, warnings).
class DecisionContract {
  final String state; // Authoritative business state (order status)
  final String version; // Decision contract version
  final int decisionVersion; // Optimistic concurrency counter
  final Action? primaryAction; // Main call-to-action
  final List<Action> secondaryActions; // Alternative actions
  final DisplayHints? display; // UI rendering hints

  const DecisionContract({
    required this.state,
    this.version = '3.0.0',
    this.decisionVersion = 0,
    this.primaryAction,
    this.secondaryActions = const [],
    this.display,
  });

  factory DecisionContract.fromJson(Map<String, dynamic>? json) {
    if (json == null) {
      return const DecisionContract(state: '');
    }

    // Parse primary_action
    final primaryActionJson = json['primary_action'];
    Action? primaryAction;
    if (primaryActionJson != null &&
        primaryActionJson is Map<String, dynamic>) {
      primaryAction = Action.fromJson(primaryActionJson);
    }

    // Parse secondary_actions
    final secondaryActionsJson = json['secondary_actions'];
    List<Action> secondaryActions = const [];
    if (secondaryActionsJson != null && secondaryActionsJson is List) {
      secondaryActions = secondaryActionsJson
          .map((e) => e is Map<String, dynamic> ? Action.fromJson(e) : null)
          .whereType<Action>()
          .toList();
    }

    // Parse display hints
    final displayJson = json['display'];
    DisplayHints? display;
    if (displayJson != null && displayJson is Map<String, dynamic>) {
      display = DisplayHints.fromJson(displayJson);
    }

    return DecisionContract(
      state: json['state'] as String? ?? '',
      version: json['version'] as String? ?? '3.0.0',
      decisionVersion: json['decision_version'] as int? ?? 0,
      primaryAction: primaryAction,
      secondaryActions: secondaryActions,
      display: display,
    );
  }

  Map<String, dynamic> toJson() => {
    'state': state,
    'version': version,
    'decision_version': decisionVersion,
    if (primaryAction != null) 'primary_action': primaryAction!.toJson(),
    if (secondaryActions.isNotEmpty)
      'secondary_actions': secondaryActions.map((a) => a.toJson()).toList(),
    if (display != null) 'display': display!.toJson(),
  };

  /// Get all available actions (primary + secondary)
  List<Action> get allActions {
    final actions = <Action>[];
    if (primaryAction != null) {
      actions.add(primaryAction!);
    }
    actions.addAll(secondaryActions);
    return actions;
  }

  /// Check if an action type is available and enabled
  bool hasActionType(String actionType) {
    return allActions.any((a) => a.type == actionType && a.enabled);
  }
}

/// Display Hints from Backend (NON-AUTHORITATIVE)
///
/// These are UI hints ONLY. Frontend MUST NOT derive state or
/// allowed_actions from these hints. Always use decision.state and
/// decision.allowed_actions for logic.
class DisplayHints {
  final String? badge;
  final String? badgeVariant;
  final String? primaryAction;
  final String? warning;
  final String? info;
  final int? timeRemainingSeconds;

  const DisplayHints({
    this.badge,
    this.badgeVariant,
    this.primaryAction,
    this.warning,
    this.info,
    this.timeRemainingSeconds,
  });

  factory DisplayHints.fromJson(Map<String, dynamic> json) {
    return DisplayHints(
      badge: json['badge'] as String?,
      badgeVariant: json['badge_variant'] as String?,
      primaryAction: json['primary_action'] as String?,
      warning: json['warning'] as String?,
      info: json['info'] as String?,
      timeRemainingSeconds: json['time_remaining_seconds'] as int?,
    );
  }

  Map<String, dynamic> toJson() => {
    'badge': badge,
    'badge_variant': badgeVariant,
    'primary_action': primaryAction,
    'warning': warning,
    'info': info,
    'time_remaining_seconds': timeRemainingSeconds,
  };
}

// =============================================================================
// ORDER DOMAIN CLOSURE - GUARDRAILS FOR FUTURE DEVELOPERS
// =============================================================================
//
// This domain is CLOSED. The following residues were intentionally removed:
//
// REMOVED RESIDUE:
// 1. primaryStatus - Was redundant (always duplicated status field)
// 2. activeIssues - Was never populated or used
// 3. OrderIssue enum - Was unused, status authority belongs to backend
// 4. All canX methods (canCancel, canAccept, etc.) - Use decision contract instead
//
// DESIGN PRINCIPLES:
// 1. Backend is SINGLE SOURCE OF TRUTH for all order state
// 2. Use decision.state for authoritative business state (NOT status field alone)
// 3. Use decision.primary_action and decision.secondary_actions for ALL actions
// 4. NEVER derive allowed actions from status in Flutter
// 5. DisplayHints are UI-only hints, NEVER use for business logic
//
// STATUS MAPPING:
// - OrderStatus enum matches backend Go enum exactly
// - Use OrderStatusExtension.parse() for string → OrderStatus conversion
// - Handles legacy mappings (waiting_payment → pending, confirmed → paid)
//
// WHEN ADDING NEW FEATURES:
// - DO NOT add status-related fields to this entity
// - DO NOT create "convenience" status getters (isX, canX)
// - DO extend DecisionContract if backend adds new metadata
// - DO use decision.display for UI hints only
// =============================================================================

/// Order Entity
class Order extends Equatable {
  final String id;
  final String buyerId;
  final String sellerId;
  final List<OrderItem> items;
  final OrderStatus status;

  // Status System V2
  // NOTE: primaryStatus was removed as redundant - always duplicated status
  // NOTE: OrderSecondaryStatus was removed in Track 2 - not in backend
  // NOTE: activeIssues was removed - never populated, OrderIssue enum unused
  // Status authority: backend is single source of truth via decision.state
  final OrderStatus? statusBeforeRefund;
  final DateTime? lastStatusChangeAt;
  final String? lastStatusChangedBy;

  // Payment & Shipping
  final PaymentMethodType paymentMethod;
  final PaymentStatus paymentStatus;
  final PaymentChannel? paymentChannel;
  final int tokenRegenerationCount;
  final ShippingInfo shippingInfo;
  final OrderPricing pricing;
  final String? notes;
  final String? cancelReason;

  // 🔒 ESCROW STATUS - Financial state of the order
  // Backend authority: backend/internal/commerce/order/entity/escrow_status.go
  // Matches backend Go enum exactly - do not modify without backend alignment
  // Nullable: null = unknown value from backend (should never happen in production)
  final EscrowStatus? escrowStatus;

  // Shipping Readiness Snapshot - frozen at order creation time
  // This preserves the buyer's expectation at purchase time, even if seller
  // later changes the listing/auction preparation time
  final PreparationTime preparationTimeSnapshot;
  final String? preparationNoteSnapshot;
  final DateTime?
  readyToShipBy; // Calculated deadline: paid_at + preparation_days

  // Overdue Display Layer (computed by backend, not persisted)
  // These fields are populated when order is past ready_to_ship_by
  final String?
  overdueTier; // none, overdue, severely_overdue, critical_overdue
  final int? overdueDays; // Days past ready_to_ship_by (null if not overdue)
  final bool? isOverdue; // Convenience boolean

  final DateTime createdAt;
  final DateTime? paidAt;
  final DateTime? shippedAt;
  final DateTime? deliveredAt;
  final DateTime? cancelledAt;
  final DateTime? completedAt;
  final String? transactionId;

  // TRACK 4: Backend fields - added for Order entity completeness
  final String? orderNumber;
  final String? idempotencyKey;
  final String? priceSnapshotId;
  final String? discountId;
  final DateTime? buyerConfirmDeadline;
  final DateTime? confirmedAt;
  final DateTime? deletedAt;

  // Payment Tracking
  final double? paidViaBalance;
  final double? paidViaMidtrans;
  final String? balanceTransactionId;
  final String? midtransTransactionId;
  final DateTime? paymentDeadline;
  final DateTime? acceptanceDeadline;

  // Refund Integration
  // activeRefundId and refundStatus are order-domain truth (tracking refund state)
  // NOTE: refundedAmount was removed - seller financial data must come from finance-derived sources
  final String? activeRefundId;
  final String? refundStatus;
  final bool hasActiveRefund;
  final RefundRequest? activeRefund;

  // Source
  final OrderSource source;
  final String? sourceId;

  // P11 Phase 2: Decision Contract from Backend
  // Backend is the SINGLE SOURCE OF TRUTH for all business decisions.
  final DecisionContract? decision;

  // ===========================================================================
  // STAGE 3 — IDENTITY FIELDS (Phase 5)
  // ===========================================================================
  // Owner Truth identity scalars from Stage 1 backend payload, populated by
  // the order mapper (OrderMapper.toOrder + OrderResponseDto.toEntity).
  // All nullable: old payloads may not carry them, and Stage 3 explicitly
  // forbids fake fallback. UI consumption is deferred to Stage 4.
  // - sellerUsername  ← seller_username   (account/user identity)
  // - sellerFarmName  ← seller_farm_name  (seller/store identity)
  // - sellerAvatarUrl ← seller_avatar_url (display avatar)
  // - buyerUsername   ← buyer_username    (buyer account/user identity)
  // No fullName / displayName field — that is private/KYC.
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;
  final String? buyerUsername;

  // Payment identity — populated when a payment row exists for this order.
  final String? paymentId;

  const Order({
    required this.id,
    required this.buyerId,
    required this.sellerId,
    required this.items,
    required this.status,
    // NOTE: primaryStatus removed - redundant (always duplicated status)
    // NOTE: activeIssues removed - never populated, use decision.state for status
    // secondaryStatus removed - not in backend
    this.statusBeforeRefund,
    this.lastStatusChangeAt,
    this.lastStatusChangedBy,
    required this.paymentMethod,
    required this.paymentStatus,
    this.paymentChannel,
    this.tokenRegenerationCount = 0,
    required this.shippingInfo,
    required this.pricing,
    this.notes,
    this.cancelReason,
    // 🔒 ESCROW STATUS - Financial state from backend
    // Nullable: null = unknown value (error state)
    this.escrowStatus,
    this.preparationTimeSnapshot = PreparationTime.immediate,
    this.preparationNoteSnapshot,
    this.readyToShipBy,
    this.overdueTier,
    this.overdueDays,
    this.isOverdue,
    required this.createdAt,
    this.paidAt,
    this.shippedAt,
    this.deliveredAt,
    this.cancelledAt,
    this.completedAt,
    this.transactionId,
    // TRACK 4: Backend fields
    this.orderNumber,
    this.idempotencyKey,
    this.priceSnapshotId,
    this.discountId,
    this.buyerConfirmDeadline,
    this.confirmedAt,
    this.deletedAt,
    this.paidViaBalance,
    this.paidViaMidtrans,
    this.balanceTransactionId,
    this.midtransTransactionId,
    this.paymentDeadline,
    this.acceptanceDeadline,
    this.activeRefundId,
    this.refundStatus,
    this.hasActiveRefund = false,
    this.activeRefund,
    required this.source,
    this.sourceId,
    this.decision,
    // Stage 3 identity fields (Owner Truth: username/farmName/avatar).
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.buyerUsername,
    this.paymentId,
  });

  double get totalAmount => pricing.total;

  // ============================================================
  // BACKWARD COMPATIBILITY: These getters are kept for existing code
  // New code should use decision contract from backend instead
  // ============================================================

  /// Check if seller action is required (for UI display)
  ///
  /// O1: Updated to remove 'processing' which was never a real backend status.
  /// Better: Use decision.allowed_actions.contains('accept', 'ship', etc.)
  bool get isSellerActionRequired {
    // Only pending orders require seller acceptance/action
    return switch (status) {
      OrderStatus.pending => true,
      _ => false,
    };
  }

  // P11 Phase 2: All canX methods removed - use decision.allowed_actions instead
  // BEFORE: bool get canCancel => ...
  // AFTER: decision.allowed_actions.contains('cancel')

  @override
  List<Object?> get props => [
    id,
    buyerId,
    sellerId,
    items,
    status,
    // NOTE: primaryStatus removed - redundant
    // NOTE: activeIssues removed - unused
    // secondaryStatus removed - not in backend
    statusBeforeRefund,
    lastStatusChangeAt,
    lastStatusChangedBy,
    paymentMethod,
    paymentStatus,
    paymentChannel,
    tokenRegenerationCount,
    shippingInfo,
    pricing,
    notes,
    cancelReason,
    escrowStatus, // 🔒 ESCROW STATUS - added to props
    preparationTimeSnapshot,
    preparationNoteSnapshot,
    readyToShipBy,
    overdueTier,
    overdueDays,
    isOverdue,
    createdAt,
    paidAt,
    shippedAt,
    deliveredAt,
    cancelledAt,
    completedAt,
    transactionId,
    // TRACK 4: Backend fields
    orderNumber,
    idempotencyKey,
    priceSnapshotId,
    discountId,
    buyerConfirmDeadline,
    confirmedAt,
    deletedAt,
    paidViaBalance,
    paidViaMidtrans,
    balanceTransactionId,
    midtransTransactionId,
    paymentDeadline,
    acceptanceDeadline,
    activeRefundId,
    refundStatus,
    hasActiveRefund,
    activeRefund,
    source,
    sourceId,
    decision,
    // Stage 3 identity fields
    sellerUsername,
    sellerFarmName,
    sellerAvatarUrl,
    buyerUsername,
    paymentId,
  ];

  Order copyWith({
    String? id,
    String? buyerId,
    String? sellerId,
    List<OrderItem>? items,
    OrderStatus? status,
    // NOTE: primaryStatus removed - redundant
    // NOTE: activeIssues removed - unused
    // secondaryStatus removed - not in backend
    OrderStatus? statusBeforeRefund,
    DateTime? lastStatusChangeAt,
    String? lastStatusChangedBy,
    PaymentMethodType? paymentMethod,
    PaymentStatus? paymentStatus,
    PaymentChannel? paymentChannel,
    int? tokenRegenerationCount,
    ShippingInfo? shippingInfo,
    OrderPricing? pricing,
    String? notes,
    String? cancelReason,
    PreparationTime? preparationTimeSnapshot,
    String? preparationNoteSnapshot,
    DateTime? readyToShipBy,
    String? overdueTier,
    int? overdueDays,
    bool? isOverdue,
    DateTime? createdAt,
    DateTime? paidAt,
    DateTime? shippedAt,
    DateTime? deliveredAt,
    DateTime? cancelledAt,
    DateTime? completedAt,
    String? transactionId,
    // TRACK 4: Backend fields
    String? orderNumber,
    String? idempotencyKey,
    String? priceSnapshotId,
    String? discountId,
    DateTime? buyerConfirmDeadline,
    DateTime? confirmedAt,
    DateTime? deletedAt,
    double? paidViaBalance,
    double? paidViaMidtrans,
    String? balanceTransactionId,
    String? midtransTransactionId,
    DateTime? paymentDeadline,
    DateTime? acceptanceDeadline,
    String? activeRefundId,
    String? refundStatus,
    bool? hasActiveRefund,
    RefundRequest? activeRefund,
    OrderSource? source,
    String? sourceId,
    DecisionContract? decision,
    EscrowStatus? escrowStatus,
    // Stage 3 identity fields
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatarUrl,
    String? buyerUsername,
    String? paymentId,
  }) {
    return Order(
      id: id ?? this.id,
      buyerId: buyerId ?? this.buyerId,
      sellerId: sellerId ?? this.sellerId,
      items: items ?? this.items,
      status: status ?? this.status,
      // NOTE: primaryStatus removed - redundant
      // NOTE: activeIssues removed - unused
      // secondaryStatus removed - not in backend
      statusBeforeRefund: statusBeforeRefund ?? this.statusBeforeRefund,
      lastStatusChangeAt: lastStatusChangeAt ?? this.lastStatusChangeAt,
      lastStatusChangedBy: lastStatusChangedBy ?? this.lastStatusChangedBy,
      paymentMethod: paymentMethod ?? this.paymentMethod,
      paymentStatus: paymentStatus ?? this.paymentStatus,
      paymentChannel: paymentChannel ?? this.paymentChannel,
      tokenRegenerationCount:
          tokenRegenerationCount ?? this.tokenRegenerationCount,
      shippingInfo: shippingInfo ?? this.shippingInfo,
      pricing: pricing ?? this.pricing,
      notes: notes ?? this.notes,
      cancelReason: cancelReason ?? this.cancelReason,
      preparationTimeSnapshot:
          preparationTimeSnapshot ?? this.preparationTimeSnapshot,
      preparationNoteSnapshot:
          preparationNoteSnapshot ?? this.preparationNoteSnapshot,
      readyToShipBy: readyToShipBy ?? this.readyToShipBy,
      overdueTier: overdueTier ?? this.overdueTier,
      overdueDays: overdueDays ?? this.overdueDays,
      isOverdue: isOverdue ?? this.isOverdue,
      createdAt: createdAt ?? this.createdAt,
      paidAt: paidAt ?? this.paidAt,
      shippedAt: shippedAt ?? this.shippedAt,
      deliveredAt: deliveredAt ?? this.deliveredAt,
      cancelledAt: cancelledAt ?? this.cancelledAt,
      completedAt: completedAt ?? this.completedAt,
      transactionId: transactionId ?? this.transactionId,
      // TRACK 4: Backend fields
      orderNumber: orderNumber ?? this.orderNumber,
      idempotencyKey: idempotencyKey ?? this.idempotencyKey,
      priceSnapshotId: priceSnapshotId ?? this.priceSnapshotId,
      discountId: discountId ?? this.discountId,
      buyerConfirmDeadline: buyerConfirmDeadline ?? this.buyerConfirmDeadline,
      confirmedAt: confirmedAt ?? this.confirmedAt,
      deletedAt: deletedAt ?? this.deletedAt,
      paidViaBalance: paidViaBalance ?? this.paidViaBalance,
      paidViaMidtrans: paidViaMidtrans ?? this.paidViaMidtrans,
      balanceTransactionId: balanceTransactionId ?? this.balanceTransactionId,
      midtransTransactionId:
          midtransTransactionId ?? this.midtransTransactionId,
      paymentDeadline: paymentDeadline ?? this.paymentDeadline,
      acceptanceDeadline: acceptanceDeadline ?? this.acceptanceDeadline,
      activeRefundId: activeRefundId ?? this.activeRefundId,
      refundStatus: refundStatus ?? this.refundStatus,
      hasActiveRefund: hasActiveRefund ?? this.hasActiveRefund,
      activeRefund: activeRefund ?? this.activeRefund,
      source: source ?? this.source,
      sourceId: sourceId ?? this.sourceId,
      decision: decision ?? this.decision,
      escrowStatus: escrowStatus ?? this.escrowStatus,
      // Stage 3 identity fields
      sellerUsername: sellerUsername ?? this.sellerUsername,
      sellerFarmName: sellerFarmName ?? this.sellerFarmName,
      sellerAvatarUrl: sellerAvatarUrl ?? this.sellerAvatarUrl,
      buyerUsername: buyerUsername ?? this.buyerUsername,
      paymentId: paymentId ?? this.paymentId,
    );
  }
}
