/// Repost Attribution Bar
///
/// SHARE CONTRACT V1: Displays attribution for reposted content
/// Shows when a content is a repost/share with proper attribution
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Widget that displays repost attribution
///
/// Shows:
/// - "Repost from" indicator
/// - Original author's name (if provided)
class RepostAttributionBar extends StatelessWidget {
  /// Original author ID
  final String? originalAuthorId;

  /// Original author name (if available)
  final String? originalAuthorName;

  /// Callback when tapped - should navigate to original content
  final VoidCallback? onTap;

  const RepostAttributionBar({
    super.key,
    this.originalAuthorId,
    this.originalAuthorName,
    this.onTap,
  });

  /// Whether this is a repost
  bool get isRepost => originalAuthorId != null;

  @override
  Widget build(BuildContext context) {
    if (!isRepost) {
      return const SizedBox.shrink();
    }

    final isDark = Theme.of(context).brightness == Brightness.dark;

    Widget attributionWidget = Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray700.withValues(alpha: 0.5)
            : AppColors.neutralGray100,
        border: Border(
          left: BorderSide(
            color: AppColors.primaryRed.withValues(alpha: 0.5),
            width: 3,
          ),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.repeat_rounded, size: 14, color: AppColors.neutralGray600),
          const SizedBox(width: 6),
          Flexible(
            child: Text(
              _buildAttributionText(),
              style: AppTypography.bodySmall.copyWith(
                color: AppColors.neutralGray700,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
        ],
      ),
    );

    if (onTap != null) {
      return InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(4),
        child: attributionWidget,
      );
    }

    return attributionWidget;
  }

  String _buildAttributionText() {
    if (originalAuthorName != null && originalAuthorName!.isNotEmpty) {
      return 'Repost dari $originalAuthorName';
    }
    return 'Repost';
  }
}

/// Simplified repost indicator for compact spaces
class RepostIndicator extends StatelessWidget {
  final VoidCallback? onTap;

  const RepostIndicator({super.key, this.onTap});

  @override
  Widget build(BuildContext context) {
    Widget child = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.repeat_rounded, size: 12, color: AppColors.neutralGray500),
        const SizedBox(width: 4),
        Text(
          'Repost',
          style: AppTypography.labelSmall.copyWith(
            color: AppColors.neutralGray500,
          ),
        ),
      ],
    );

    if (onTap != null) {
      return InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(4),
        child: child,
      );
    }

    return child;
  }
}
