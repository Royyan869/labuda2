import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'localization_types.dart';

/// Localization provider untuk mengelola bahasa dengan persistence.
///
/// Mengikuti GUIDELINES.md dan pattern dari theme_provider:
/// - Riverpod state management
/// - Shared preferences untuk persistence
/// - Professional UX dengan system locale detection
/// - Fallback ke Bahasa Indonesia sebagai default
class LocalizationController extends Notifier<LocalizationState> {
  static const String _localeKey = 'app_locale';

  @override
  LocalizationState build() {
    _loadLocale();
    return const LocalizationState(currentLocale: SupportedLocale.indonesian);
  }

  /// Load locale dari shared preferences
  Future<void> _loadLocale() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final localeCode = prefs.getString(_localeKey);

      if (localeCode != null) {
        final supportedLocale = SupportedLocale.fromLanguageCode(localeCode);
        state = state.copyWith(currentLocale: supportedLocale);
      } else {
        // Auto-detect system locale jika belum pernah set
        await _detectSystemLocale();
      }
    } catch (e) {
      // Fallback ke Indonesian jika error
      state = state.copyWith(currentLocale: SupportedLocale.indonesian);
    }
  }

  /// Auto-detect system locale
  Future<void> _detectSystemLocale() async {
    try {
      final systemLocales = WidgetsBinding.instance.platformDispatcher.locales;

      for (final systemLocale in systemLocales) {
        for (final supportedLocale in SupportedLocale.values) {
          if (supportedLocale.languageCode == systemLocale.languageCode) {
            state = state.copyWith(currentLocale: supportedLocale);

            // Save detected locale to preferences
            final prefs = await SharedPreferences.getInstance();
            await prefs.setString(_localeKey, supportedLocale.languageCode);
            return;
          }
        }
      }

      // Fallback ke Indonesian jika system locale tidak didukung
      state = state.copyWith(currentLocale: SupportedLocale.indonesian);
    } catch (e) {
      state = state.copyWith(currentLocale: SupportedLocale.indonesian);
    }
  }

  /// Set locale dan simpan ke preferences
  Future<void> setLocale(SupportedLocale locale) async {
    try {
      state = state.copyWith(currentLocale: locale);

      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_localeKey, locale.languageCode);
    } catch (e) {
      // Log error tapi tetap update state
      state = state.copyWith(currentLocale: locale);
    }
  }

  /// Set locale from language code
  Future<void> setLocaleFromCode(String languageCode) async {
    final locale = SupportedLocale.fromLanguageCode(languageCode);
    await setLocale(locale);
  }

  /// Toggle between Indonesian and English
  Future<void> toggleLanguage() async {
    switch (state.currentLocale) {
      case SupportedLocale.indonesian:
        await setLocale(SupportedLocale.english);
        break;
      case SupportedLocale.english:
        await setLocale(SupportedLocale.indonesian);
        break;
    }
  }

  /// Set to Indonesian
  Future<void> setIndonesian() async {
    await setLocale(SupportedLocale.indonesian);
  }

  /// Set to English
  Future<void> setEnglish() async {
    await setLocale(SupportedLocale.english);
  }

  /// Reset to system locale
  Future<void> resetToSystemLocale() async {
    await _detectSystemLocale();
  }
}

/// Provider untuk localization controller
final localizationControllerProvider =
    NotifierProvider<LocalizationController, LocalizationState>(
      LocalizationController.new,
    );
