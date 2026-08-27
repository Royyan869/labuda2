/// Payment Method Entity
///
/// Pure Dart entity for payment methods.
/// Uses core.PaymentChannel from core module (enum only).
library;

import 'package:labuda/core/core.dart' as core;

// ============================================================
// BACKEND IS AUTHORITY FOR CHECKOUT FEES
// ============================================================
// This entity keeps legacy metadata only.
// Checkout fee calculations must come from backend snapshots.

/// Payment method category
///
/// Financial compliance: 'balance' removed.
/// Labuda is not a stored-value wallet/e-money app.
enum PaymentMethodCategory {
  bankTransfer('Bank Transfer'),
  eWallet('E-Wallet'),
  qris('QRIS'),
  card('Kartu'),
  payLater('PayLater'),
  convenienceStore('Minimarket');

  final String displayName;
  const PaymentMethodCategory(this.displayName);
}

/// Payment method entity
class PaymentMethod {
  /// The payment channel (from core)
  final core.PaymentChannel channel;

  /// Display name for the payment method
  final String displayName;

  /// Category of the payment method
  final PaymentMethodCategory category;

  /// Legacy fee placeholder.
  ///
  /// Retained only to avoid wide API churn. Checkout pricing is backend-owned.
  final PaymentMethodFee fee;

  /// Whether this method is currently available
  final bool isAvailable;

  /// Whether this method is coming soon
  final bool isComingSoon;

  /// Icon asset path (optional)
  final String? iconAsset;

  /// Deep link scheme for mobile apps (e.g., gopay://)
  final String? deepLinkScheme;

  const PaymentMethod({
    required this.channel,
    required this.displayName,
    required this.category,
    required this.fee,
    this.isAvailable = true,
    this.isComingSoon = false,
    this.iconAsset,
    this.deepLinkScheme,
  });

  /// Check if this method supports deep link payment.
  bool get supportsDeepLink => deepLinkScheme != null;

  /// Legacy no-op: backend snapshots own checkout fee authority now.
  double calculateFee(double amount) => 0.0;

  /// Legacy no-op: backend snapshots own checkout fee authority now.
  double calculateTotal(double amount) => amount;

  /// Create a copy with modified fields
  PaymentMethod copyWith({
    core.PaymentChannel? channel,
    String? displayName,
    PaymentMethodCategory? category,
    PaymentMethodFee? fee,
    bool? isAvailable,
    bool? isComingSoon,
    String? iconAsset,
    String? deepLinkScheme,
  }) {
    return PaymentMethod(
      channel: channel ?? this.channel,
      displayName: displayName ?? this.displayName,
      category: category ?? this.category,
      fee: fee ?? this.fee,
      isAvailable: isAvailable ?? this.isAvailable,
      isComingSoon: isComingSoon ?? this.isComingSoon,
      iconAsset: iconAsset ?? this.iconAsset,
      deepLinkScheme: deepLinkScheme ?? this.deepLinkScheme,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is PaymentMethod && other.channel == channel;

  @override
  int get hashCode => channel.hashCode;
}

/// Payment method fee structure.
///
/// Legacy data container only.
class PaymentMethodFee {
  /// Fee type
  final PaymentFeeType type;

  /// Flat fee amount (in Rupiah)
  final double flatFee;

  /// Percentage fee (e.g., 2.0 for 2%)
  final double percentageFee;

  /// Minimum fee (floor)
  final double? minFee;

  /// Maximum fee (ceiling)
  final double? maxFee;

  const PaymentMethodFee({
    required this.type,
    this.flatFee = 0,
    this.percentageFee = 0,
    this.minFee,
    this.maxFee,
  });

  /// Create flat fee structure
  const PaymentMethodFee.flat(double amount)
    : type = PaymentFeeType.flat,
      flatFee = amount,
      percentageFee = 0,
      minFee = null,
      maxFee = null;

  /// Create percentage fee structure
  const PaymentMethodFee.percentage(double percent, {double? min, double? max})
    : type = PaymentFeeType.percentage,
      flatFee = 0,
      percentageFee = percent,
      minFee = min,
      maxFee = max;

  /// Create combined fee structure (flat + percentage)
  const PaymentMethodFee.combined({
    required double flat,
    required double percent,
    double? min,
    double? max,
  }) : type = PaymentFeeType.combined,
       flatFee = flat,
       percentageFee = percent,
       minFee = min,
       maxFee = max;

  /// Legacy no-op: backend snapshots own checkout fee authority now.
  double calculateFee(double amount) => 0.0;

  /// Legacy no-op: backend snapshots own checkout fee authority now.
  double calculateTotal(double amount) => amount;

  /// Get display string for fee metadata.
  String get displayString {
    switch (type) {
      case PaymentFeeType.flat:
        return 'Rp ${_formatNumber(flatFee.toInt())}';
      case PaymentFeeType.percentage:
        return '${percentageFee.toStringAsFixed(1)}%';
      case PaymentFeeType.combined:
        return '${percentageFee.toStringAsFixed(1)}% + Rp ${_formatNumber(flatFee.toInt())}';
    }
  }

  String _formatNumber(int number) {
    return number.toString().replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match m) => '${m[1]}.',
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is PaymentMethodFee &&
          other.type == type &&
          other.flatFee == flatFee &&
          other.percentageFee == percentageFee &&
          other.minFee == minFee &&
          other.maxFee == maxFee;

  @override
  int get hashCode =>
      type.hashCode ^
      flatFee.hashCode ^
      percentageFee.hashCode ^
      minFee.hashCode ^
      maxFee.hashCode;
}

/// Fee type enum
enum PaymentFeeType { flat, percentage, combined }
