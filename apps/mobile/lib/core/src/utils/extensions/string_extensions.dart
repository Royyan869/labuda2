import 'package:labuda/shared/helpers/canonical_email_validator.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:labuda/shared/utils/currency_utils.dart';

extension StringExtensions on String {
  /// STAGE 4B-1: delegates to the canonical email authority.
  bool get isValidEmail => CanonicalEmailValidator.isValid(this);

  /// STAGE 4B-1: delegates to the canonical Indonesian phone authority
  /// (format only — no normalization, no verification).
  bool get isValidPhoneNumber => CanonicalPhoneValidator.isValid(this);

  String get capitalizeFirst {
    if (isEmpty) return this;
    return this[0].toUpperCase() + substring(1);
  }

  String get capitalizeWords {
    if (isEmpty) return this;
    return split(' ').map((word) => word.capitalizeFirst).join(' ');
  }

  String get removeWhitespace {
    return replaceAll(RegExp(r'\s+'), '');
  }

  String get normalizeWhitespace {
    return replaceAll(RegExp(r'\s+'), ' ').trim();
  }

  bool get isNumeric {
    return double.tryParse(this) != null;
  }

  double? get toDoubleOrNull {
    return double.tryParse(this);
  }

  int? get toIntOrNull {
    return int.tryParse(this);
  }

  String truncate(int maxLength, {String suffix = '...'}) {
    if (length <= maxLength) return this;
    return '${substring(0, maxLength)}$suffix';
  }

  String get formatPhoneNumber {
    if (!isValidPhoneNumber) return this;

    String cleaned = replaceAll(RegExp(r'[^0-9]'), '');

    if (cleaned.startsWith('0')) {
      cleaned = '62${cleaned.substring(1)}';
    } else if (cleaned.startsWith('62')) {
      // Already in correct format
    } else {
      cleaned = '62$cleaned';
    }

    return '+$cleaned';
  }

  /// Formats numeric string as currency
  ///
  /// Delegates to [CurrencyUtils.format] for consistent formatting.
  String get formatCurrency {
    final number = toDoubleOrNull;
    if (number == null) return this;
    return CurrencyUtils.format(number);
  }

  String get slug {
    return toLowerCase()
        .replaceAll(RegExp(r'[^a-z0-9\s-]'), '')
        .replaceAll(RegExp(r'\s+'), '-')
        .replaceAll(RegExp(r'-+'), '-')
        .replaceAll(RegExp(r'^-|-$'), '');
  }

  bool containsIgnoreCase(String other) {
    return toLowerCase().contains(other.toLowerCase());
  }

  String get initials {
    final words = trim().split(RegExp(r'\s+'));
    if (words.isEmpty) return '';

    if (words.length == 1) {
      return words[0].isNotEmpty
          ? words[0].substring(0, 1.clamp(0, words[0].length)).toUpperCase()
          : '';
    }

    return words
        .take(2)
        .map((word) => word.isNotEmpty ? word[0].toUpperCase() : '')
        .join('');
  }

  String get hideEmail {
    if (!isValidEmail) return this;

    final parts = split('@');
    final username = parts[0];
    final domain = parts[1];

    if (username.length <= 2) {
      return '${username[0]}***@$domain';
    }

    return '${username.substring(0, 2.clamp(0, username.length))}***@$domain';
  }

  String get hidePhoneNumber {
    if (!isValidPhoneNumber) return this;

    final formatted = formatPhoneNumber;
    if (formatted.length <= 6) return formatted;

    return '${formatted.substring(0, 6.clamp(0, formatted.length))}***${formatted.substring((formatted.length - 2).clamp(0, formatted.length))}';
  }
}
