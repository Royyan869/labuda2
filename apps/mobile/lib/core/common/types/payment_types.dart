/// Payment type enumerations - Single source of truth
/// All payment-related enums should use these types
library;

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

  /// Parse a wire value into a [PaymentStatus].
  ///
  /// The vocabulary IS the enum names: the backend owns payment state and
  /// never puts gateway vocabulary on this wire (`midtrans_status` is a
  /// forbidden response key). Anything else is a contract violation, and it is
  /// rejected loudly instead of coerced — a settled payment that silently reads
  /// as `pending` is a money-safety lie.
  static PaymentStatus fromString(String value) {
    final normalized = value.trim().toLowerCase();
    for (final status in PaymentStatus.values) {
      if (status.name == normalized) return status;
    }
    throw FormatException('Unknown payment status from wire: "$value"');
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

  /// Check if payment is still ongoing
  bool get isOngoing {
    return this == PaymentStatus.pending || this == PaymentStatus.processing;
  }
}
