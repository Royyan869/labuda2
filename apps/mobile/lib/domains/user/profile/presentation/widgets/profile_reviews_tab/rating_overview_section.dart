import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
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
                    fontSize: 36,
                    fontWeight: FontWeight.bold,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                _buildStarRating(averageRating, 16),
                const SizedBox(height: 3),
                Text(
                  '$totalReviews reviews',
                  style: TextStyle(
                    fontSize: 12,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray600,
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
                  padding: const EdgeInsets.symmetric(vertical: 1.5),
                  child: Row(
                    children: [
                      Text(
                        '$starCount',
                        style: TextStyle(
                          fontSize: 11,
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray600,
                        ),
                      ),
                      const Icon(Icons.star, size: 11, color: Colors.amber),
                      const SizedBox(width: 6),
                      Expanded(
                        child: LinearProgressIndicator(
                          value: percentage,
                          backgroundColor: isDark
                              ? AppColors.darkGray600
                              : AppColors.neutralGray200,
                          valueColor: const AlwaysStoppedAnimation<Color>(
                            Colors.amber,
                          ),
                        ),
                      ),
                      const SizedBox(width: 6),
                      SizedBox(
                        width: 22,
                        child: Text(
                          '$count',
                          style: TextStyle(
                            fontSize: 11,
                            color: isDark
                                ? AppColors.neutralGray300
                                : AppColors.neutralGray600,
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

  Widget _buildStarRating(double rating, double size) {
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
          color: Colors.amber,
        );
      }),
    );
  }
}
