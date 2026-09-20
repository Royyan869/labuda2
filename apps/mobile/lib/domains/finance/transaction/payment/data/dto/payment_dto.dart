/// Payment DTOs
///
/// Data Transfer Objects for payment API responses.
/// Handles serialization/deserialization of API data.
///
/// WIRE AUTHORITY (do not invent fields here):
///   POST /api/v1/payments        → backend/internal/serverboot/dependencies.go
///                                  CorePaymentHandler.CreatePayment
///   GET  /api/v1/payments/:id    → CorePaymentHandler.GetPayment
///   GET  /api/v1/payments/methods→ CorePaymentHandler.ListPaymentMethods
///
/// PHASE 1F: Payment domain closure - using unified PaymentStatus from core
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/core/common/types/payment_types.dart';
import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';

/// Decision Contract DTO from backend
///
/// Backend is the SINGLE SOURCE OF TRUTH for all business decisions.
/// Frontend MUST NOT derive state or allowed actions from other fields.
///
/// TRACK 8: Added for Payment decision parsing (consistent with Order)
class DecisionContractResponseDto {
  final String state;
  final List<String> allowedActions;
  final DisplayHintsDto? display;

  const DecisionContractResponseDto({
    required this.state,
    this.allowedActions = const [],
    this.display,
  });

