/// Payment Entity
///
/// Pure Dart entity for payment transactions.
/// No dependencies on external services or Firestore.
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/core/common/types/payment_types.dart';

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
    this.priceSnapshotId,
    this.updatedAt,
  });

  // No client-side business-state derivation: payment state comes from the
  // backend `status` field (PaymentStatus). Do not reintroduce
  // isValid/canPay/isCompleted/isFailed/timeRemaining getters here.

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

/// A payment method option for a PRE-ORDER pricing token, as returned by
/// `GET /payments/pre-order-methods`. `finalPayableAmount` is the post-fee
/// amount the buyer will pay for this method — computed by the backend, never
/// by the client.
class PreOrderPaymentMethodOption {
  final String methodCode;
  final String displayName;
  final int buyerPaymentFeeAmount;
  final int finalPayableAmount;

  const PreOrderPaymentMethodOption({
    required this.methodCode,
    required this.displayName,
    required this.buyerPaymentFeeAmount,
    required this.finalPayableAmount,
  });
}

/// Canonical pre-order payment pricing for one pricing token: the immutable
/// escrow base, the resolved coin redemption, the cash base, and every enabled
/// method with its backend-computed fee and FINAL payable amount.
///
/// This is the single source the checkout reads to display the final total and
/// to know which method the buyer selected. The client performs no arithmetic.
class PreOrderPaymentPricing {
  final String pricingToken;
  final DateTime? expiresAt;
  final int escrowAmount;
  final int coinsToUse;
  final int cashAmount;
  final String currency;
  final List<PreOrderPaymentMethodOption> methods;

  const PreOrderPaymentPricing({
    required this.pricingToken,
    required this.expiresAt,
    required this.escrowAmount,
    required this.coinsToUse,
    required this.cashAmount,
    required this.currency,
    required this.methods,
  });

  /// The selected method option, or null when none is selected/available.
  PreOrderPaymentMethodOption? optionFor(String? methodCode) {
    if (methodCode == null) return null;
    for (final m in methods) {
      if (m.methodCode == methodCode) return m;
    }
    return null;
  }
}
