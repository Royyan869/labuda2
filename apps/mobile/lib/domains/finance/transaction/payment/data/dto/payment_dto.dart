/// Payment DTOs
///
/// Data Transfer Objects for payment API responses.
/// Handles serialization/deserialization of API data.
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

/// Payment DTO from API
class PaymentDto extends Equatable {
  final String id;
  final String paymentNumber;
  final String userId;
  final double grossAmount;
  final int coinDiscount;
  final double coinDiscountAmount;
  final double netAmount;
  final String status;
  final String? midtransOrderId;
  final String? midtransTransactionId;
  final String? midtransPaymentType;
  final String? midtransStatus;
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
  final DecisionContractResponseDto? decision;

  const PaymentDto({
    required this.id,
    required this.paymentNumber,
    required this.userId,
    required this.grossAmount,
    required this.coinDiscount,
    required this.coinDiscountAmount,
    required this.netAmount,
    required this.status,
    this.midtransOrderId,
    this.midtransTransactionId,
    this.midtransPaymentType,
    this.midtransStatus,
    required this.referenceType,
    this.referenceId,
    required this.createdAt,
    this.paidAt,
    this.expiredAt,
    this.paymentUrl,
    this.priceSnapshotId,
    this.updatedAt,
    this.decision,
  });

  /// Parse from JSON
  ///
  /// TRACK 8: Added decision parsing from backend (consistent with Order)
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
      grossAmount: (json['gross_amount'] as num).toDouble(),
      coinDiscount: json['coin_discount'] as int? ?? 0,
      coinDiscountAmount:
          (json['coin_discount_amount'] as num?)?.toDouble() ?? 0.0,
      netAmount: (json['net_amount'] as num).toDouble(),
      status: json['status'] as String,
      midtransOrderId: json['midtrans_order_id'] as String?,
      midtransTransactionId: json['midtrans_transaction_id'] as String?,
      midtransPaymentType: json['midtrans_payment_type'] as String?,
      midtransStatus: json['midtrans_status'] as String?,
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
  ///
  /// TRACK 8: Added decision conversion to Payment entity (consistent with Order)
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
      coinDiscount: coinDiscount,
      coinDiscountAmount: coinDiscountAmount,
      netAmount: netAmount,
      status: PaymentStatus.fromString(status),
      midtransOrderId: midtransOrderId,
      midtransTransactionId: midtransTransactionId,
      midtransPaymentType: midtransPaymentType,
      midtransStatus: midtransStatus,
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

/// Payment Intent DTO from API
class PaymentIntentDto extends Equatable {
  final String id;
  final String paymentNumber;
  final double amount;
  final String currency;
  final String status;
  final String? paymentUrl;
  final String? deepLinkUrl;
  final String? vaNumber;
  final String? vaBank;
  final String? qrString;
  final DateTime? expiresAt;

  const PaymentIntentDto({
    required this.id,
    required this.paymentNumber,
    required this.amount,
    required this.currency,
    required this.status,
    this.paymentUrl,
    this.deepLinkUrl,
    this.vaNumber,
    this.vaBank,
    this.qrString,
    this.expiresAt,
  });

  /// Parse from JSON
  factory PaymentIntentDto.fromJson(Map<String, dynamic> json) {
    return PaymentIntentDto(
      id: json['id'] as String,
      paymentNumber: json['payment_number'] as String? ?? '',
      amount: (json['amount'] as num).toDouble(),
      currency: json['currency'] as String? ?? 'IDR',
      status: json['status'] as String? ?? 'pending',
      paymentUrl: json['payment_url'] as String?,
      deepLinkUrl: json['deep_link_url'] as String?,
      vaNumber: json['va_number'] as String?,
      vaBank: json['va_bank'] as String?,
      qrString: json['qr_string'] as String?,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
    );
  }

  /// Convert to entity
  PaymentIntent toEntity() {
    return PaymentIntent(
      id: id,
      paymentNumber: paymentNumber,
      amount: amount,
      currency: currency,
      status: status,
      paymentUrl: paymentUrl,
      deepLinkUrl: deepLinkUrl,
      vaNumber: vaNumber,
      vaBank: vaBank,
      qrString: qrString,
      expiresAt: expiresAt,
    );
  }

  @override
  List<Object?> get props => [id, paymentNumber, amount, status];
}

/// Create Payment Request DTO
///
/// Matches backend CreatePaymentRequest struct:
///   order_id            uuid.UUID (required)
///   payment_method_code string    (required)
///   coin_discount       int
///   price_snapshot_id   *uuid.UUID
///
/// PASS_18V: backend calculates the buyer payment fee/gross amount from the
/// selected method — the client never submits either.
class CreatePaymentRequestDto {
  /// Order ID to create payment for (required)
  final String orderId;

  /// Canonical payment method code the buyer selected (required)
  final String paymentMethodCode;

  /// Number of coins to use for discount
  final int coinDiscount;

  /// Price snapshot ID from order (optional, for backend validation)
  final String? priceSnapshotId;

  const CreatePaymentRequestDto({
    required this.orderId,
    required this.paymentMethodCode,
    this.coinDiscount = 0,
    this.priceSnapshotId,
  });

  /// Convert to JSON — matches backend binding struct
  Map<String, dynamic> toJson() => {
    'order_id': orderId,
    'payment_method_code': paymentMethodCode,
    'coin_discount': coinDiscount,
    if (priceSnapshotId != null) 'price_snapshot_id': priceSnapshotId,
  };

  /// Create from entity request
  factory CreatePaymentRequestDto.fromRequest(CreatePaymentRequest request) {
    return CreatePaymentRequestDto(
      orderId: request.orderId,
      paymentMethodCode: request.paymentMethodCode,
      coinDiscount: request.coinDiscount,
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
class PaymentMethodOptionsDto {
  final String orderId;
  final int escrowAmount;
  final List<PaymentMethodOptionDto> methods;

  const PaymentMethodOptionsDto({
    required this.orderId,
    required this.escrowAmount,
    required this.methods,
  });

  factory PaymentMethodOptionsDto.fromJson(Map<String, dynamic> json) {
    return PaymentMethodOptionsDto(
      orderId: json['order_id'] as String,
      escrowAmount: (json['escrow_amount'] as num).toInt(),
      methods: (json['methods'] as List<dynamic>? ?? [])
          .map(
            (e) => PaymentMethodOptionDto.fromJson(e as Map<String, dynamic>),
          )
          .toList(),
    );
  }
}
