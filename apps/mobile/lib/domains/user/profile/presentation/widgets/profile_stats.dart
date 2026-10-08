import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
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
    final scheme = Theme.of(context).colorScheme;

    // Watch follow stats stream untuk realtime updates
    final followStatsAsync = ref.watch(followStatsStreamProvider(userId));

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p12,
      ),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          top: BorderSide(color: scheme.outlineVariant),
          bottom: BorderSide(color: scheme.outlineVariant),
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
                icon: Icons.people_outline,
                onTap: () => _navigateToFollowers(context),
              ),
              loading: () => _StatItem(
                value: '-',
                label: 'Followers',
                icon: Icons.people_outline,
                onTap: () => _navigateToFollowers(context),
              ),
              error: (_, _) => _StatItem(
                value: '-',
                label: 'Followers',
                icon: Icons.people_outline,
                onTap: () => _navigateToFollowers(context),
              ),
            ),
          ),

          _buildDivider(context),

          // Following
          Expanded(
            child: followStatsAsync.when(
              data: (stats) => _StatItem(
                value: _formatCount(stats.followingCount),
                label: 'Following',
                icon: Icons.person_add_alt_outlined,
                onTap: () => _navigateToFollowing(context),
              ),
              loading: () => _StatItem(
                value: '-',
                label: 'Following',
                icon: Icons.person_add_alt_outlined,
                onTap: () => _navigateToFollowing(context),
              ),
              error: (_, _) => _StatItem(
                value: '-',
                label: 'Following',
                icon: Icons.person_add_alt_outlined,
                onTap: () => _navigateToFollowing(context),
              ),
            ),
          ),

          // Rating (seller only)
          if (isSeller) ...[
            _buildDivider(context),
            Expanded(child: _buildRatingItem(context, ref)),
          ],
        ],
      ),
    );
  }

  Widget _buildDivider(BuildContext context) {
    return Container(
      height: 32,
      width: 1,
      color: Theme.of(context).colorScheme.outlineVariant,
    );
  }

  Widget _buildRatingItem(BuildContext context, WidgetRef ref) {
    final ratingSummaryAsync = ref.watch(
      getUserRatingSummaryProvider(userId: userId),
    );

    return ratingSummaryAsync.when(
      data: (result) {
        if (result.isError || result.data == null) {
          return _RatingStatItem(
            rating: 0.0,
            reviewCount: 0,
            onTap: () => _navigateToReviews(context),
          );
        }

        final summary = result.data!;
        return _RatingStatItem(
          rating: summary.averageRating,
          reviewCount: summary.totalRatings,
          onTap: () => _navigateToReviews(context),
        );
      },
      loading: () => _RatingStatItem(
        rating: null,
        reviewCount: 0,
        onTap: () => _navigateToReviews(context),
      ),
      error: (_, _) => _RatingStatItem(
        rating: 0.0,
        reviewCount: 0,
        onTap: () => _navigateToReviews(context),
      ),
    );
  }

  void _navigateToFollowers(BuildContext context) {
    context.push(RoutePaths.followListPath(userId));
  }

  void _navigateToFollowing(BuildContext context) {
    context.push(RoutePaths.followListPath(userId, following: true));
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
  final IconData? icon;
  final VoidCallback? onTap;

  const _StatItem({
    required this.value,
    required this.label,
    this.icon,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(
              icon,
              size: AppIconSize.inlineGlyph,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(height: 2),
          ],
          Text(
            value,
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
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
  final VoidCallback? onTap;

  const _RatingStatItem({
    required this.rating,
    required this.reviewCount,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
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
                size: AppIconSize.inlineGlyph,
                color: starPosition <= ratingValue.round()
                    ? AppColors.koiGold
                    : scheme.outline,
              );
            }),
          ),
          const SizedBox(height: 2),
          Text(
            rating == null ? '-' : rating!.toStringAsFixed(1),
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            reviewCount > 0 ? 'Rating ($reviewCount)' : 'Rating',
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
