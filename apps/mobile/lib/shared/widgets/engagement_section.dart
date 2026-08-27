import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Configuration for an individual engagement action
class EngagementAction {
  final IconData icon;
  final IconData? activeIcon;
  final String label;
  final int count;
  final bool isActive;
  final bool isEnabled;
  final VoidCallback? onTap;
  final Color? activeColor;

  const EngagementAction({
    required this.icon,
    this.activeIcon,
    required this.label,
    this.count = 0,
    this.isActive = false,
    this.isEnabled = true,
    this.onTap,
    this.activeColor,
  });

  /// Factory for Like action
  factory EngagementAction.like({
    required int count,
    required bool isLiked,
    VoidCallback? onTap,
  }) => EngagementAction(
    icon: Icons.favorite_border,
    activeIcon: Icons.favorite,
    label: 'Like',
    count: count,
    isActive: isLiked,
    onTap: onTap,
    activeColor: AppColors.primaryRed,
  );

  /// Factory for Comment action
  factory EngagementAction.comment({
    required int count,
    bool allowComments = true,
    VoidCallback? onTap,
  }) => EngagementAction(
    icon: allowComments
        ? Icons.comment_outlined
        : Icons.comments_disabled_outlined,
    label: 'Comment',
    count: count,
    isEnabled: allowComments,
    onTap: onTap,
  );

  /// Factory for Share action
  factory EngagementAction.share({required int count, VoidCallback? onTap}) =>
      EngagementAction(
        icon: Icons.share_outlined,
        label: 'Share',
        count: count,
        onTap: onTap,
      );

  /// Factory for View action (info only)
  factory EngagementAction.view({required int count}) => EngagementAction(
    icon: Icons.visibility_outlined,
    label: 'View',
    count: count,
    onTap: null, // View is informational only
  );

  /// Factory for Watch/Bookmark action
  factory EngagementAction.watch({
    required int count,
    required bool isWatching,
    VoidCallback? onTap,
  }) => EngagementAction(
    icon: Icons.visibility_outlined,
    activeIcon: Icons.visibility,
    label: 'Watch',
    count: count,
    isActive: isWatching,
    onTap: onTap,
    activeColor: AppColors.primaryRed,
  );

  /// Factory for Save/Bookmark action
  factory EngagementAction.save({
    required int count,
    required bool isSaved,
    VoidCallback? onTap,
  }) => EngagementAction(
    icon: Icons.bookmark_border,
    activeIcon: Icons.bookmark,
    label: 'Save',
    count: count,
    isActive: isSaved,
    onTap: onTap,
    activeColor: AppColors.coinPrimary,
  );

  /// Factory for Chat action
  factory EngagementAction.chat({VoidCallback? onTap}) => EngagementAction(
    icon: Icons.chat_bubble_outline,
    label: 'Chat',
    count: 0,
    onTap: onTap,
  );
}

/// Layout style for engagement section
enum EngagementLayoutStyle {
  /// Facebook-style: horizontal with count above icon/label
  horizontal,

  /// TikTok-style: vertical stacked buttons
  vertical,

  /// Compact: icon + count only, no label
  compact,

  /// Minimal: only icons, no labels or counts
  minimal,
}

/// Reusable Engagement Section widget
///
/// Consolidates common engagement patterns (Like, Comment, Share, View)
///
/// Usage:
/// ```dart
/// EngagementSection(
///   actions: [
///     EngagementAction.like(count: 123, isLiked: true, onTap: () {}),
///     EngagementAction.comment(count: 45, onTap: () {}),
///     EngagementAction.share(count: 12, onTap: () {}),
///     EngagementAction.view(count: 1000),
///   ],
/// )
/// ```
class EngagementSection extends StatelessWidget {
  /// List of engagement actions to display
  final List<EngagementAction> actions;

  /// Layout style
  final EngagementLayoutStyle style;

  /// Icon size
  final double iconSize;

  /// Show counts
  final bool showCounts;

  /// Show labels
  final bool showLabels;

  /// Spacing between actions
  final double spacing;

  /// Whether to expand actions evenly
  final bool expandActions;

  const EngagementSection({
    super.key,
    required this.actions,
    this.style = EngagementLayoutStyle.horizontal,
    this.iconSize = 18,
    this.showCounts = true,
    this.showLabels = true,
    this.spacing = 0,
    this.expandActions = true,
  });

  /// Compact style with icon + count only
  const EngagementSection.compact({
    super.key,
    required this.actions,
    this.iconSize = 16,
    this.spacing = 16,
  }) : style = EngagementLayoutStyle.compact,
       showCounts = true,
       showLabels = false,
       expandActions = false;

  /// Minimal style with icons only
  const EngagementSection.minimal({
    super.key,
    required this.actions,
    this.iconSize = 20,
    this.spacing = 8,
  }) : style = EngagementLayoutStyle.minimal,
       showCounts = false,
       showLabels = false,
       expandActions = false;

  /// TikTok-style vertical layout
  const EngagementSection.vertical({
    super.key,
    required this.actions,
    this.iconSize = 24,
    this.spacing = 16,
  }) : style = EngagementLayoutStyle.vertical,
       showCounts = true,
       showLabels = true,
       expandActions = false;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (style == EngagementLayoutStyle.vertical) {
      return Column(
        mainAxisSize: MainAxisSize.min,
        children: actions.map((action) {
          return Padding(
            padding: EdgeInsets.only(bottom: spacing),
            child: _buildVerticalAction(action, isDark),
          );
        }).toList(),
      );
    }

