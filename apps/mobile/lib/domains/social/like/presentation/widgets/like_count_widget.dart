import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/like/presentation/widgets/like_count_states.dart';
import 'package:labuda/domains/social/like/presentation/widgets/like_count_style_utils.dart';

/// Like count widget with various styling options
///
/// Features:
/// - Multiple styling options (compact, normal, detailed)
/// - Loading and error states
/// - Count formatting (K, M suffixes)
/// - Theme-aware colors
///
/// Refactored into modular components:
/// - LikeCountStates: For loading and error states
/// - LikeCountStyleUtils: For styling utilities
class LikeCountWidget extends ConsumerWidget {
  final String targetId;
  final LikeTargetType targetType;
  final String? currentUserId;
  final VoidCallback? onTap;
  final LikeCountStyle style;
  final bool showZeroCount;

  const LikeCountWidget({
    super.key,
    required this.targetId,
    required this.targetType,
    this.currentUserId,
    this.onTap,
    this.style = LikeCountStyle.normal,
    this.showZeroCount = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Get current user ID with proper authentication check
    final authenticatedUserId = currentUserId ?? '';

    // Watch like stats for this target
    final likeStatsAsync = ref.watch(
      likeStatsProvider(
        LikeStatsParams(
          targetId: targetId,
          targetType: targetType,
          currentUserId: authenticatedUserId,
        ),
      ),
    );

    return likeStatsAsync.when(
      loading: () => LikeCountStates.buildLoadingState(isDark, style),
      error: (error, stackTrace) =>
          LikeCountStates.buildErrorState(isDark, style),
      data: (stats) => _buildContent(context, stats, isDark),
    );
  }

  Widget _buildContent(BuildContext context, LikeStats stats, bool isDark) {
    if (!showZeroCount && stats.totalLikes == 0) {
      return const SizedBox.shrink();
    }

    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: LikeCountStyleUtils.getPadding(style),
        decoration: LikeCountStyleUtils.getDecoration(style, isDark),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              stats.isLikedByCurrentUser
                  ? Icons.favorite
                  : Icons.favorite_border,
              size: LikeCountStyleUtils.getIconSize(style),
              color: LikeCountStyleUtils.getIconColor(
                stats.isLikedByCurrentUser,
                isDark,
              ),
            ),
            const SizedBox(width: 4),
            Text(
              LikeCountStyleUtils.formatCount(stats.totalLikes),
              style: TextStyle(
                fontSize: LikeCountStyleUtils.getTextSize(style),
                fontWeight: FontWeight.w600,
                color: LikeCountStyleUtils.getTextColor(
                  stats.isLikedByCurrentUser,
                  isDark,
                ),
              ),
            ),
            if (style == LikeCountStyle.detailed && stats.totalLikes > 0) ...[
              const SizedBox(width: 4),
              Text(
                stats.totalLikes == 1 ? 'like' : 'likes',
                style: TextStyle(
                  fontSize: LikeCountStyleUtils.getTextSize(style) - 2,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
