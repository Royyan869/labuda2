import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'app_theme.dart';

/// Theme state class
class ThemeState {
  final ThemeMode themeMode;
  final bool isLoading;

  const ThemeState({required this.themeMode, this.isLoading = false});

  ThemeState copyWith({ThemeMode? themeMode, bool? isLoading}) {
    return ThemeState(
      themeMode: themeMode ?? this.themeMode,
      isLoading: isLoading ?? this.isLoading,
    );
  }

  /// Get current effective theme berdasarkan system
  Brightness getCurrentBrightness(BuildContext context) {
    switch (themeMode) {
      case ThemeMode.light:
        return Brightness.light;
      case ThemeMode.dark:
        return Brightness.dark;
      case ThemeMode.system:
        return MediaQuery.platformBrightnessOf(context);
    }
  }

  /// Check if currently using dark theme
  bool isDarkMode(BuildContext context) {
    return getCurrentBrightness(context) == Brightness.dark;
  }
}

/// Theme provider untuk mengelola dark/light mode dengan persistence.
///
/// Mengikuti GUIDELINES.md:
/// - Riverpod state management
/// - Shared preferences untuk persistence
/// - Professional UX dengan system theme detection
class ThemeController extends Notifier<ThemeState> {
  static const String _themeKey = 'theme_mode';

  @override
  ThemeState build() {
    _loadTheme();
    return const ThemeState(themeMode: ThemeMode.system);
  }

  /// Load theme dari shared preferences
  Future<void> _loadTheme() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final themeIndex = prefs.getInt(_themeKey);

      if (themeIndex != null) {
        state = state.copyWith(themeMode: ThemeMode.values[themeIndex]);
      }
    } catch (e) {
      // Fallback ke system theme jika error
      state = state.copyWith(themeMode: ThemeMode.system);
    }
  }

  /// Set theme mode dan simpan ke preferences
  Future<void> setThemeMode(ThemeMode themeMode) async {
    try {
      state = state.copyWith(themeMode: themeMode);

      final prefs = await SharedPreferences.getInstance();
      await prefs.setInt(_themeKey, themeMode.index);
    } catch (e) {
      // Log error tapi tetap update state
      state = state.copyWith(themeMode: themeMode);
    }
  }

  /// Toggle antara light dan dark (skip system)
  Future<void> toggleTheme() async {
    switch (state.themeMode) {
      case ThemeMode.light:
        await setThemeMode(ThemeMode.dark);
        break;
      case ThemeMode.dark:
        await setThemeMode(ThemeMode.light);
        break;
      case ThemeMode.system:
        // Detect current system theme dan toggle ke opposite
        final brightness =
            WidgetsBinding.instance.platformDispatcher.platformBrightness;
        if (brightness == Brightness.dark) {
          await setThemeMode(ThemeMode.light);
        } else {
          await setThemeMode(ThemeMode.dark);
        }
        break;
    }
  }

  /// Set ke system theme
  Future<void> setSystemTheme() async {
    await setThemeMode(ThemeMode.system);
  }
}

/// Provider untuk theme controller
final themeControllerProvider = NotifierProvider<ThemeController, ThemeState>(
  ThemeController.new,
);

/// Helper extension untuk easy access
extension ThemeModeExtension on ThemeMode {
  String get displayName {
    switch (this) {
      case ThemeMode.light:
        return 'Light';
      case ThemeMode.dark:
        return 'Dark';
      case ThemeMode.system:
        return 'System';
    }
  }

  IconData get icon {
    switch (this) {
      case ThemeMode.light:
        return Icons.light_mode;
      case ThemeMode.dark:
        return Icons.dark_mode;
      case ThemeMode.system:
        return Icons.brightness_auto;
    }
  }
}

/// Helper untuk mendapatkan theme data
class ThemeHelper {
  static ThemeData getThemeData(
    ThemeMode themeMode,
    Brightness systemBrightness,
  ) {
    switch (themeMode) {
      case ThemeMode.light:
        return AppTheme.lightTheme;
      case ThemeMode.dark:
        return AppTheme.darkTheme;
      case ThemeMode.system:
        return systemBrightness == Brightness.dark
            ? AppTheme.darkTheme
            : AppTheme.lightTheme;
    }
  }
}
