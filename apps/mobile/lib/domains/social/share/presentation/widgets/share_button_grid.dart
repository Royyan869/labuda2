import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import '../../domain/entities/share_destination.dart';
import 'share_destination_extensions.dart';

/// Compact grid button for share destinations
class ShareButtonGrid extends StatelessWidget {
  final List<ShareDestination> destinations;
  final Function(ShareDestination) onTap;

  const ShareButtonGrid({
    super.key,
    required this.destinations,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
      child: Wrap(
        spacing: 12,
        runSpacing: 16,
        children: destinations.map((destination) {
          return _buildButton(context, destination);
        }).toList(),
      ),
    );
  }

  Widget _buildButton(BuildContext context, ShareDestination destination) {
    final scheme = Theme.of(context).colorScheme;
    final textColor = scheme.onSurface;
    final iconBgColor = scheme.surfaceContainerHighest;
    final destinationColor = destination.color;

    return SizedBox(
      width: 72,
      child: InkWell(
        onTap: () => onTap(destination),
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Icon container
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: destinationColor != null
                    ? destinationColor.withValues(alpha: 0.12)
                    : iconBgColor,
                borderRadius: BorderRadius.circular(AppShape.r12),
              ),
              child: Icon(
                destination.iconData,
                color:
                    destinationColor ??
                    scheme.onSurfaceVariant,
                size: AppIconSize.emphasis,
              ),
            ),
            const SizedBox(height: 8),
            // Label
            Text(
              destination.label,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: textColor,
                fontWeight: FontWeight.w500,
              ),
              textAlign: TextAlign.center,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}
