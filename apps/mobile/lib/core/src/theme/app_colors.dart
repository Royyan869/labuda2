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
  static const Color neutralGray700 = Color(0xFF374151);
  static const Color neutralGray800 = Color(0xFF1F2937);
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

  // Aliases untuk product status (sesuai naming convention)
  static const Color successGreen = statusSuccess;
  static const Color warningYellow = statusWarning;

  // Social media specific colors
  static const Color koiOrange = Color(0xFFFF6B35);
  static const Color koiBlue = Color(0xFF1DA1F2);
  static const Color koiGold = Color(0xFFFFD700);

  // LABUDA Coins colors
  static const Color coinPrimary = Color(0xFFFFA726); // Amber
  static const Color coinSecondary = Color(0xFFFF9800); // Orange

  // Shortcut aliases untuk consistency
  static const Color primary = primaryRed;
  static const Color success = statusSuccess;
  static const Color warning = statusWarning;
  static const Color error = statusError;
  static const Color light = neutralWhite; // For backward compatibility
  static const Color dark = neutralBlack; // For backward compatibility
  static const Color neutral = neutralGray500; // For backward compatibility

  // Gradients
  static const LinearGradient primaryGradient = LinearGradient(
    colors: [primaryRed, primaryPink],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient successGradient = LinearGradient(
    colors: [primaryGreen, Color(0xFF34D399)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient warningGradient = LinearGradient(
    colors: [primaryYellow, Color(0xFFFBBF24)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient coinGradient = LinearGradient(
    colors: [coinPrimary, coinSecondary],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  // Light theme colors
  static const ColorScheme lightColorScheme = ColorScheme.light(
    primary: primaryRed,
    secondary: primaryBlue,
    surface: neutralWhite,
    error: statusError,
    onPrimary: neutralWhite,
    onSecondary: neutralWhite,
    onSurface: neutralGray900,
    onError: neutralWhite,
    brightness: Brightness.light,
  );

  // Dark theme colors
  static const ColorScheme darkColorScheme = ColorScheme.dark(
    primary: primaryRed,
    secondary: primaryBlue,
    surface: darkGray800,
    error: statusError,
    onPrimary: neutralWhite,
    onSecondary: neutralWhite,
    onSurface: neutralGray100,
    onError: neutralWhite,
    brightness: Brightness.dark,
  );
}
