/// Payment Entity
///
/// Pure Dart entity for payment transactions.
/// No dependencies on external services or Firestore.
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/core/common/types/payment_types.dart';

// ============================================================
// P11 PHASE 2: DECISION CONTRACT (Backend is Authority)
// ============================================================
// All business decisions come from backend via decision contract.
// Frontend MUST NOT compute payment state or allowed actions.

/// Decision Contract from Backend
class DecisionContract {
  final String state;
  final List<String> allowedActions;
  final DisplayHints? display;

  const DecisionContract({
    required this.state,
    this.allowedActions = const [],
    this.display,
  });

  factory DecisionContract.fromJson(Map<String, dynamic>? json) {
    if (json == null) {
      return const DecisionContract(state: '', allowedActions: []);
    }
    return DecisionContract(
      state: json['state'] as String? ?? '',
      allowedActions:
          (json['allowed_actions'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      display: json['display'] != null
          ? DisplayHints.fromJson(json['display'] as Map<String, dynamic>)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'state': state,
    'allowed_actions': allowedActions,
    if (display != null) 'display': display!.toJson(),
  };
}

/// Display Hints from Backend (NON-AUTHORITATIVE)
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

/// Main payment entity
class Payment extends Equatable {
  /// Unique payment ID
  final String id;

  /// Human-readable payment number (e.g., "PAY-1706123456")
  final String paymentNumber;

  /// User ID who made the payment
  final String userId;

  /// `gross_amount` — cash after coin deduction plus the buyer fee (Rupiah).
  /// Whole Rupiah integer on the wire; never a double, never a minor unit.
  final int grossAmount;

  /// `coins_to_use` — loyalty coins redeemed on this payment (count, not money).
  final int coinsToUse;

  /// `coin_discount_amount` — Rupiah value of [coinsToUse].
  final int coinDiscountAmount;

  /// Payment status
  final PaymentStatus status;

  /// Midtrans order ID (if applicable)
  final String? midtransOrderId;

  /// Midtrans transaction ID (if applicable)
  final String? midtransTransactionId;

  /// Midtrans payment type (e.g., "bank_transfer", "gopay")
  final String? midtransPaymentType;

  /// Reference type (e.g., "order", "seller_subscription")
  /// IMPORTANT: Coins are loyalty points for discounts, NOT purchasable packages
  final String referenceType;

  /// Reference ID (ID of the related entity)
  final String? referenceId;

  /// When the payment was created
  final DateTime createdAt;

  /// When the payment was completed (nullable)
  final DateTime? paidAt;

  /// When the payment expires (nullable)
  final DateTime? expiredAt;

  /// Payment URL for redirect (nullable)
  final String? paymentUrl;

  // P11 Phase 2: Decision Contract from Backend
  final DecisionContract? decision;

  /// Price snapshot ID from backend - SINGLE SOURCE OF TRUTH for pricing
  final String? priceSnapshotId;

  /// When the payment was last updated (from backend)
  final DateTime? updatedAt;

  const Payment({
    required this.id,
    required this.paymentNumber,
    required this.userId,
    required this.grossAmount,
    required this.coinsToUse,
    required this.coinDiscountAmount,
    required this.status,
    required this.referenceType,
    required this.createdAt,
    this.midtransOrderId,
    this.midtransTransactionId,
    this.midtransPaymentType,
    this.referenceId,
    this.paidAt,
    this.expiredAt,
    this.paymentUrl,
    this.decision,
    this.priceSnapshotId,
    this.updatedAt,
  });

  // P11 Phase 2: All validation logic computed properties removed
  // BEFORE: bool get isValid => status == PaymentStatus.pending && ...
  // AFTER: Use decision.state from backend
  //
  // BEFORE: bool get canPay => status.canPay && isValid;
  // AFTER: Use decision.allowed_actions.contains('pay')
  //
  // BEFORE: bool get isCompleted => status.isSuccess;
  // AFTER: Use decision.state from backend
  //
  // BEFORE: bool get isFailed => status == PaymentStatus.failed || ...
  // AFTER: Use decision.state from backend
  //
  // BEFORE: Duration? get timeRemaining { ... }
  // AFTER: Use decision.display.timeRemainingSeconds from backend

  /// Create a copy with modified fields
  Payment copyWith({
    String? id,
    String? paymentNumber,
    String? userId,
    int? grossAmount,
    int? coinsToUse,
    int? coinDiscountAmount,
    PaymentStatus? status,
    String? midtransOrderId,
    String? midtransTransactionId,
    String? midtransPaymentType,
    String? referenceType,
    String? referenceId,
    DateTime? createdAt,
    DateTime? paidAt,
    DateTime? expiredAt,
    String? paymentUrl,
    DecisionContract? decision,
    String? priceSnapshotId,
    DateTime? updatedAt,
  }) {
    return Payment(
      id: id ?? this.id,
      paymentNumber: paymentNumber ?? this.paymentNumber,
      userId: userId ?? this.userId,
      grossAmount: grossAmount ?? this.grossAmount,
      coinsToUse: coinsToUse ?? this.coinsToUse,
      coinDiscountAmount: coinDiscountAmount ?? this.coinDiscountAmount,
      status: status ?? this.status,
      midtransOrderId: midtransOrderId ?? this.midtransOrderId,
      midtransTransactionId:
          midtransTransactionId ?? this.midtransTransactionId,
      midtransPaymentType: midtransPaymentType ?? this.midtransPaymentType,
      referenceType: referenceType ?? this.referenceType,
      referenceId: referenceId ?? this.referenceId,
      createdAt: createdAt ?? this.createdAt,
      paidAt: paidAt ?? this.paidAt,
      expiredAt: expiredAt ?? this.expiredAt,
      paymentUrl: paymentUrl ?? this.paymentUrl,
      decision: decision ?? this.decision,
      priceSnapshotId: priceSnapshotId ?? this.priceSnapshotId,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    paymentNumber,
    userId,
    grossAmount,
    coinsToUse,
    coinDiscountAmount,
    status,
    midtransOrderId,
    midtransTransactionId,
    referenceType,
    referenceId,
    createdAt,
    paidAt,
    expiredAt,
    paymentUrl,
    decision,
    priceSnapshotId,
    updatedAt,
  ];
}

/// Request to create a new payment
///
/// Matches backend CreatePaymentRequest struct:
///   order_id            uuid.UUID (required)
///   payment_method_code string    (required)
///   price_snapshot_id   *uuid.UUID
///
/// PASS_18V: the backend is the sole authority for the buyer payment fee and
/// gross amount. The client selects a canonical payment method (see
/// PaymentRepository.getPaymentMethodOptions) and sends only its code —
/// it never computes or submits a fee/gross amount.
///
/// PAY-B: there is NO `coins_to_use` here. K is fixed at Order creation via the
/// checkout `use_coins` intent; the backend persists it on the pricing token
/// and derives it at payment. A payment-time K would be a competing authority.
class CreatePaymentRequest {
  /// Order ID to create payment for (required)
  final String orderId;

  /// Canonical payment method code the buyer selected (required) — backend
  /// calculates the fee and gross amount from this.
  final String paymentMethodCode;

  /// Price snapshot ID from order (optional, for backend validation)
  final String? priceSnapshotId;

  const CreatePaymentRequest({
    required this.orderId,
    required this.paymentMethodCode,
    this.priceSnapshotId,
  });

  /// Validate request
  String? validate() {
    if (orderId.isEmpty) {
      return 'Order ID is required';
    }
    if (paymentMethodCode.isEmpty) {
      return 'Payment method is required';
    }
    return null;
  }

  /// Convert to JSON for API request — matches backend binding struct.
  ///
  /// There is no `coins_to_use` (nor the stale `coin_discount`) key: the
  /// payment is created from the order's canonical pricing-token K snapshot.
  Map<String, dynamic> toJson() => {
    'order_id': orderId,
    'payment_method_code': paymentMethodCode,
    if (priceSnapshotId != null) 'price_snapshot_id': priceSnapshotId,
  };
}

/// A canonical payment method option with the buyer payment fee/total the
/// backend calculated for a specific order — see GET /payments/methods.
///
/// PASS_18V: fee/total are backend-authoritative display values. The buyer
/// picks one of these before CreatePaymentRequest is sent.
class PaymentMethodOption {
  final String methodCode;
  final String displayName;
  final int buyerPaymentFeeAmount;
  final int totalPayableAmount;

  const PaymentMethodOption({
    required this.methodCode,
    required this.displayName,
    required this.buyerPaymentFeeAmount,
    required this.totalPayableAmount,
  });
}
