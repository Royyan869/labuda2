import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Page indicators dan counter untuk media carousel
///
/// Features:
/// - Dot indicators
/// - Image counter
/// - Customizable positioning
class CarouselIndicators extends StatelessWidget {
  final int currentIndex;
  final int totalItems;
  final bool showIndicators;
  final bool showCounter;

  const CarouselIndicators({
    super.key,
    required this.currentIndex,
    required this.totalItems,
    this.showIndicators = true,
    this.showCounter = true,
  });

  @override
  Widget build(BuildContext context) {
    // Only show dots indicator at bottom - no counter, no top indicators
    if (!showIndicators || totalItems <= 1) {
      return const SizedBox.shrink();
    }

    return Positioned(
      bottom: 12,
      left: 0,
      right: 0,
      child: _buildPageIndicators(),
    );
  }

  Widget _buildPageIndicators() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
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
    );
  }
}

/// Standalone page indicators widget
class MediaPageIndicators extends StatelessWidget {
  final int currentIndex;
  final int totalItems;

  const MediaPageIndicators({
    super.key,
    required this.currentIndex,
    required this.totalItems,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
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
    );
  }
}

/// Standalone image counter widget
class MediaCounter extends StatelessWidget {
  final int currentIndex;
  final int totalItems;

  const MediaCounter({
    super.key,
    required this.currentIndex,
    required this.totalItems,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: AppColors.dark.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        '${currentIndex + 1}/$totalItems',
        style: const TextStyle(
          color: AppColors.light,
          fontSize: 12,
          fontWeight: FontWeight.w500,
        ),
      ),
    );
  }
}
