import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Page indicators widget untuk media viewer
class MediaViewerIndicators extends StatelessWidget {
  final int currentIndex;
  final int totalItems;

  const MediaViewerIndicators({
    super.key,
    required this.currentIndex,
    required this.totalItems,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      alignment: Alignment.center,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        decoration: BoxDecoration(
          color: AppColors.dark.withValues(alpha: 0.6),
          borderRadius: BorderRadius.circular(20),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: List.generate(
            totalItems,
            (index) => Container(
              margin: const EdgeInsets.symmetric(horizontal: 3),
              width: 6,
              height: 6,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: currentIndex == index
                    ? AppColors.light
                    : AppColors.light.withValues(alpha: 0.4),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
