import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Utility functions for like count widget styling
///
/// Features:
/// - Style-specific configurations
/// - Color utilities
/// - Count formatting
/// - Decoration builders
class LikeCountStyleUtils {
  static EdgeInsets getPadding(LikeCountStyle style) {
    switch (style) {
      case LikeCountStyle.compact:
        return const EdgeInsets.symmetric(horizontal: 4, vertical: 2);
      case LikeCountStyle.detailed:
        return const EdgeInsets.symmetric(horizontal: 12, vertical: 6);
      case LikeCountStyle.withRecent:
        return const EdgeInsets.symmetric(horizontal: 8, vertical: 4);
      case LikeCountStyle.normal:
        return const EdgeInsets.symmetric(horizontal: 8, vertical: 4);
    }
  }

  static BoxDecoration? getDecoration(LikeCountStyle style, bool isDark) {
    switch (style) {
      case LikeCountStyle.detailed:
        return BoxDecoration(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
          ),
        );
      case LikeCountStyle.compact:
      case LikeCountStyle.normal:
      case LikeCountStyle.withRecent:
        return null;
    }
  }

  static double getIconSize(LikeCountStyle style) {
    switch (style) {
      case LikeCountStyle.compact:
        return 14;
      case LikeCountStyle.detailed:
        return 18;
      case LikeCountStyle.normal:
      case LikeCountStyle.withRecent:
        return 16;
    }
  }

  static double getTextSize(LikeCountStyle style) {
    switch (style) {
      case LikeCountStyle.compact:
        return 11;
      case LikeCountStyle.detailed:
        return 14;
      case LikeCountStyle.normal:
      case LikeCountStyle.withRecent:
        return 12;
    }
  }

  static Color getIconColor(bool isLiked, bool isDark) {
    return isLiked
        ? AppColors.primaryRed
        : (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600);
  }

  static Color getTextColor(bool isLiked, bool isDark) {
    return isLiked
        ? AppColors.primaryRed
        : (isDark ? AppColors.neutralGray300 : AppColors.neutralGray700);
  }

  static String formatCount(int count) {
    if (count < 1000) {
      return count.toString();
    } else if (count < 1000000) {
      double k = count / 1000;
      return '${k.toStringAsFixed(k % 1 == 0 ? 0 : 1)}K';
    } else {
      double m = count / 1000000;
      return '${m.toStringAsFixed(m % 1 == 0 ? 0 : 1)}M';
    }
  }
}

/// Style options for LikeCountWidget
enum LikeCountStyle {
  compact, // Minimal style
  normal, // Default style
  detailed, // With background and "likes" text
  withRecent, // Shows recent liker avatars
}
