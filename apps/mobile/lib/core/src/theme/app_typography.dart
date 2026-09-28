import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

class AppTypography {
  AppTypography._();

  // Headlines
  static const TextStyle h1 = TextStyle(

    fontSize: AppType.s32,
    fontWeight: FontWeight.bold,
    height: 1.2,
  );

  static const TextStyle h2 = TextStyle(

    fontSize: AppType.s28,
    fontWeight: FontWeight.bold,
    height: 1.25,
  );

  static const TextStyle h3 = TextStyle(

    fontSize: AppType.s24,
    fontWeight: FontWeight.w600,
    height: 1.3,
  );

  static const TextStyle h4 = TextStyle(

    fontSize: AppType.s20,
    fontWeight: FontWeight.w600,
    height: 1.4,
  );

  static const TextStyle h5 = TextStyle(

    fontSize: AppType.s18,
    fontWeight: FontWeight.w600,
    height: 1.4,
  );

  static const TextStyle h6 = TextStyle(

    fontSize: AppType.s16,
    fontWeight: FontWeight.w600,
    height: 1.5,
  );

  // Body text
  static const TextStyle bodyLarge = TextStyle(

    fontSize: AppType.s16,
    fontWeight: FontWeight.normal,
    height: 1.5,
  );

  static const TextStyle bodyMedium = TextStyle(

    fontSize: AppType.s14,
    fontWeight: FontWeight.normal,
    height: 1.5,
  );

  static const TextStyle bodySmall = TextStyle(

    fontSize: AppType.s12,
    fontWeight: FontWeight.normal,
    height: 1.5,
  );

  // Labels
  static const TextStyle labelLarge = TextStyle(

    fontSize: AppType.s14,
    fontWeight: FontWeight.w500,
    height: 1.4,
  );

  static const TextStyle labelMedium = TextStyle(

    fontSize: AppType.s12,
    fontWeight: FontWeight.w500,
    height: 1.4,
  );

  static const TextStyle labelSmall = TextStyle(

    fontSize: AppType.s10,
    fontWeight: FontWeight.w500,
    height: 1.4,
  );

  static const TextStyle labelEmphasized = TextStyle(

    fontSize: AppType.s14,
    fontWeight: FontWeight.w600,
    height: 1.4,
  );

  // Captions
  static const TextStyle caption = TextStyle(

    fontSize: AppType.s12,
    fontWeight: FontWeight.normal,
    height: 1.3,
  );

  static const TextStyle overline = TextStyle(

    fontSize: AppType.s10,
    fontWeight: FontWeight.w500,
    height: 1.6,
    letterSpacing: 1.5,
  );

  // Button text
  static const TextStyle button = TextStyle(

    fontSize: AppType.s14,
    fontWeight: FontWeight.w500,
    height: 1.25,
  );

  // Custom styles
  static const TextStyle price = TextStyle(

    fontSize: AppType.s18,
    fontWeight: FontWeight.bold,
    height: 1.2,
  );

  static const TextStyle koiVariety = TextStyle(

    fontSize: AppType.s14,
    fontWeight: FontWeight.w500,
    height: 1.3,
  );

  static const TextStyle username = TextStyle(

    fontSize: AppType.s14,
    fontWeight: FontWeight.w600,
    height: 1.3,
  );

  static const TextStyle timestamp = TextStyle(

    fontSize: AppType.s12,
    fontWeight: FontWeight.normal,
    height: 1.2,
  );
}
