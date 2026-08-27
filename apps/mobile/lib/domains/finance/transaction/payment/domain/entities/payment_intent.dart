/// Payment Intent Entity
///
/// Pure Dart entity for payment intent.
/// Represents a payment that is being initiated.
library;

import 'package:equatable/equatable.dart';

/// Payment intent entity
class PaymentIntent extends Equatable {
  /// Unique intent ID
  final String id;

  /// Human-readable payment number
  final String paymentNumber;

  /// Amount to pay
  final double amount;

  /// Currency code (default: IDR)
  final String currency;

  /// Payment status
  final String status;

  /// Payment URL (for redirect-based payments)
  final String? paymentUrl;

  /// Deep link for mobile payment (e.g., gopay://)
  final String? deepLinkUrl;

  /// VA number (for virtual account payments)
  final String? vaNumber;

  /// VA bank name
  final String? vaBank;

  /// QR code string (for QRIS)
  final String? qrString;

  /// Expiry time
  final DateTime? expiresAt;

  const PaymentIntent({
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

  /// Check if payment intent is still valid
  bool get isValid {
    if (status == 'completed' || status == 'failed') return false;
    if (expiresAt == null) return true;
    return DateTime.now().isBefore(expiresAt!);
  }

  /// Check if payment requires redirect
  bool get requiresRedirect => paymentUrl != null && paymentUrl!.isNotEmpty;

  /// Check if payment requires deep link
  bool get requiresDeepLink => deepLinkUrl != null && deepLinkUrl!.isNotEmpty;

  /// Check if payment has VA details
  bool get hasVaDetails => vaNumber != null && vaNumber!.isNotEmpty;

  /// Check if payment has QR code
  bool get hasQrCode => qrString != null && qrString!.isNotEmpty;

  /// Get payment type for display
  String get paymentType {
    if (hasVaDetails) return 'Virtual Account';
    if (hasQrCode) return 'QRIS';
    if (requiresDeepLink) return 'E-Wallet';
    if (requiresRedirect) return 'Online Payment';
    return 'Other';
  }

  @override
  List<Object?> get props => [
    id,
    paymentNumber,
    amount,
    currency,
    status,
    paymentUrl,
    deepLinkUrl,
    vaNumber,
    vaBank,
    qrString,
    expiresAt,
  ];
}
