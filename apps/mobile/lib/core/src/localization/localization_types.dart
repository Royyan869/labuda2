import 'package:flutter/material.dart';

/// Supported locales untuk LABUDA
enum SupportedLocale {
  indonesian('id', 'ID', 'Bahasa Indonesia'),
  english('en', 'US', 'English');

  const SupportedLocale(this.languageCode, this.countryCode, this.displayName);

  final String languageCode;
  final String countryCode;
  final String displayName;

  /// Convert to Flutter Locale
  Locale get locale => Locale(languageCode, countryCode);

  /// Get from locale
  static SupportedLocale fromLocale(Locale locale) {
    for (final supportedLocale in SupportedLocale.values) {
      if (supportedLocale.languageCode == locale.languageCode) {
        return supportedLocale;
      }
    }
    return SupportedLocale.indonesian; // Default fallback
  }

  /// Get from language code
  static SupportedLocale fromLanguageCode(String languageCode) {
    for (final supportedLocale in SupportedLocale.values) {
      if (supportedLocale.languageCode == languageCode) {
        return supportedLocale;
      }
    }
    return SupportedLocale.indonesian; // Default fallback
  }
}

/// Localization state class
class LocalizationState {
  final SupportedLocale currentLocale;
  final bool isLoading;

  const LocalizationState({
    required this.currentLocale,
    this.isLoading = false,
  });

  LocalizationState copyWith({
    SupportedLocale? currentLocale,
    bool? isLoading,
  }) {
    return LocalizationState(
      currentLocale: currentLocale ?? this.currentLocale,
      isLoading: isLoading ?? this.isLoading,
    );
  }

  /// Get current locale as Flutter Locale
  Locale get locale => currentLocale.locale;

  /// Check if current locale is Indonesian
  bool get isIndonesian => currentLocale == SupportedLocale.indonesian;

  /// Check if current locale is English
  bool get isEnglish => currentLocale == SupportedLocale.english;
}

/// Extension untuk SupportedLocale helper methods
extension SupportedLocaleExtension on SupportedLocale {
  /// Get flag emoji
  String get flagEmoji {
    switch (this) {
      case SupportedLocale.indonesian:
        return '🇮🇩';
      case SupportedLocale.english:
        return '🇺🇸';
    }
  }

  /// Get icon
  IconData get icon {
    switch (this) {
      case SupportedLocale.indonesian:
        return Icons.language;
      case SupportedLocale.english:
        return Icons.language;
    }
  }

  /// Get short name
  String get shortName {
    switch (this) {
      case SupportedLocale.indonesian:
        return 'ID';
      case SupportedLocale.english:
        return 'EN';
    }
  }
}
