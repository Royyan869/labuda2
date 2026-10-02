/// Preparation Time - Shipping Readiness for Koi Business
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// BUSINESS TRUTH
/// ═══════════════════════════════════════════════════════════════════════════════
/// Preparation time is WHEN THE SELLER CAN SHIP AFTER CHECKOUT. In the koi
/// business, sellers may need time before shipping due to:
/// - Karantina (quarantine) requirements
/// - Puasa (fasting) before transport
/// - Observasi (observation) for health
/// - Stabilisasi (stabilization) after handling
///
/// Buyers understand and accept this when expectation is clear UP FRONT.
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// CANONICAL VOCABULARY (owner decision 2026-10-02 — exactly 3 ranges)
/// ═══════════════════════════════════════════════════════════════════════════════
/// 1–3 hari (default) | 4–7 hari | 8–15 hari
/// Wire/DB values: `1_3_days` | `4_7_days` | `8_15_days`
/// The range is the MAXIMUM time (upper bound) the seller needs; the order
/// fulfillment deadline maps to that upper bound (3 / 7 / 15 days).
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// THIS IS AN EXPECTATION LAYER, NOT A FULFILLMENT STATE MACHINE
/// ═══════════════════════════════════════════════════════════════════════════════
/// - These values are BUYER EXPECTATIONS set before purchase
/// - Order gets a SNAPSHOT of these values at creation time
/// - Seller changing forSale/auction preparation time later does NOT affect existing orders
/// ═══════════════════════════════════════════════════════════════════════════════
library;

/// Preparation time for shipping - domain-native to koi/live animal business
enum PreparationTime {
  /// 1-3 days preparation (DEFAULT)
  /// Display: "1–3 hari"
  days1_3,

  /// 4-7 days preparation
  /// Display: "4–7 hari"
  days4_7,

  /// 8-15 days preparation
  /// Display: "8–15 hari"
  days8_15;

  /// Backend JSON value
  String toJson() {
    switch (this) {
      case PreparationTime.days1_3:
        return '1_3_days';
      case PreparationTime.days4_7:
        return '4_7_days';
      case PreparationTime.days8_15:
        return '8_15_days';
    }
  }

  /// Parse from backend JSON value
  /// Defaults to `days1_3` for unknown values (owner-set default 1–3 hari)
  static PreparationTime fromJson(String? value) {
    switch (value?.toLowerCase()) {
      case '4_7_days':
        return PreparationTime.days4_7;
      case '8_15_days':
        return PreparationTime.days8_15;
      case '1_3_days':
        return PreparationTime.days1_3;
      default:
        return PreparationTime.days1_3; // Safe default: 1–3 hari
    }
  }

  /// User-facing display label in Indonesian
  String get displayName {
    switch (this) {
      case PreparationTime.days1_3:
        return '1–3 hari';
      case PreparationTime.days4_7:
        return '4–7 hari';
      case PreparationTime.days8_15:
        return '8–15 hari';
    }
  }

  /// Description explaining what this means to buyers
  String get description {
    switch (this) {
      case PreparationTime.days1_3:
        return 'Penjual perlu 1–3 hari untuk menyiapkan ikan setelah pembayaran';
      case PreparationTime.days4_7:
        return 'Penjual perlu 4–7 hari untuk karantina/persiapan ikan';
      case PreparationTime.days8_15:
        return 'Penjual perlu 8–15 hari untuk karantina/stabilisasi ikan';
    }
  }

  /// Preparation days for calculation (ready_to_ship_by) — the UPPER bound of
  /// the promised range: 1-3 → 3, 4-7 → 7, 8-15 → 15.
  int get days {
    switch (this) {
      case PreparationTime.days1_3:
        return 3;
      case PreparationTime.days4_7:
        return 7;
      case PreparationTime.days8_15:
        return 15;
    }
  }
}

/// Extension for PreparationTime with additional utilities
extension PreparationTimeX on PreparationTime {
  /// Short label for UI chips
  String get shortLabel {
    switch (this) {
      case PreparationTime.days1_3:
        return '1-3 hari';
      case PreparationTime.days4_7:
        return '4-7 hari';
      case PreparationTime.days8_15:
        return '8-15 hari';
    }
  }

  /// Badge color hint for UI
  String get colorVariant {
    switch (this) {
      case PreparationTime.days1_3:
        return 'success'; // Green (default, fastest)
      case PreparationTime.days4_7:
        return 'info'; // Blue
      case PreparationTime.days8_15:
        return 'warning'; // Orange/Yellow (longest)
    }
  }
}
