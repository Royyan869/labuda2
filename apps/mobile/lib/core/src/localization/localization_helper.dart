import 'package:flutter/material.dart';
import 'localization_types.dart';

/// Helper untuk mendapatkan list supported locales
class LocalizationHelper {
  /// Get all supported locales as Flutter Locale list
  static List<Locale> get supportedLocales {
    return SupportedLocale.values.map((e) => e.locale).toList();
  }

  /// Get all supported locales
  static List<SupportedLocale> get allSupportedLocales {
    return SupportedLocale.values;
  }

  /// Check if locale is supported
  static bool isLocaleSupported(Locale locale) {
    return supportedLocales.any(
      (supported) => supported.languageCode == locale.languageCode,
    );
  }

  /// Get closest supported locale
  static Locale getClosestSupportedLocale(Locale locale) {
    // Try exact match first
    for (final supported in supportedLocales) {
      if (supported.languageCode == locale.languageCode &&
          supported.countryCode == locale.countryCode) {
        return supported;
      }
    }

    // Try language code match
    for (final supported in supportedLocales) {
      if (supported.languageCode == locale.languageCode) {
        return supported;
      }
    }

    // Fallback to Indonesian
    return SupportedLocale.indonesian.locale;
  }
}
