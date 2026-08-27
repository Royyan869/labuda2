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

/// Payment channel - Specific Midtrans payment methods
/// Maps to Midtrans payment types for fee calculation and API calls
enum PaymentChannel {
  // ========== Bank Transfer / Virtual Account ==========
  /// BCA Virtual Account
  bcaVa('bca_va', PaymentMethodType.bankTransfer),

  /// BNI Virtual Account
  bniVa('bni_va', PaymentMethodType.bankTransfer),

  /// BRI Virtual Account
  briVa('bri_va', PaymentMethodType.bankTransfer),

  /// Mandiri Virtual Account (echannel)
  mandiriVa('echannel', PaymentMethodType.bankTransfer),

  /// Permata Virtual Account
  permataVa('permata_va', PaymentMethodType.bankTransfer),

  /// CIMB Niaga Virtual Account
  cimbVa('cimb_va', PaymentMethodType.bankTransfer),

  /// BSI (Bank Syariah Indonesia) Virtual Account
  bsiVa('bsi_va', PaymentMethodType.bankTransfer),

  /// Danamon Virtual Account
  danamonVa('danamon_va', PaymentMethodType.bankTransfer),

  /// Maybank Virtual Account
  maybankVa('maybank_va', PaymentMethodType.bankTransfer),

  /// BTN Virtual Account
  btnVa('btn_va', PaymentMethodType.bankTransfer),

  // ========== E-Wallet ==========
  /// GoPay (Gojek payment)
  gopay('gopay', PaymentMethodType.eWallet),

  /// ShopeePay
  shopeepay('shopeepay', PaymentMethodType.eWallet),

  /// DANA
  dana('dana', PaymentMethodType.eWallet),

  /// OVO
  ovo('ovo', PaymentMethodType.eWallet),

  /// LinkAja
  linkAja('linkaja', PaymentMethodType.eWallet),

  // ========== QRIS ==========
  /// QRIS - Universal QR payment
  qris('qris', PaymentMethodType.qris),

  // ========== Card ==========
  /// Credit Card (Visa, Mastercard, JCB, Amex)
  creditCard('credit_card', PaymentMethodType.creditCard),

  /// Debit Card
  debitCard('debit_card', PaymentMethodType.debitCard),

  // ========== Other ==========
  /// Akulaku PayLater
  akulaku('akulaku', PaymentMethodType.eWallet),

  /// Kredivo
  kredivo('kredivo', PaymentMethodType.eWallet),

  /// Alfamart (convenience store)
  alfamart('alfamart', PaymentMethodType.manualTransfer),

  /// Indomaret (convenience store)
  indomaret('indomaret', PaymentMethodType.manualTransfer);

  /// Midtrans payment type key
  final String midtransKey;

  /// High-level payment method type
  final PaymentMethodType type;

  const PaymentChannel(this.midtransKey, this.type);

  /// Get PaymentChannel from Midtrans key
  /// Returns null if key is not found
  static PaymentChannel? fromMidtransKey(String? key) {
    if (key == null) return null;

    for (final channel in PaymentChannel.values) {
      if (channel.midtransKey == key) {
        return channel;
      }
    }

    return null;
  }

  /// Get PaymentChannel from enum name string
  /// Used for Firestore deserialization
  static PaymentChannel? fromName(String? name) {
    if (name == null) return null;

    try {
      return PaymentChannel.values.firstWhere(
        (channel) => channel.name == name,
      );
    } catch (_) {
      return null;
    }
  }

  /// Check if this channel supports deep linking
  /// E-wallets typically support deep links to open their apps
  bool get supportsDeepLink {
    return type == PaymentMethodType.eWallet &&
        (this == PaymentChannel.gopay || this == PaymentChannel.shopeepay);
  }

  /// Check if this channel is instant payment
  /// E-wallets and QRIS are typically instant
  bool get isInstantPayment {
    return type == PaymentMethodType.eWallet || type == PaymentMethodType.qris;
  }

  /// Check if this channel requires manual verification
  /// Bank transfer and convenience stores need manual check
  bool get requiresManualVerification {
    return this == PaymentChannel.alfamart || this == PaymentChannel.indomaret;
  }

  /// Get display name for UI
  String get displayName {
    switch (this) {
      case PaymentChannel.bcaVa:
        return 'BCA Virtual Account';
      case PaymentChannel.bniVa:
        return 'BNI Virtual Account';
      case PaymentChannel.briVa:
        return 'BRI Virtual Account';
      case PaymentChannel.mandiriVa:
        return 'Mandiri Virtual Account';
      case PaymentChannel.permataVa:
        return 'Permata Virtual Account';
      case PaymentChannel.cimbVa:
        return 'CIMB Niaga Virtual Account';
      case PaymentChannel.bsiVa:
        return 'BSI (Bank Syariah Indonesia)';
      case PaymentChannel.danamonVa:
        return 'Danamon Virtual Account';
      case PaymentChannel.maybankVa:
        return 'Maybank Virtual Account';
      case PaymentChannel.btnVa:
        return 'BTN Virtual Account';
      case PaymentChannel.gopay:
        return 'GoPay';
      case PaymentChannel.shopeepay:
        return 'ShopeePay';
      case PaymentChannel.dana:
        return 'DANA';
      case PaymentChannel.ovo:
        return 'OVO';
      case PaymentChannel.linkAja:
        return 'LinkAja';
      case PaymentChannel.qris:
        return 'QRIS';
      case PaymentChannel.creditCard:
        return 'Kartu Kredit';
      case PaymentChannel.debitCard:
        return 'Kartu Debit';
      case PaymentChannel.akulaku:
        return 'Akulaku PayLater';
      case PaymentChannel.kredivo:
        return 'Kredivo';
      case PaymentChannel.alfamart:
        return 'Alfamart';
      case PaymentChannel.indomaret:
        return 'Indomaret';
    }
  }

  /// Get bank type for Midtrans VA API
  /// Only applicable for bank transfer channels
  String? get bankType {
    switch (this) {
      case PaymentChannel.bcaVa:
        return 'bca';
      case PaymentChannel.bniVa:
        return 'bni';
      case PaymentChannel.briVa:
        return 'bri';
      case PaymentChannel.mandiriVa:
        return 'echannel';
      case PaymentChannel.permataVa:
        return 'permata';
      case PaymentChannel.cimbVa:
        return 'cimb';
      case PaymentChannel.bsiVa:
        return 'bsi';
      case PaymentChannel.danamonVa:
        return 'danamon';
      case PaymentChannel.maybankVa:
        return 'maybank';
      case PaymentChannel.btnVa:
        return 'btn';
      default:
        return null;
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
