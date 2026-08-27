import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Empty state widget for reviews
class ReviewsEmptyState extends StatelessWidget {
  const ReviewsEmptyState({super.key});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.rate_review_outlined,
            size: 64,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          ),
          const SizedBox(height: 16),
          Text(
            'No Reviews Yet',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Be the first to review this seller',
            style: TextStyle(
              fontSize: 14,
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
