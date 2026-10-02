import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Rating overview section showing overall rating and breakdown
class RatingOverviewSection extends StatelessWidget {
  final double averageRating;
  final int totalReviews;
  final Map<int, int> ratingBreakdown;

  const RatingOverviewSection({
    super.key,
    required this.averageRating,
    required this.totalReviews,
    required this.ratingBreakdown,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          bottom: BorderSide(
            color: scheme.outlineVariant,
          ),
        ),
      ),
      child: Row(
        children: [
          // Overall rating
          Expanded(
            flex: 2,
            child: Column(
              children: [
                Text(
                  averageRating.toStringAsFixed(1),
                  style: TextStyle(
                    fontSize: AppType.s24,
                    fontWeight: FontWeight.bold,
                    color: scheme.onSurface,
                  ),
                ),
                _buildStarRating(context, averageRating, 16),
                const SizedBox(height: 3),
                Text(
                  '$totalReviews reviews',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),

          const SizedBox(width: 16),

          // Rating breakdown
          Expanded(
            flex: 3,
            child: Column(
              children: List.generate(5, (index) {
                final starCount = 5 - index;
                final count = ratingBreakdown[starCount] ?? 0;
                final percentage = totalReviews > 0
                    ? count / totalReviews
                    : 0.0;

                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p4),
                  child: Row(
                    children: [
                      Text(
                        '$starCount',
                        style: TextStyle(
                          fontSize: AppType.s12,
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      Icon(Icons.star, size: AppIconSize.inlineGlyph, color: AppColors.koiGold),
                      const SizedBox(width: 6),
                      Expanded(
                        child: LinearProgressIndicator(
                          value: percentage,
                          backgroundColor: scheme.surfaceContainerHighest,
                          valueColor: const AlwaysStoppedAnimation<Color>(
                            AppColors.koiGold,
                          ),
                        ),
                      ),
                      const SizedBox(width: 6),
                      SizedBox(
                        width: AppContentSize.badge,
                        child: Text(
                          '$count',
                          style: TextStyle(
                            fontSize: AppType.s12,
                            color: scheme.onSurfaceVariant,
                          ),
                          textAlign: TextAlign.end,
                        ),
                      ),
                    ],
                  ),
                );
              }),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStarRating(BuildContext context, double rating, double size) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (index) {
        return Icon(
          index < rating.floor()
              ? Icons.star
              : index < rating
              ? Icons.star_half
              : Icons.star_border,
          size: size,
          color: AppColors.koiGold,
        );
      }),
    );
  }
}
