library;

import 'package:intl/intl.dart';
import 'package:labuda/shared/utils/currency_utils.dart';

/// Centralized Formatting Utility
///
/// This is the SINGLE SOURCE OF TRUTH for all formatting in the app.
/// All modules should use this utility for consistent formatting.
///
/// Usage:
/// ```dart
/// import 'package:labuda/shared/utils/app_formatters.dart';
///
/// // Currency format: Rp 1.000.000
/// AppFormatters.formatCurrency(1000000);
///
/// // Date format: 15 Jan 2024
/// AppFormatters.formatDate(DateTime.now());
///
/// // Date with time: 15 Jan 2024, 14:30
/// AppFormatters.formatDateTime(DateTime.now());
/// ```
class AppFormatters {
  AppFormatters._();

  // Date formatters
  static final _dateFormat = DateFormat('dd MMM yyyy', 'id_ID');
  static final _dateTimeFormat = DateFormat('dd MMM yyyy, HH:mm', 'id_ID');
  static final _shortDateFormat = DateFormat('dd/MM/yyyy', 'id_ID');
  static final _timeFormat = DateFormat('HH:mm', 'id_ID');

  /// Format currency with Rupiah symbol
  ///
  /// Example: 1000000 -> "Rp 1.000.000"
  static String formatCurrency(double amount) {
    return CurrencyUtils.format(amount);
  }

  /// Format currency from int with Rupiah symbol
  ///
  /// Example: 1000000 -> "Rp 1.000.000"
  static String formatCurrencyInt(int amount) {
    return CurrencyUtils.formatInt(amount);
  }

  /// Format currency in shorthand for compact display
  ///
  /// Examples:
  /// - 2300000000 -> "Rp 2.3M" (Miliar)
  /// - 1500000 -> "Rp 1.5Jt" (Juta)
  /// - 500000 -> "Rp 500K" (Ribu)
  static String formatCurrencyShorthand(double amount) {
    return CurrencyUtils.formatShorthand(amount);
  }

  /// Format date in readable format
  ///
  /// Example: DateTime(2024, 1, 15) -> "15 Jan 2024"
  static String formatDate(DateTime date) {
    return _dateFormat.format(date);
  }

  /// Format date with time
  ///
  /// Example: DateTime(2024, 1, 15, 14, 30) -> "15 Jan 2024, 14:30"
  static String formatDateTime(DateTime dateTime) {
    return _dateTimeFormat.format(dateTime);
  }

  /// Format date in short format
  ///
  /// Example: DateTime(2024, 1, 15) -> "15/01/2024"
  static String formatShortDate(DateTime date) {
    return _shortDateFormat.format(date);
  }

  /// Format time only
  ///
  /// Example: DateTime(2024, 1, 15, 14, 30) -> "14:30"
  static String formatTime(DateTime dateTime) {
    return _timeFormat.format(dateTime);
  }

  /// Format relative time (e.g., "2 days ago", "3 hours ago")
  ///
  /// Example:
  /// - 2 hours ago -> "2 jam yang lalu"
  /// - 1 day ago -> "1 hari yang lalu"
  static String formatRelativeTime(DateTime dateTime) {
    final now = DateTime.now();
    final difference = now.difference(dateTime);

    if (difference.inDays > 365) {
      final years = (difference.inDays / 365).floor();
      return '$years tahun yang lalu';
    } else if (difference.inDays > 30) {
      final months = (difference.inDays / 30).floor();
      return '$months bulan yang lalu';
    } else if (difference.inDays > 0) {
      return '${difference.inDays} hari yang lalu';
    } else if (difference.inHours > 0) {
      return '${difference.inHours} jam yang lalu';
    } else if (difference.inMinutes > 0) {
      return '${difference.inMinutes} menit yang lalu';
    } else {
      return 'Baru saja';
    }
  }

  /// Format number with thousand separator
  ///
  /// Example: 1000000 -> "1.000.000"
  static String formatNumber(double number) {
    return CurrencyUtils.formatNumber(number);
  }

  /// Format number from int with thousand separator
  ///
  /// Example: 1000000 -> "1.000.000"
  static String formatNumberInt(int number) {
    return CurrencyUtils.formatNumberInt(number);
  }

  /// Parse formatted currency string back to double
  ///
  /// Example: "Rp 1.000.000" -> 1000000.0
  static double? parseCurrency(String formatted) {
    return CurrencyUtils.parse(formatted);
  }

  /// Parse formatted currency string back to int
  ///
  /// Example: "Rp 1.000.000" -> 1000000
  static int? parseCurrencyInt(String formatted) {
    return CurrencyUtils.parseInt(formatted);
  }
}
