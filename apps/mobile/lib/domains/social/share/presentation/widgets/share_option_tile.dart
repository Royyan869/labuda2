import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/share_destination.dart';
import 'share_destination_extensions.dart';

/// List tile for each share destination option
class ShareOptionTile extends StatelessWidget {
  final ShareDestination destination;
  final VoidCallback onTap;
  final bool showDivider;

  const ShareOptionTile({
    super.key,
    required this.destination,
    required this.onTap,
    this.showDivider = true,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final textColor = scheme.onSurface;
    final iconBgColor = scheme.surfaceContainerHighest;
    final iconColor = scheme.onSurfaceVariant;
    final dividerColor = scheme.outlineVariant;
    final destinationColor = destination.color;

    return Column(
      children: [
        ListTile(
          onTap: onTap,
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 24,
            vertical: 8,
          ),
          leading: Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: destinationColor != null
                  ? destinationColor.withValues(alpha: 0.1)
                  : iconBgColor,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(
              destination.iconData,
              color: destinationColor ?? iconColor,
              size: 24,
            ),
          ),
          title: Text(
            destination.label,
            style: AppTypography.bodyLarge.copyWith(
              fontWeight: FontWeight.w500,
              color: textColor,
            ),
          ),
          trailing: Icon(
            Icons.arrow_forward_ios,
            size: 16,
            color: scheme.onSurfaceVariant,
          ),
        ),
        if (showDivider)
          Divider(height: 1, indent: 88, endIndent: 24, color: dividerColor),
      ],
    );
  }
}
