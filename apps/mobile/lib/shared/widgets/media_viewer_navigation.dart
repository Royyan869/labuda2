import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Navigation components untuk Media Viewer
///
/// Features:
/// - Navigation arrows (left/right)
/// - Bottom page indicators (dots)
/// - Smooth page transitions
class MediaViewerNavigation extends StatelessWidget {
  final int currentIndex;
  final int totalItems;
  final VoidCallback? onPrevious;
  final VoidCallback? onNext;

  const MediaViewerNavigation({
    super.key,
    required this.currentIndex,
    required this.totalItems,
    this.onPrevious,
    this.onNext,
  });

  @override
  Widget build(BuildContext context) {
    if (totalItems <= 1) return const SizedBox.shrink();
    final scheme = Theme.of(context).colorScheme;

    return Stack(
      children: [
        // Navigation arrows
        _buildNavigationArrows(scheme),

        // Bottom page indicators
        Positioned(
          bottom: 32,
          left: 0,
          right: 0,
          child: _buildPageIndicators(scheme),
        ),
      ],
    );
  }

  Widget _buildNavigationArrows(ColorScheme scheme) {
    return Row(
      children: [
        // Left arrow
        if (currentIndex > 0)
          Positioned(
            left: 16,
            top: 0,
            bottom: 0,
            child: Center(
              child: MediaViewerNavigationButton(
                icon: Icons.chevron_left,
                onTap: onPrevious ?? () {},
              ),
            ),
          ),

        const Spacer(),

        // Right arrow
        if (currentIndex < totalItems - 1)
          Positioned(
            right: 16,
            top: 0,
            bottom: 0,
            child: Center(
              child: MediaViewerNavigationButton(
                icon: Icons.chevron_right,
                onTap: onNext ?? () {},
              ),
            ),
          ),
      ],
    );
  }

  Widget _buildPageIndicators(ColorScheme scheme) {
    return Container(
      alignment: Alignment.center,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
        decoration: BoxDecoration(
          color: scheme.scrim.withValues(alpha: 0.6),
          borderRadius: BorderRadius.circular(AppShape.r20),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: List.generate(
            totalItems,
            (index) => Container(
              margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p3),
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
        ),
      ),
    );
  }
}

/// Navigation button widget untuk arrows
class MediaViewerNavigationButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;

  const MediaViewerNavigationButton({
    super.key,
    required this.icon,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 48,
        height: 48,
        decoration: BoxDecoration(
          color: scheme.scrim.withValues(alpha: 0.6),
          shape: BoxShape.circle,
        ),
        child: Icon(icon, color: scheme.onPrimary, size: 28),
      ),
    );
  }
}

/// Widget untuk positioned navigation arrows
class MediaViewerArrows extends StatelessWidget {
  final int currentIndex;
  final int totalItems;
  final VoidCallback? onPrevious;
  final VoidCallback? onNext;

  const MediaViewerArrows({
    super.key,
    required this.currentIndex,
    required this.totalItems,
    this.onPrevious,
    this.onNext,
  });

  @override
  Widget build(BuildContext context) {
    if (totalItems <= 1) return const SizedBox.shrink();

    return Stack(
      children: [
        // Left arrow
        if (currentIndex > 0)
          Positioned(
            left: 16,
            top: 0,
            bottom: 0,
            child: Center(
              child: MediaViewerNavigationButton(
                icon: Icons.chevron_left,
                onTap: onPrevious ?? () {},
              ),
            ),
          ),

        // Right arrow
        if (currentIndex < totalItems - 1)
          Positioned(
            right: 16,
            top: 0,
            bottom: 0,
            child: Center(
              child: MediaViewerNavigationButton(
                icon: Icons.chevron_right,
                onTap: onNext ?? () {},
              ),
            ),
          ),
      ],
    );
  }
}
