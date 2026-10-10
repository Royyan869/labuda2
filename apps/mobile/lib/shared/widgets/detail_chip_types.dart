import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Size options untuk DetailChipWidget
enum DetailChipSize { small, medium, large }

/// Theme role that supplies a chip's default tint when the caller does not
/// bind one — resolved in build, so the widget never owns a colour decision.
enum DetailChipTone { primary, success, warning, tertiary }

/// Style options untuk DetailChipWidget
enum DetailChipStyle {
  /// Filled style dengan background color yang transparan
  filled,

  /// Outlined style dengan border saja
  outlined,

  /// Solid style dengan background color yang penuh
  solid,
}

/// Utility methods untuk DetailChip styling
class DetailChipStyleUtils {
  static EdgeInsets getPadding(DetailChipSize size) {
    switch (size) {
      case DetailChipSize.small:
        return const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4);
      case DetailChipSize.medium:
        return const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8);
      case DetailChipSize.large:
        return const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8);
    }
  }

  static double getIconSize(DetailChipSize size) {
    switch (size) {
      case DetailChipSize.small:
        return 12;
      case DetailChipSize.medium:
        return 14;
      case DetailChipSize.large:
        return 16;
    }
  }

  static double getSpacing(DetailChipSize size) {
    switch (size) {
      case DetailChipSize.small:
        return 3;
      case DetailChipSize.medium:
        return 4;
      case DetailChipSize.large:
        return 6;
    }
  }

  static double getBorderRadius(DetailChipSize size) {
    switch (size) {
      case DetailChipSize.small:
        return 6;
      case DetailChipSize.medium:
        return 8;
      case DetailChipSize.large:
        return 10;
    }
  }

  static double getFontSize(DetailChipSize size) {
    switch (size) {
      case DetailChipSize.small:
        return 11;
      case DetailChipSize.medium:
        return 12;
      case DetailChipSize.large:
        return 13;
    }
  }

  static BoxDecoration getDecoration(
    DetailChipStyle style,
    Color color,
    DetailChipSize size,
  ) {
    switch (style) {
      case DetailChipStyle.filled:
        return BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(getBorderRadius(size)),
          border: Border.all(color: color.withValues(alpha: 0.3), width: 1),
        );
      case DetailChipStyle.outlined:
        return BoxDecoration(
          color: Colors.transparent,
          borderRadius: BorderRadius.circular(getBorderRadius(size)),
          border: Border.all(color: color.withValues(alpha: 0.5), width: 1),
        );
      case DetailChipStyle.solid:
        return BoxDecoration(
          color: color,
          borderRadius: BorderRadius.circular(getBorderRadius(size)),
        );
    }
  }

  /// Content ink for a chip bound to semantic [color].
  ///
  /// Tinted styles keep the bound color in BOTH modes (the alpha-0.8 dark
  /// dimming is killed: it only worsened dark-mode contrast). Solid fills
  /// pair with scheme onPrimary. No brightness branch anywhere.
  static Color getContentColor(
    DetailChipStyle style,
    Color color,
    ColorScheme scheme,
  ) {
    switch (style) {
      case DetailChipStyle.filled:
      case DetailChipStyle.outlined:
        return color;
      case DetailChipStyle.solid:
        return scheme.onPrimary;
    }
  }
}