    final List<Widget> children = [];
    for (int i = 0; i < actions.length; i++) {
      final action = actions[i];
      Widget actionWidget;

      switch (style) {
        case EngagementLayoutStyle.horizontal:
          actionWidget = _buildHorizontalAction(action, isDark);
          break;
        case EngagementLayoutStyle.compact:
          actionWidget = _buildCompactAction(action, isDark);
          break;
        case EngagementLayoutStyle.minimal:
          actionWidget = _buildMinimalAction(action, isDark);
          break;
        case EngagementLayoutStyle.vertical:
          actionWidget = _buildVerticalAction(action, isDark);
          break;
      }

      if (expandActions && style == EngagementLayoutStyle.horizontal) {
        children.add(Expanded(child: actionWidget));
      } else {
        children.add(actionWidget);
        if (i < actions.length - 1 && spacing > 0) {
          children.add(SizedBox(width: spacing));
        }
      }
    }

    return Row(
      mainAxisSize: expandActions ? MainAxisSize.max : MainAxisSize.min,
      children: children,
    );
  }

  /// Horizontal style: count above, icon + label below
  Widget _buildHorizontalAction(EngagementAction action, bool isDark) {
    final effectiveIcon = action.isActive && action.activeIcon != null
        ? action.activeIcon!
        : action.icon;
    final effectiveColor = action.isActive
        ? (action.activeColor ?? AppColors.primaryRed)
        : (action.isEnabled
              ? (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600)
              : (isDark ? AppColors.neutralGray600 : AppColors.neutralGray400));

    return InkWell(
      onTap: action.isEnabled ? action.onTap : null,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (showCounts)
              Text(
                _formatCount(action.count),
                style: AppTypography.labelMedium.copyWith(
                  fontWeight: FontWeight.bold,
                  color: effectiveColor,
                ),
              ),
            const SizedBox(height: 2),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(effectiveIcon, size: iconSize, color: effectiveColor),
                if (showLabels) ...[
                  const SizedBox(width: 4),
                  Text(
                    action.label,
                    style: AppTypography.labelSmall.copyWith(
                      fontWeight: FontWeight.w600,
                      color: effectiveColor,
                    ),
                  ),
                ],
              ],
            ),
          ],
        ),
      ),
    );
  }

  /// Compact style: icon + count in row
  Widget _buildCompactAction(EngagementAction action, bool isDark) {
    final effectiveIcon = action.isActive && action.activeIcon != null
        ? action.activeIcon!
        : action.icon;
    final effectiveColor = action.isActive
        ? (action.activeColor ?? AppColors.primaryRed)
        : (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600);

    return InkWell(
      onTap: action.isEnabled ? action.onTap : null,
      borderRadius: BorderRadius.circular(4),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(effectiveIcon, size: iconSize, color: effectiveColor),
            if (showCounts && action.count > 0) ...[
              const SizedBox(width: 4),
              Text(
                _formatCount(action.count),
                style: AppTypography.labelSmall.copyWith(
                  fontWeight: FontWeight.w600,
                  color: effectiveColor,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  /// Minimal style: icon only
  Widget _buildMinimalAction(EngagementAction action, bool isDark) {
    final effectiveIcon = action.isActive && action.activeIcon != null
        ? action.activeIcon!
        : action.icon;
    final effectiveColor = action.isActive
        ? (action.activeColor ?? AppColors.primaryRed)
        : (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600);

    return InkWell(
      onTap: action.isEnabled ? action.onTap : null,
      borderRadius: BorderRadius.circular(4),
      child: Padding(
        padding: const EdgeInsets.all(4),
        child: Icon(effectiveIcon, size: iconSize, color: effectiveColor),
      ),
    );
  }

  /// Vertical style: icon at top, count below, label at bottom
  Widget _buildVerticalAction(EngagementAction action, bool isDark) {
    final effectiveIcon = action.isActive && action.activeIcon != null
        ? action.activeIcon!
        : action.icon;
    final effectiveColor = action.isActive
        ? (action.activeColor ?? AppColors.primaryRed)
        : (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600);

    return InkWell(
      onTap: action.isEnabled ? action.onTap : null,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(effectiveIcon, size: iconSize, color: effectiveColor),
            if (showCounts) ...[
              const SizedBox(height: 2),
              Text(
                _formatCount(action.count),
                style: AppTypography.labelSmall.copyWith(
                  fontWeight: FontWeight.w600,
                  color: effectiveColor,
                ),
              ),
            ],
            if (showLabels) ...[
              const SizedBox(height: 2),
              Text(
                action.label,
                style: AppTypography.labelSmall.copyWith(
                  fontSize: 10,
                  color: effectiveColor,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  /// Format count with K/M suffix
  String _formatCount(int count) {
    if (count == 0) return '0';
    if (count >= 1000000) {
      return '${(count / 1000000).toStringAsFixed(1)}M';
    }
    if (count >= 1000) {
      return '${(count / 1000).toStringAsFixed(1)}K';
    }
    return count.toString();
  }
}

/// Single engagement button for standalone use
class EngagementButton extends StatelessWidget {
  final EngagementAction action;
  final EngagementLayoutStyle style;
  final double iconSize;
  final bool showCount;
  final bool showLabel;

  const EngagementButton({
    super.key,
    required this.action,
    this.style = EngagementLayoutStyle.compact,
    this.iconSize = 18,
    this.showCount = true,
    this.showLabel = false,
  });

  @override
  Widget build(BuildContext context) {
    return EngagementSection(
      actions: [action],
      style: style,
      iconSize: iconSize,
      showCounts: showCount,
      showLabels: showLabel,
      expandActions: false,
    );
  }
}
