import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/promoted_badge.dart';

/// Small promoted section shown near the top of Explore tabs.
class ExplorePromotedSection extends StatelessWidget {
  final String title;
  final List<Widget> children;

  const ExplorePromotedSection({
    super.key,
    required this.title,
    required this.children,
  });

  @override
  Widget build(BuildContext context) {
    if (children.isEmpty) {
      return const SizedBox.shrink();
    }

    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              PromotedBadge.chip(),
              const SizedBox(width: 8),
              Text(
                title,
                style: AppTypography.h6.copyWith(
                  fontWeight: FontWeight.w700,
                  color: isDark
                      ? AppColors.neutralGray100
                      : AppColors.neutralGray900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          ...children.map(
            (child) => Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: child,
            ),
          ),
        ],
      ),
    );
  }
}
