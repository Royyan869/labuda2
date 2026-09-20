/// Payment Intent Entity
///
/// Canonical model of the `POST /api/v1/payments` response.
///
/// WIRE AUTHORITY: backend/internal/serverboot/dependencies.go
///   func (h *CorePaymentHandler) CreatePayment(c *gin.Context)
///     response.Success(c, gin.H{
///         "payment_id", "payment_number", "status", "payment_url",
///         "payment_method_code", "buyer_payment_fee_amount", "gross_amount",
///         "coins_to_use", "coin_discount_amount", "reference_type",
///         "reference_id", "expired_at",
///     })
///
/// Every field below maps 1:1 to a key that handler emits. Nothing here is
/// invented for Flutter's convenience: there is no `amount`, `currency`, or
/// `net_amount` on this wire, and no aliases are kept for them.
library;

import 'package:equatable/equatable.dart';

/// Payment intent entity — the buyer's created (or reused) payment attempt.
class PaymentIntent extends Equatable {
  /// `payment_id` — UUID of the payments row.
  final String paymentId;

  /// `payment_number` — human-readable payment number (e.g. "PAY-1706123456").
  final String paymentNumber;

  /// `status` — raw backend payment status (`payment_status_enum`).
  final String status;

  /// `payment_url` — Midtrans Snap redirect URL, presented in-app via
  /// PaymentWebviewScreen. Nullable on the wire (the reuse branch returns the
  /// stored value, which can still be empty if Snap never answered).
  final String? paymentUrl;

  /// `payment_method_code` — canonical Labuda payment method the buyer chose.
  final String? paymentMethodCode;

  /// `buyer_payment_fee_amount` — backend-calculated gateway fee (Rupiah).
  final int buyerPaymentFeeAmount;

  /// `gross_amount` — cash after coin deduction plus the buyer fee (Rupiah).
  final int grossAmount;

  /// `coins_to_use` — loyalty coins redeemed on this payment (count, not money).
  final int coinsToUse;

  /// `coin_discount_amount` — Rupiah value of [coinsToUse].
  final int coinDiscountAmount;

  /// `reference_type` — "order" | "billing" | "subscription".
  final String referenceType;

  /// `reference_id` — UUID of the referenced entity.
  final String? referenceId;

  /// `expired_at` — end of the payment window (RFC3339).
  final DateTime? expiredAt;

  const PaymentIntent({
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

  /// Whether the backend handed us a payment URL to present in the WebView.
  bool get requiresRedirect => paymentUrl != null && paymentUrl!.isNotEmpty;

  /// Whether the payment window has already closed.
  bool get isExpired {
    final expiry = expiredAt;
    if (expiry == null) return false;
    return !DateTime.now().isBefore(expiry);
  }

  @override
  List<Object?> get props => [
    paymentId,
    paymentNumber,
    status,
    paymentUrl,
    paymentMethodCode,
    buyerPaymentFeeAmount,
    grossAmount,
    coinsToUse,
    coinDiscountAmount,
    referenceType,
    referenceId,
    expiredAt,
  ];
}
