import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Empty state widget for reviews
class ReviewsEmptyState extends StatelessWidget {
  const ReviewsEmptyState({super.key});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.rate_review_outlined,
            size: 64,
            color: scheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            'No Reviews Yet',
            style: TextStyle(
              fontSize: AppType.s18,
              fontWeight: FontWeight.w600,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Be the first to review this seller',
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
