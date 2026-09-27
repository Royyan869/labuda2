import 'package:flutter/material.dart';

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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      alignment: Alignment.center,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        decoration: BoxDecoration(
          color: scheme.scrim.withValues(alpha: 0.6),
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
