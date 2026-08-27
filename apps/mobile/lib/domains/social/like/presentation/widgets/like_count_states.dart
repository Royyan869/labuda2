import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/like/presentation/widgets/like_count_style_utils.dart';

/// Loading and error states for like count widget
///
/// Features:
/// - Loading state with spinner
/// - Error state with fallback
/// - Style-aware sizing
class LikeCountStates {
  static Widget buildLoadingState(bool isDark, LikeCountStyle style) {
    return Container(
      padding: _getPadding(style),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: _getIconSize(style),
            height: _getIconSize(style),
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(
                isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
              ),
            ),
          ),
          const SizedBox(width: 4),
          Text(
            '...',
            style: TextStyle(
              fontSize: _getTextSize(style),
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
            ),
          ),
        ],
      ),
    );
  }

  static Widget buildErrorState(bool isDark, LikeCountStyle style) {
    return Container(
      padding: _getPadding(style),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.favorite_border,
            size: _getIconSize(style),
            color: isDark ? AppColors.neutralGray600 : AppColors.neutralGray400,
          ),
          const SizedBox(width: 4),
          Text(
            '0',
            style: TextStyle(
              fontSize: _getTextSize(style),
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray400,
            ),
          ),
        ],
      ),
    );
  }

  static EdgeInsets _getPadding(LikeCountStyle style) {
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

  static double _getIconSize(LikeCountStyle style) {
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

  static double _getTextSize(LikeCountStyle style) {
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
}
