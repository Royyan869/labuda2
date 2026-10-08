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
            size: AppIconSize.display,
            color: scheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            'No Reviews Yet',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Be the first to review this seller',
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
