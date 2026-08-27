import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/share_destination.dart';
import 'share_destination_extensions.dart';

/// Compact grid button for share destinations
class ShareButtonGrid extends StatelessWidget {
  final List<ShareDestination> destinations;
  final Function(ShareDestination) onTap;
  final bool isDark;

  const ShareButtonGrid({
    super.key,
    required this.destinations,
    required this.onTap,
    this.isDark = false,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Wrap(
        spacing: 12,
        runSpacing: 16,
        children: destinations.map((destination) {
          return _buildButton(destination);
        }).toList(),
      ),
    );
  }

  Widget _buildButton(ShareDestination destination) {
    final textColor = isDark
        ? AppColors.neutralGray100
        : AppColors.neutralGray900;
    final iconBgColor = isDark
        ? AppColors.darkGray700
        : AppColors.neutralGray50;
    final destinationColor = destination.color;

    return SizedBox(
      width: 72,
      child: InkWell(
        onTap: () => onTap(destination),
        borderRadius: BorderRadius.circular(12),
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
                borderRadius: BorderRadius.circular(12),
              ),
              child: Icon(
                destination.iconData,
                color:
                    destinationColor ??
                    (isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700),
                size: 28,
              ),
            ),
            const SizedBox(height: 8),
            // Label
            Text(
              destination.label,
              style: AppTypography.caption.copyWith(
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
