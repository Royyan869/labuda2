/// Payment type enumerations - Single source of truth
/// All payment-related enums should use these types
library;

/// Payment method type - High-level payment categories
enum PaymentMethodType {
  /// Bank transfer (VA, manual transfer)
  bankTransfer,

  /// Credit card
  creditCard,

  /// Debit card
  debitCard,

  /// E-Wallet (GoPay, ShopeePay, DANA, OVO)
  eWallet,

  /// QRIS (Quick Response Code Indonesian Standard)
  qris,

  /// Cash on Delivery
  cod,

  /// Manual bank transfer (not VA)
  manualTransfer,

  /// Coins (loyalty points - NOT real money)
  /// Used for order flow to indicate coin usage for discounts
  coins,
}

/// Payment status for orders
///
/// PHASE 1F: Unified PaymentStatus across entire codebase.
/// This is the SINGLE SOURCE OF TRUTH for payment status.
/// All payment-related features MUST use this enum.
enum PaymentStatus {
  /// Payment is pending/awaiting
  pending,

  /// Payment is being processed
  processing,

  /// Payment completed successfully
  paid,

  /// Payment failed
  failed,

  /// Payment deadline expired
  expired,

  /// Payment refunded
  refunded;

  /// Parse PaymentStatus from string value.
  ///
  /// Includes legacy support for removed/mapped values:
  /// - 'completed' → paid (legacy mapping)
  /// - 'settlement' → paid (backend Midtrans status)
  /// - 'capture' → paid (backend Midtrans status)
  static PaymentStatus fromString(String value) {
    // First try direct match
    for (final status in PaymentStatus.values) {
      if (status.name == value) {
        return status;
      }
    }

    // Legacy fallback for removed/mapped statuses
    switch (value.toLowerCase()) {
      case 'completed':
      case 'settlement':
      case 'capture':
        return PaymentStatus.paid;
      case 'processing':
      case 'process':
      case 'challenge':
        return PaymentStatus.processing;
      case 'deny':
      case 'cancel':
      case 'cancelled':
      case 'failed':
        return PaymentStatus.failed;
      case 'expire':
      case 'expired':
        return PaymentStatus.expired;
      default:
        return PaymentStatus.pending;
    }
  }
}

/// Extension methods for PaymentMethodType
extension PaymentMethodTypeExtension on PaymentMethodType {
  /// Get display name for UI
  String get displayName {
    switch (this) {
      case PaymentMethodType.bankTransfer:
        return 'Transfer Bank';
      case PaymentMethodType.creditCard:
        return 'Kartu Kredit';
      case PaymentMethodType.debitCard:
        return 'Kartu Debit';
      case PaymentMethodType.eWallet:
        return 'E-Wallet';
      case PaymentMethodType.qris:
        return 'QRIS';
      case PaymentMethodType.cod:
        return 'Bayar di Tempat';
      case PaymentMethodType.manualTransfer:
        return 'Transfer Manual';
      case PaymentMethodType.coins:
        return 'Koin Loyalti';
    }
  }
}

/// Extension methods for PaymentStatus
extension PaymentStatusExtension on PaymentStatus {
  /// Get display name for UI
  String get displayName {
    switch (this) {
      case PaymentStatus.pending:
        return 'Menunggu Pembayaran';
      case PaymentStatus.processing:
        return 'Memproses';
      case PaymentStatus.paid:
        return 'Lunas';
      case PaymentStatus.failed:
        return 'Gagal';
      case PaymentStatus.expired:
        return 'Kedaluwarsa';
      case PaymentStatus.refunded:
        return 'Dikembalikan';
    }
  }

  /// Check if this is a terminal status (no more state changes expected)
  bool get isTerminal {
    return this == PaymentStatus.paid ||
        this == PaymentStatus.failed ||
        this == PaymentStatus.expired ||
        this == PaymentStatus.refunded;
  }

  /// Check if payment is successful
  bool get isSuccessful {
    return this == PaymentStatus.paid;
  }

  /// Check if payment is still ongoing
  bool get isOngoing {
    return this == PaymentStatus.pending || this == PaymentStatus.processing;
  }

  /// PHASE 1F: Alias for isSuccessful for backward compatibility
  /// Some code uses isSuccess, some uses isSuccessful
  bool get isSuccess => isSuccessful;

  /// PHASE 1F: Alias for isTerminal for backward compatibility
  /// Some code uses isFinal, some uses isTerminal
  bool get isFinal => isTerminal;
}
