library;

import 'package:intl/intl.dart';

/// Centralized Currency Formatting Utility
///
/// This is the SINGLE SOURCE OF TRUTH for all currency formatting in the app.
/// All modules should use this utility instead of creating their own formatting logic.
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

  static final _currencyFormat = NumberFormat.currency(
    locale: 'id_ID',
    symbol: 'Rp ',
    decimalDigits: 0,
  );

  static final _numberFormat = NumberFormat('#,##0', 'id_ID');

  /// Standard currency format with Rupiah symbol
  ///
  /// Example: 1000000 -> "Rp 1.000.000"
  static String format(double amount) {
    return _currencyFormat.format(amount);
  }

  /// Format from int value
  ///
  /// Example: 1000000 -> "Rp 1.000.000"
  static String formatInt(int amount) {
    return _currencyFormat.format(amount);
  }

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

  /// Number format without currency symbol
  ///
  /// Example: 1000000 -> "1.000.000"
  static String formatNumber(double amount) {
    return _numberFormat.format(amount);
  }

  /// Number format from int without currency symbol
  ///
  /// Example: 1000000 -> "1.000.000"
  static String formatNumberInt(int amount) {
    return _numberFormat.format(amount);
  }

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
