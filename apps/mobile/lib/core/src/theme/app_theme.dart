import 'package:flutter/material.dart';
import 'app_colors.dart';

class AppTheme {
  AppTheme._();

  /// THEME AUTHORITY (Tahap 0).
  ///
  /// `AppTheme` is the single colour/type authority. Widgets must read from
  /// `Theme.of(context)` (`colorScheme`, `textTheme`, component themes) and
  /// must NOT branch on `isDark`/`brightness`, bind `AppColors.neutral*` /
  /// `AppColors.darkGray*` directly, or hardcode raw Material colours or
  /// raw hex colours.
  /// `AppColors` is the raw token store for this file only — not a widget API.
  ///
  /// Canonical surface mapping (both modes):
  /// screen → `surface`; cards → `surface`; subtle fills →
  /// `surfaceContainerHighest`; borders/dividers → `outlineVariant`;
  /// secondary text/icons → `onSurfaceVariant`.
  /// `textTheme` sizes stay on the M3 default scale (Inter via `fontFamily`);
  /// remapping the type scale is a separate typography scope, not this one.
  static ThemeData get lightTheme {
    final scheme = AppColors.lightColorScheme;
    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      fontFamily: 'Inter',
      scaffoldBackgroundColor: scheme.surface,

      // AppBar theme
      appBarTheme: const AppBarTheme(
        backgroundColor: AppColors.neutralWhite,
        foregroundColor: AppColors.neutralGray900,
        elevation: 0,
        centerTitle: true,
      ),

      // Dialog / bottom sheet / divider / list tile / snackbar follow the
      // scheme roles — identical to the M3 widget defaults, pinned here so
      // the authority is explicit and locked by contract test.
      dialogTheme: DialogThemeData(backgroundColor: scheme.surfaceContainerHigh),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: scheme.surfaceContainerLow,
      ),
      dividerTheme: DividerThemeData(color: scheme.outlineVariant),
      listTileTheme: ListTileThemeData(iconColor: scheme.onSurfaceVariant),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: scheme.inverseSurface,
        actionTextColor: scheme.inversePrimary,
      ),

      // Card theme
      cardTheme: CardThemeData(
        color: AppColors.neutralWhite,
        elevation: 2,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),

      // Elevated button theme
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.primaryRed,
          foregroundColor: AppColors.neutralWhite,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
        ),
      ),

      // Input decoration theme - consistent dengan components
      inputDecorationTheme: InputDecorationTheme(
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: AppColors.neutralGray300.withValues(alpha: 0.5),
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: AppColors.neutralGray300.withValues(alpha: 0.5),
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: AppColors.primaryRed.withValues(alpha: 0.7),
            width: 1.5,
          ),
        ),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 12,
        ),
      ),
    );
  }

  static ThemeData get darkTheme {
    final scheme = AppColors.darkColorScheme;
    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      fontFamily: 'Inter',
      scaffoldBackgroundColor: scheme.surface,

      // AppBar theme
      appBarTheme: const AppBarTheme(
        backgroundColor: AppColors.darkGray800,
        foregroundColor: AppColors.neutralGray100,
        elevation: 0,
        centerTitle: true,
      ),

      // Same canonical mapping as light — scheme roles only (see above).
      dialogTheme: DialogThemeData(backgroundColor: scheme.surfaceContainerHigh),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: scheme.surfaceContainerLow,
      ),
      dividerTheme: DividerThemeData(color: scheme.outlineVariant),
      listTileTheme: ListTileThemeData(iconColor: scheme.onSurfaceVariant),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: scheme.inverseSurface,
        actionTextColor: scheme.inversePrimary,
      ),

      // Card theme
      cardTheme: CardThemeData(
        color: AppColors.darkGray800,
        elevation: 2,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),

      // Elevated button theme
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.primaryRed,
          foregroundColor: AppColors.neutralWhite,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
        ),
      ),

      // Input decoration theme - consistent dengan components
      inputDecorationTheme: InputDecorationTheme(
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: AppColors.darkGray600.withValues(alpha: 0.5),
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: AppColors.darkGray600.withValues(alpha: 0.5),
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: AppColors.primaryRed.withValues(alpha: 0.7),
            width: 1.5,
          ),
        ),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 12,
        ),
      ),
    );
  }
}
