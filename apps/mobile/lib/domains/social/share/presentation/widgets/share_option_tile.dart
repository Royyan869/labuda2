import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/share_destination.dart';
import 'share_destination_extensions.dart';

/// List tile for each share destination option
class ShareOptionTile extends StatelessWidget {
  final ShareDestination destination;
  final VoidCallback onTap;
  final bool showDivider;
  final bool isDark;

  const ShareOptionTile({
    super.key,
    required this.destination,
    required this.onTap,
    this.showDivider = true,
    this.isDark = false,
  });

  @override
  Widget build(BuildContext context) {
    final textColor = isDark
        ? AppColors.neutralGray100
        : AppColors.neutralGray900;
    final iconBgColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray100;
    final iconColor = isDark
        ? AppColors.neutralGray400
        : AppColors.neutralGray600;
    final dividerColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;
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
            color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
          ),
        ),
        if (showDivider)
          Divider(height: 1, indent: 88, endIndent: 24, color: dividerColor),
      ],
    );
  }
}
