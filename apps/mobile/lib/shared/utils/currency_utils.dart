library;

import 'package:labuda/shared/domain/entities/resource_projection.dart';

/// Centralized Currency Formatting Utility
///
/// This is the SINGLE DISPLAY AUTHORITY for Rupiah strings. It no longer
/// owns a second formatting engine: every grouping below delegates to
/// [formatGroupedAmount] (the canonical envelope loop), so seller/wallet
/// screens and chat/discovery cards cannot drift apart. `intl` is kept out
/// of display entirely — it survives only in `currency_input_formatter.dart`,
/// which is an input mask (groups while the user types), not a display
/// formatter.
///
/// Usage:
/// ```dart
/// import 'package:labuda/shared/utils/currency_utils.dart';
///
/// // Standard format: Rp 1.000.000
/// CurrencyUtils.format(1000000);
///
/// // Shorthand for dashboards: Rp 1.5Jt, Rp 2.3M, Rp 500K
/// CurrencyUtils.formatShorthand(1500000);
///
/// // Number only without prefix: 1.000.000
/// CurrencyUtils.formatNumber(1000000);
/// ```
class CurrencyUtils {
  CurrencyUtils._();

  /// Standard currency format with Rupiah symbol.
  ///
  /// Example: 1000000 -> "Rp 1.000.000"; -1500 -> "-Rp 1.500" (the minus
  /// stays ahead of the symbol, exactly as the ICU engine emitted it).
  static String format(double amount) => _rupiah(amount.round());

  /// Format from int value.
  ///
  /// Example: 1000000 -> "Rp 1.000.000"
  static String formatInt(int amount) => _rupiah(amount);

  /// The only place a 'Rp ' prefix is attached to a number outside the
  /// canonical envelope. The grouping itself is [formatGroupedAmount].
  static String _rupiah(int amount) => amount < 0
      ? '-Rp ${formatGroupedAmount(-amount)}'
      : 'Rp ${formatGroupedAmount(amount)}';

  /// Shorthand format for compact display (dashboards, cards)
  ///
  /// Examples:
  /// - 2300000000 -> "Rp 2.3M" (Miliar)
  /// - 1500000 -> "Rp 1.5Jt" (Juta)
  /// - 500000 -> "Rp 500K" (Ribu)
  /// - 50000 -> "Rp 50.000" (standard format for small amounts)
  static String formatShorthand(double amount) {
    if (amount >= 1000000000) {
      final value = amount / 1000000000;
      return 'Rp ${_formatDecimal(value)}M';
    } else if (amount >= 1000000) {
      final value = amount / 1000000;
      return 'Rp ${_formatDecimal(value)}Jt';
    } else if (amount >= 100000) {
      final value = amount / 1000;
      return 'Rp ${value.toStringAsFixed(0)}K';
    }
    return format(amount);
  }

  /// Format decimal for shorthand (removes trailing .0)
  static String _formatDecimal(double value) {
    if (value == value.roundToDouble()) {
      return value.toStringAsFixed(0);
    }
    return value.toStringAsFixed(1);
  }

  /// Number format without currency symbol.
  ///
  /// Example: 1000000 -> "1.000.000"
  static String formatNumber(double amount) =>
      formatGroupedAmount(amount.round());

  /// Number format from int without currency symbol.
  ///
  /// Example: 1000000 -> "1.000.000"
  static String formatNumberInt(int amount) => formatGroupedAmount(amount);

  /// Parse formatted currency string back to double
  ///
  /// Handles various formats:
  /// - "Rp 1.000.000" -> 1000000.0
  /// - "1.000.000" -> 1000000.0
  /// - "Rp1000000" -> 1000000.0
  ///
  /// Returns null if parsing fails
  static double? parse(String formatted) {
    if (formatted.isEmpty) return null;

    try {
      final cleaned = formatted
          .replaceAll('Rp', '')
          .replaceAll(' ', '')
          .replaceAll('.', '')
          .replaceAll(',', '.')
          .trim();

      return double.tryParse(cleaned);
    } catch (_) {
      return null;
    }
  }

  /// Parse formatted currency string back to int
  ///
  /// Returns null if parsing fails
  static int? parseInt(String formatted) {
    final result = parse(formatted);
    return result?.toInt();
  }
}
