/// Repost Attribution Bar
///
/// SHARE CONTRACT V1: Displays attribution for reposted content
/// Shows when a content is a repost/share with proper attribution
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';

/// Widget that displays repost attribution
///
/// Shows:
/// - "Repost from" indicator
/// - Original author's name (if provided)
class RepostAttributionBar extends StatelessWidget {
  /// Original author ID
  final String? originalAuthorId;

  /// Original author name (if available)
  final String? originalAuthorName;

  /// Callback when tapped - should navigate to original content
  final VoidCallback? onTap;

  const RepostAttributionBar({
    super.key,
    this.originalAuthorId,
    this.originalAuthorName,
    this.onTap,
  });

  /// Whether this is a repost
  bool get isRepost => originalAuthorId != null;

  @override
  Widget build(BuildContext context) {
    if (!isRepost) {
      return const SizedBox.shrink();
    }

    final scheme = Theme.of(context).colorScheme;

    Widget attributionWidget = Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        border: Border(
          left: BorderSide(
            color: scheme.primary.withValues(alpha: 0.5),
            width: 3,
          ),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.repeat_rounded, size: AppIconSize.inlineGlyph, color: scheme.onSurfaceVariant),
          const SizedBox(width: 6),
          Flexible(
            child: Text(
              _buildAttributionText(),
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: scheme.onSurface,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
        ],
      ),
    );

    if (onTap != null) {
      return InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r4),
        child: attributionWidget,
      );
    }

    return attributionWidget;
  }

  String _buildAttributionText() {
    if (originalAuthorName != null && originalAuthorName!.isNotEmpty) {
      return 'Repost dari $originalAuthorName';
    }
    return 'Repost';
  }
}

/// Simplified repost indicator for compact spaces
class RepostIndicator extends StatelessWidget {
  final VoidCallback? onTap;

  const RepostIndicator({super.key, this.onTap});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    Widget child = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.repeat_rounded, size: AppIconSize.inlineGlyph, color: scheme.onSurfaceVariant),
        const SizedBox(width: 4),
        Text(
          'Repost',
          style: Theme.of(context).textTheme.labelSmall?.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),
      ],
    );

    if (onTap != null) {
      return InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r4),
        child: child,
      );
    }

    return child;
  }
}
