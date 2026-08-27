import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/follow.dart';
import 'package:labuda/domains/social/rating/rating.dart';

/// Profile Stats Widget V2 - Fresh design dengan horizontal layout
///
/// CONTRACT ALIGNMENT V1: Honest stats display
/// - Followers, Following count dengan realtime updates
/// - Rating stars (seller only) - only shows if real data available
/// - Trust Score - HIDDEN (not yet implemented, no fake data)
/// - Tap to navigate ke detail pages
class ProfileStats extends ConsumerWidget {
  final String userId;
  final bool isSeller;

  const ProfileStats({super.key, required this.userId, this.isSeller = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Watch follow stats stream untuk realtime updates
    final followStatsAsync = ref.watch(followStatsStreamProvider(userId));

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          top: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          // Followers
          Expanded(
            child: followStatsAsync.when(
              data: (stats) => _StatItem(
                value: _formatCount(stats.followersCount),
                label: 'Followers',
                isDark: isDark,
                icon: Icons.people_outline,
                onTap: () => _navigateToFollowers(context),
              ),
              loading: () => _StatItem(
                value: '-',
                label: 'Followers',
                isDark: isDark,
                icon: Icons.people_outline,
                onTap: () => _navigateToFollowers(context),
              ),
              error: (_, _) => _StatItem(
                value: '-',
                label: 'Followers',
                isDark: isDark,
                icon: Icons.people_outline,
                onTap: () => _navigateToFollowers(context),
              ),
            ),
          ),

          _buildDivider(isDark),

          // Following
          Expanded(
            child: followStatsAsync.when(
              data: (stats) => _StatItem(
                value: _formatCount(stats.followingCount),
                label: 'Following',
                isDark: isDark,
                icon: Icons.person_add_alt_outlined,
                onTap: () => _navigateToFollowing(context),
              ),
              loading: () => _StatItem(
                value: '-',
                label: 'Following',
                isDark: isDark,
                icon: Icons.person_add_alt_outlined,
                onTap: () => _navigateToFollowing(context),
              ),
              error: (_, _) => _StatItem(
                value: '-',
                label: 'Following',
                isDark: isDark,
                icon: Icons.person_add_alt_outlined,
                onTap: () => _navigateToFollowing(context),
              ),
            ),
          ),

          // Rating (seller only)
          if (isSeller) ...[
            _buildDivider(isDark),
            Expanded(child: _buildRatingItem(context, isDark, ref)),
          ],
        ],
      ),
    );
  }

  Widget _buildDivider(bool isDark) {
    return Container(
      height: 32,
      width: 1,
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
    );
  }

  Widget _buildRatingItem(BuildContext context, bool isDark, WidgetRef ref) {
    final ratingSummaryAsync = ref.watch(
      getUserRatingSummaryProvider(userId: userId),
    );

    return ratingSummaryAsync.when(
      data: (result) {
        if (result.isError || result.data == null) {
          return _RatingStatItem(
            rating: 0.0,
            reviewCount: 0,
            isDark: isDark,
            onTap: () => _navigateToReviews(context),
          );
        }

        final summary = result.data!;
        return _RatingStatItem(
          rating: summary.averageRating,
          reviewCount: summary.totalRatings,
          isDark: isDark,
          onTap: () => _navigateToReviews(context),
        );
      },
      loading: () => _RatingStatItem(
        rating: null,
        reviewCount: 0,
        isDark: isDark,
        onTap: () => _navigateToReviews(context),
      ),
      error: (_, _) => _RatingStatItem(
        rating: 0.0,
        reviewCount: 0,
        isDark: isDark,
        onTap: () => _navigateToReviews(context),
      ),
    );
  }

  void _navigateToFollowers(BuildContext context) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) =>
            FollowListScreen(userId: userId, type: FollowListType.followers),
      ),
    );
  }

  void _navigateToFollowing(BuildContext context) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) =>
            FollowListScreen(userId: userId, type: FollowListType.following),
      ),
    );
  }

  void _navigateToReviews(BuildContext context) {
    // TODO: Navigate to reviews tab
  }

  String _formatCount(int count) {
    if (count >= 1000000) {
      return '${(count / 1000000).toStringAsFixed(1)}M';
    } else if (count >= 1000) {
      return '${(count / 1000).toStringAsFixed(1)}K';
    }
    return count.toString();
  }
}

/// Single stat item
class _StatItem extends StatelessWidget {
  final String value;
  final String label;
  final bool isDark;
  final IconData? icon;
  final VoidCallback? onTap;

  const _StatItem({
    required this.value,
    required this.label,
    required this.isDark,
    this.icon,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 16, color: AppColors.neutralGray500),
            const SizedBox(height: 2),
          ],
          Text(
            value,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }
}

/// Rating stat item with stars
class _RatingStatItem extends StatelessWidget {
  final double? rating;
  final int reviewCount;
  final bool isDark;
  final VoidCallback? onTap;

  const _RatingStatItem({
    required this.rating,
    required this.reviewCount,
    required this.isDark,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Stars row
          Row(
            mainAxisSize: MainAxisSize.min,
            children: List.generate(5, (index) {
              final starPosition = index + 1;
              final ratingValue = rating ?? 0.0;
              return Icon(
                starPosition <= ratingValue.round()
                    ? Icons.star
                    : Icons.star_border,
                size: 12,
                color: starPosition <= ratingValue.round()
                    ? Colors.amber
                    : (isDark
                          ? AppColors.neutralGray600
                          : AppColors.neutralGray400),
              );
            }),
          ),
          const SizedBox(height: 2),
          Text(
            rating == null ? '-' : rating!.toStringAsFixed(1),
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            reviewCount > 0 ? 'Rating ($reviewCount)' : 'Rating',
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }
}
