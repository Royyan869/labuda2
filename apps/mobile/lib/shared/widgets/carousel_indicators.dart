import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
      child: _buildPageIndicators(Theme.of(context).colorScheme),
    );
  }

  Widget _buildPageIndicators(ColorScheme scheme) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: List.generate(
        totalItems,
        (index) => Container(
          margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p4),
          width: 6,
          height: 6,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: currentIndex == index
                ? scheme.onPrimary
                : scheme.onPrimary.withValues(alpha: 0.4),
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
    final scheme = Theme.of(context).colorScheme;
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: List.generate(
        totalItems,
        (index) => Container(
          margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p4),
          width: 6,
          height: 6,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: currentIndex == index
                ? scheme.onPrimary
                : scheme.onPrimary.withValues(alpha: 0.4),
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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: scheme.scrim.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Text(
        '${currentIndex + 1}/$totalItems',
        style: context.typeRoles.labelMicro.copyWith(
          color: scheme.onPrimary,
          fontWeight: FontWeight.w500,
        ),
      ),
    );
  }
}
