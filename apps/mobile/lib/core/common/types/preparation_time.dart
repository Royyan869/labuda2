/// Preparation Time - Shipping Readiness for Koi Business
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// BUSINESS TRUTH
/// ═══════════════════════════════════════════════════════════════════════════════
/// In the koi business, sellers may need time before shipping due to:
/// - Karantina (quarantine) requirements
/// - Puasa (fasting) before transport
/// - Observasi (observation) for health
/// - Stabilisasi (stabilization) after handling
///
/// Buyers understand and accept this when expectation is clear UP FRONT.
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// THIS IS AN EXPECTATION LAYER, NOT A FULFILLMENT STATE MACHINE
/// ═══════════════════════════════════════════════════════════════════════════════
/// - These values are BUYER EXPECTATIONS set before purchase
/// - Order gets a SNAPSHOT of these values at creation time
/// - Seller changing listing/auction preparation time later does NOT affect existing orders
/// ═══════════════════════════════════════════════════════════════════════════════
library;

/// Preparation time for shipping - domain-native to koi/live animal business
enum PreparationTime {
  /// Ready to ship immediately (0 days)
  /// Display: "Siap kirim langsung"
  immediate,

  /// 1-2 days preparation
  /// Display: "1–2 hari"
  short,

  /// 3-5 days preparation
  /// Display: "3–5 hari"
  medium,

  /// 7+ days preparation
  /// Display: "7+ hari"
  long;

  /// Backend JSON value
  String toJson() {
    switch (this) {
      case PreparationTime.immediate:
        return 'immediate';
      case PreparationTime.short:
        return 'short';
      case PreparationTime.medium:
        return 'medium';
      case PreparationTime.long:
        return 'long';
    }
  }

  /// Parse from backend JSON value
  /// Defaults to `immediate` for unknown values (safe default)
  static PreparationTime fromJson(String? value) {
    switch (value?.toLowerCase()) {
      case 'immediate':
        return PreparationTime.immediate;
      case 'short':
        return PreparationTime.short;
      case 'medium':
        return PreparationTime.medium;
      case 'long':
        return PreparationTime.long;
      default:
        return PreparationTime.immediate; // Safe default
    }
  }

  /// User-facing display label in Indonesian
  String get displayName {
    switch (this) {
      case PreparationTime.immediate:
        return 'Siap kirim langsung';
      case PreparationTime.short:
        return '1–2 hari';
      case PreparationTime.medium:
        return '3–5 hari';
      case PreparationTime.long:
        return '7+ hari';
    }
  }

  /// Description explaining what this means to buyers
  String get description {
    switch (this) {
      case PreparationTime.immediate:
        return 'Penjual siap mengirim ikan segera setelah pembayaran';
      case PreparationTime.short:
        return 'Penjual mungkin perlu 1–2 hari untuk persiapan pengiriman';
      case PreparationTime.medium:
        return 'Penjual mungkin perlu 3–5 hari untuk karantina/persiapan ikan';
      case PreparationTime.long:
        return 'Penjual mungkin perlu 7+ hari untuk karantina/stabilisasi ikan';
    }
  }

  /// Preparation days for calculation (ready_to_ship_by)
  /// immediate = 0, short = 2, medium = 5, long = 7
  int get days {
    switch (this) {
      case PreparationTime.immediate:
        return 0;
      case PreparationTime.short:
        return 2;
      case PreparationTime.medium:
        return 5;
      case PreparationTime.long:
        return 7;
    }
  }

  /// Check if this is immediate (ready to ship right away)
  bool get isImmediate => this == PreparationTime.immediate;

  /// Check if this requires preparation time
  bool get requiresPreparation => this != PreparationTime.immediate;
}

/// Extension for PreparationTime with additional utilities
extension PreparationTimeX on PreparationTime {
  /// Short label for UI chips
  String get shortLabel {
    switch (this) {
      case PreparationTime.immediate:
        return 'Siap kirim';
      case PreparationTime.short:
        return '1-2 hari';
      case PreparationTime.medium:
        return '3-5 hari';
      case PreparationTime.long:
        return '7+ hari';
    }
  }

  /// Badge color hint for UI
  String get colorVariant {
    switch (this) {
      case PreparationTime.immediate:
        return 'success'; // Green
      case PreparationTime.short:
        return 'info'; // Blue
      case PreparationTime.medium:
        return 'warning'; // Orange/Yellow
      case PreparationTime.long:
        return 'neutral'; // Gray
    }
  }
}
