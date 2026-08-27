import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Section header for grouped list items
class ListSectionHeader extends StatelessWidget {
  final String title;
  final String? action;
  final VoidCallback? onAction;

  const ListSectionHeader({
    super.key,
    required this.title,
    this.action,
    this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            title.toUpperCase(),
            style: AppTypography.labelSmall.copyWith(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              fontWeight: FontWeight.w600,
              letterSpacing: 1.2,
            ),
          ),
          if (action != null)
            GestureDetector(
              onTap: onAction,
              child: Text(
                action!,
                style: AppTypography.labelSmall.copyWith(
                  color: AppColors.primaryRed,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
        ],
      ),
    );
  }
}

/// Divider for list items
class ListItemDivider extends StatelessWidget {
  final double indent;
  final double endIndent;

  const ListItemDivider({super.key, this.indent = 16, this.endIndent = 16});

  /// Divider that aligns with content after leading widget
  const ListItemDivider.afterLeading({
    super.key,
    double leadingSize = 40,
    double spacing = 12,
  }) : indent = 16 + leadingSize + spacing,
       endIndent = 16;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Divider(
      height: 1,
      thickness: 1,
      indent: indent,
      endIndent: endIndent,
      color: isDark ? AppColors.neutralGray800 : AppColors.neutralGray200,
    );
  }
}
