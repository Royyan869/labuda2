import 'package:flutter/material.dart';

/// Laravel-inspired color palette for LABUDA
/// Supporting both light and dark modes with social media aesthetics
class AppColors {
  AppColors._();

  // Laravel-inspired primary colors
  static const Color primaryRed = Color(0xFFEF4444);
  static const Color primaryBlue = Color(0xFF3B82F6);
  static const Color primaryGreen = Color(0xFF10B981);
  static const Color primaryYellow = Color(0xFFF59E0B);
  static const Color primaryPurple = Color(0xFF8B5CF6);
  static const Color primaryPink = Color(0xFFEC4899);

  // Neutral colors (Light Mode)
  static const Color neutralWhite = Color(0xFFFFFFFF);
  static const Color neutralGray50 = Color(0xFFF9FAFB);
  static const Color neutralGray100 = Color(0xFFF3F4F6);
  static const Color neutralGray200 = Color(0xFFE5E7EB);
  static const Color neutralGray300 = Color(0xFFD1D5DB);
  static const Color neutralGray400 = Color(0xFF9CA3AF);
  static const Color neutralGray500 = Color(0xFF6B7280);
  static const Color neutralGray600 = Color(0xFF4B5563);
  static const Color neutralGray900 = Color(0xFF111827);
  static const Color neutralBlack = Color(0xFF000000);

  // Dark mode colors
  static const Color darkGray900 = Color(0xFF0D1117);
  static const Color darkGray800 = Color(0xFF161B22);
  static const Color darkGray700 = Color(0xFF21262D);
  static const Color darkGray600 = Color(0xFF30363D);
  static const Color darkGray500 = Color(0xFF484F58);

  // Status colors
  static const Color statusSuccess = Color(0xFF059669);
  static const Color statusWarning = Color(0xFFD97706);
  static const Color statusError = Color(0xFFDC2626);
  static const Color statusInfo = Color(0xFF0284C7);

  // Dark-mode status tones.
  //
  // SAME doctrine as Tahap 0: no new hex — each dark tone reuses an existing
  // palette token. Measured against the dark surface (`darkGray800` #161B22),
  // the light tokens fail WCAG AA (4.5:1) for normal text: error #DC2626 = 3.6:1
  // and info #0284C7 = 4.3:1 (success 4.6:1 and warning 5.5:1 pass). Retuned:
  // error 4.6:1, success 6.8:1, warning 7.9:1, info 4.7:1 — all >= 4.5.
  // Consumed ONLY by AppStatusColors.dark; widgets read them through the
  // theme, never bind them (gate: theme_authority_contract_test).
  static const Color darkStatusSuccess = primaryGreen;
  static const Color darkStatusWarning = primaryYellow;
  static const Color darkStatusError = primaryRed;
  static const Color darkStatusInfo = primaryBlue;

  // Social media specific colors
  static const Color koiOrange = Color(0xFFFF6B35);
  static const Color koiGold = Color(0xFFFFD700);

  // LABUDA Coins colors
  static const Color coinPrimary = Color(0xFFFFA726); // Amber
  static const Color coinSecondary = Color(0xFFFF9800); // Orange

  // Gradients
  static const LinearGradient primaryGradient = LinearGradient(
    colors: [primaryRed, primaryPink],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient coinGradient = LinearGradient(
    colors: [coinPrimary, coinSecondary],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  // Light theme colors.
  //
  // THEME AUTHORITY (Tahap 0): every tone below reuses an existing palette
  // token — no new hex. Container roles step darker away from the surface
  // (M3 direction); secondary text and borders use gray tones instead of the
  // previous flat pure-black outline default.
  //
  // EVERY M3 role is defined here. A role left out silently falls back to
  // Flutter's baseline Material palette — that leak is why the snackbar used an
  // unbranded `inverseSurface` and why `tertiary` was Material purple. Roles
  // added below reuse the palette ramp/brand accents (no new hex); retuning a
  // hue is a follow-up, a missing role never is.
  static const ColorScheme lightColorScheme = ColorScheme.light(
    primary: primaryRed,
    secondary: primaryBlue,
    surface: neutralWhite,
    error: statusError,
    onPrimary: neutralWhite,
    onSecondary: neutralWhite,
    onSurface: neutralGray900,
    onSurfaceVariant: neutralGray600,
    onError: neutralWhite,
    outline: neutralGray500,
    outlineVariant: neutralGray300,
    surfaceContainerLowest: neutralWhite,
    surfaceContainerLow: neutralGray50,
    surfaceContainer: neutralGray100,
    surfaceContainerHigh: neutralGray200,
    surfaceContainerHighest: neutralGray300,
    surfaceDim: neutralGray200,
    surfaceBright: neutralWhite,
    inverseSurface: neutralGray900,
    onInverseSurface: neutralWhite,
    inversePrimary: neutralGray100,
    scrim: neutralBlack,
    shadow: neutralBlack,
    surfaceTint: primaryRed,
    primaryContainer: neutralGray200,
    onPrimaryContainer: neutralGray900,
    secondaryContainer: neutralGray100,
    onSecondaryContainer: neutralGray900,
    tertiary: primaryPurple,
    onTertiary: neutralWhite,
    tertiaryContainer: neutralGray100,
    onTertiaryContainer: neutralGray900,
    errorContainer: neutralGray200,
    onErrorContainer: neutralGray900,
    brightness: Brightness.light,
  );

  // Dark theme colors.
  //
  // Same doctrine: container roles step lighter away from the surface
  // (M3 direction); secondary text and borders use gray tones instead of
  // the previous flat pure-white outline default.
  //
  // Same rule as light: every role defined, none left to the Material
  // baseline.
  static const ColorScheme darkColorScheme = ColorScheme.dark(
    primary: primaryRed,
    secondary: primaryBlue,
    surface: darkGray800,
    error: darkStatusError,
    onPrimary: neutralWhite,
    onSecondary: neutralWhite,
    onSurface: neutralGray100,
    onSurfaceVariant: neutralGray300,
    onError: neutralWhite,
    outline: neutralGray400,
    outlineVariant: darkGray600,
    surfaceContainerLowest: darkGray900,
    surfaceContainerLow: darkGray800,
    surfaceContainer: darkGray700,
    surfaceContainerHigh: darkGray600,
    surfaceContainerHighest: darkGray500,
    surfaceDim: darkGray900,
    surfaceBright: darkGray700,
    inverseSurface: neutralGray100,
    onInverseSurface: neutralGray900,
    inversePrimary: darkGray900,
    scrim: neutralBlack,
    shadow: neutralBlack,
    surfaceTint: primaryRed,
    primaryContainer: darkGray700,
    onPrimaryContainer: neutralGray100,
    secondaryContainer: darkGray700,
    onSecondaryContainer: neutralGray100,
    tertiary: primaryPurple,
    onTertiary: neutralWhite,
    tertiaryContainer: darkGray700,
    onTertiaryContainer: neutralGray100,
    errorContainer: darkGray700,
    onErrorContainer: neutralGray100,
    brightness: Brightness.dark,
  );
}