  factory DecisionContractResponseDto.fromJson(Map<String, dynamic> json) {
    return DecisionContractResponseDto(
      state: json['state'] as String? ?? '',
      allowedActions:
          (json['allowed_actions'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      display: json['display'] != null
          ? DisplayHintsDto.fromJson(json['display'] as Map<String, dynamic>)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'state': state,
    'allowed_actions': allowedActions,
    if (display != null) 'display': display!.toJson(),
  };
}

/// Display Hints DTO from backend (NON-AUTHORITATIVE)
///
/// These are UI hints ONLY. Frontend MUST NOT derive state or
/// allowed_actions from these hints.
///
/// TRACK 8: Added for Payment decision parsing (consistent with Order)
class DisplayHintsDto {
  final String? badge;
  final String? badgeVariant;
  final String? primaryAction;
  final String? warning;
  final String? info;
  final int? timeRemainingSeconds;

  const DisplayHintsDto({
    this.badge,
    this.badgeVariant,
    this.primaryAction,
    this.warning,
    this.info,
    this.timeRemainingSeconds,
  });

  factory DisplayHintsDto.fromJson(Map<String, dynamic> json) {
    return DisplayHintsDto(
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

/// Payment DTO from `GET /api/v1/payments/:id`.
///
/// Emitted keys (canonical, verbatim):
///   id, payment_number, user_id, gross_amount, coins_to_use,
///   coin_discount_amount, status, midtrans_order_id, midtrans_transaction_id,
///   midtrans_payment_type, reference_type, reference_id, created_at, paid_at,
///   expired_at, payment_url, price_snapshot_id, updated_at
///
/// There is NO `net_amount` on this wire: migration 000037 dropped
/// `payments.net_amount`, and the handler never re-emits it. Money is whole
/// Rupiah (int64) — never a double, never a minor unit.
class PaymentDto extends Equatable {
  final String id;
  final String paymentNumber;
  final String userId;
  final int grossAmount;
  final int coinsToUse;
  final int coinDiscountAmount;
  final String status;
  final String? midtransOrderId;
  final String? midtransTransactionId;
  final String? midtransPaymentType;
  final String referenceType;
  final String? referenceId;
  final DateTime createdAt;
  final DateTime? paidAt;
  final DateTime? expiredAt;
  final String? paymentUrl;

  /// Price snapshot ID from backend - SINGLE SOURCE OF TRUTH for pricing
  final String? priceSnapshotId;

  /// When the payment was last updated (from backend)
  final DateTime? updatedAt;

  /// Decision contract from backend - SINGLE SOURCE OF TRUTH for business decisions
  ///
  /// TRACK 8: Backend sends decision object for state-based UI rendering.
  /// Frontend MUST NOT derive state or allowed actions from other fields.
  ///
  /// NOTE: `GET /payments/:id` does not currently emit `decision`. This field
  /// is a nullable passthrough only; it is never an authority and its absence
  /// must never fail parsing.
  final DecisionContractResponseDto? decision;

  const PaymentDto({
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
    this.priceSnapshotId,
    this.updatedAt,
    this.decision,
  });

  /// Parse from JSON
  factory PaymentDto.fromJson(Map<String, dynamic> json) {
    // Parse decision object if present
    DecisionContractResponseDto? decision;
    if (json['decision'] != null) {
      decision = DecisionContractResponseDto.fromJson(
        json['decision'] as Map<String, dynamic>,
      );
    }

    return PaymentDto(
      id: json['id'] as String,
      paymentNumber: json['payment_number'] as String? ?? '',
      userId: json['user_id'] as String,
      grossAmount: (json['gross_amount'] as num).toInt(),
      coinsToUse: json['coins_to_use'] as int? ?? 0,
      coinDiscountAmount: (json['coin_discount_amount'] as num?)?.toInt() ?? 0,
      status: json['status'] as String,
      midtransOrderId: json['midtrans_order_id'] as String?,
      midtransTransactionId: json['midtrans_transaction_id'] as String?,
      midtransPaymentType: json['midtrans_payment_type'] as String?,
      referenceType: json['reference_type'] as String,
      referenceId: json['reference_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      paidAt: json['paid_at'] != null
          ? DateTime.parse(json['paid_at'] as String)
          : null,
      expiredAt: json['expired_at'] != null
          ? DateTime.parse(json['expired_at'] as String)
          : null,
      paymentUrl: json['payment_url'] as String?,
      priceSnapshotId: json['price_snapshot_id'] as String?,
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : null,
      decision: decision,
    );
  }

  /// Convert to entity
  Payment toEntity() {
    // Convert DecisionContractResponseDto to DecisionContract using fromJson
    // This leverages the existing domain entity factory method
    final domainDecision = decision != null
        ? DecisionContract.fromJson(decision!.toJson())
        : null;

    return Payment(
      id: id,
      paymentNumber: paymentNumber,
      userId: userId,
      grossAmount: grossAmount,
      coinsToUse: coinsToUse,
      coinDiscountAmount: coinDiscountAmount,
      status: PaymentStatus.fromString(status),
      midtransOrderId: midtransOrderId,
      midtransTransactionId: midtransTransactionId,
      midtransPaymentType: midtransPaymentType,
      referenceType: referenceType,
      referenceId: referenceId,
      createdAt: createdAt,
      paidAt: paidAt,
      expiredAt: expiredAt,
      paymentUrl: paymentUrl,
      priceSnapshotId: priceSnapshotId,
      updatedAt: updatedAt,
      decision: domainDecision,
    );
  }

  @override
  List<Object?> get props => [id, paymentNumber, status, createdAt];
}

/// Payment Intent DTO from `POST /api/v1/payments`.
///
/// Mirrors the canonical handler response exactly — see
/// `PaymentIntent` in domain/entities/payment_intent.dart for the key map.
/// The backend never emits `id`, `amount`, or `currency` on this endpoint.
class PaymentIntentDto extends Equatable {
  final String paymentId;
  final String paymentNumber;
  final String status;
  final String? paymentUrl;
  final String? paymentMethodCode;
  final int buyerPaymentFeeAmount;
  final int grossAmount;
  final int coinsToUse;
  final int coinDiscountAmount;
  final String referenceType;
  final String? referenceId;
  final DateTime? expiredAt;

  const PaymentIntentDto({
    required this.paymentId,
    required this.paymentNumber,
    required this.status,
    required this.buyerPaymentFeeAmount,
    required this.grossAmount,
    required this.coinsToUse,
    required this.coinDiscountAmount,
    required this.referenceType,
    this.paymentUrl,
    this.paymentMethodCode,
    this.referenceId,
    this.expiredAt,
  });

  /// Parse from JSON
  factory PaymentIntentDto.fromJson(Map<String, dynamic> json) {
    return PaymentIntentDto(
      paymentId: json['payment_id'] as String,
      paymentNumber: json['payment_number'] as String? ?? '',
      status: json['status'] as String,
      paymentUrl: json['payment_url'] as String?,
      paymentMethodCode: json['payment_method_code'] as String?,
      buyerPaymentFeeAmount:
          (json['buyer_payment_fee_amount'] as num?)?.toInt() ?? 0,
      grossAmount: (json['gross_amount'] as num).toInt(),
      coinsToUse: json['coins_to_use'] as int? ?? 0,
      coinDiscountAmount: (json['coin_discount_amount'] as num?)?.toInt() ?? 0,
      referenceType: json['reference_type'] as String,
      referenceId: json['reference_id'] as String?,
      expiredAt: json['expired_at'] != null
          ? DateTime.parse(json['expired_at'] as String)
          : null,
    );
  }

  /// Convert to entity
  PaymentIntent toEntity() {
    return PaymentIntent(
      paymentId: paymentId,
      paymentNumber: paymentNumber,
      status: status,
      paymentUrl: paymentUrl,
      paymentMethodCode: paymentMethodCode,
      buyerPaymentFeeAmount: buyerPaymentFeeAmount,
      grossAmount: grossAmount,
      coinsToUse: coinsToUse,
      coinDiscountAmount: coinDiscountAmount,
      referenceType: referenceType,
      referenceId: referenceId,
      expiredAt: expiredAt,
    );
  }

  @override
  List<Object?> get props => [
    paymentId,
    paymentNumber,
    status,
    paymentUrl,
    grossAmount,
  ];
}

/// Create Payment Request DTO
///
/// Matches backend CreatePaymentRequest struct:
///   order_id            uuid.UUID (required)
///   payment_method_code string    (required)
///   price_snapshot_id   *uuid.UUID
///
/// PAY-B: no `coins_to_use` — K is fixed at Order creation and derived by the
/// backend from the pricing token snapshot.
///
/// PASS_18V: the backend is the sole authority for the buyer payment fee and
/// gross amount; the client selects a canonical payment method (see
/// PaymentRepository.getPaymentMethodOptions) and sends only its code — it
/// never computes or submits a fee/gross amount.
///
/// There is no client idempotency key on this request: POST /payments is made
/// idempotent server-side (order + active-payment reuse). Do not add one here
/// without a matching backend contract.
class CreatePaymentRequestDto {
  /// Order ID to create payment for (required)
  final String orderId;

  /// Canonical payment method code the buyer selected (required)
  final String paymentMethodCode;

  /// Price snapshot ID from order (optional, for backend validation)
  final String? priceSnapshotId;

  const CreatePaymentRequestDto({
    required this.orderId,
    required this.paymentMethodCode,
    this.priceSnapshotId,
  });

  /// Convert to JSON — matches backend binding struct
  Map<String, dynamic> toJson() => {
    'order_id': orderId,
    'payment_method_code': paymentMethodCode,
    if (priceSnapshotId != null) 'price_snapshot_id': priceSnapshotId,
  };

  /// Create from entity request
  factory CreatePaymentRequestDto.fromRequest(CreatePaymentRequest request) {
    return CreatePaymentRequestDto(
      orderId: request.orderId,
      paymentMethodCode: request.paymentMethodCode,
      priceSnapshotId: request.priceSnapshotId,
    );
  }
}

/// A single canonical payment method option with its backend-calculated
/// buyer payment fee and total, as returned by GET /payments/methods.
///
/// PASS_18V: fee/total are display-only values computed by the backend —
/// the client never recomputes them.
class PaymentMethodOptionDto {
  final String methodCode;
  final String displayName;
  final int buyerPaymentFeeAmount;
  final int totalPayableAmount;

  const PaymentMethodOptionDto({
    required this.methodCode,
    required this.displayName,
    required this.buyerPaymentFeeAmount,
    required this.totalPayableAmount,
  });

  factory PaymentMethodOptionDto.fromJson(Map<String, dynamic> json) {
    return PaymentMethodOptionDto(
      methodCode: json['method_code'] as String,
      displayName: json['display_name'] as String,
      buyerPaymentFeeAmount: (json['buyer_payment_fee_amount'] as num).toInt(),
      totalPayableAmount: (json['total_payable_amount'] as num).toInt(),
    );
  }
}

/// Response wrapper for GET /payments/methods.
///
/// FIN-R01E-C: backend emits `base_amount` = order.TotalBeforeCoinsAmount = PD+S
/// as the ONE canonical order amount key for this endpoint.
///
/// FIN-R01E-D: the mobile client only ever consumes the per-method options
/// (`methods[]`); the order-level `base_amount` was parsed but never read
/// downstream, so it has been purged from this DTO rather than kept "just in
/// case". If an order-level base is ever needed, re-introduce it deliberately
/// from the canonical `base_amount` wire key — never from
/// `total_before_coins_amount` (the persisted ORDER column, not a wire key).
class PaymentMethodOptionsDto {
  final String orderId;
  final List<PaymentMethodOptionDto> methods;

  const PaymentMethodOptionsDto({
    required this.orderId,
    required this.methods,
  });

  factory PaymentMethodOptionsDto.fromJson(Map<String, dynamic> json) {
    return PaymentMethodOptionsDto(
      orderId: json['order_id'] as String,
      methods: (json['methods'] as List<dynamic>? ?? [])
          .map(
            (e) => PaymentMethodOptionDto.fromJson(e as Map<String, dynamic>),
          )
          .toList(),
    );
  }
}
