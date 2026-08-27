import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'detail_chip_types.dart';

/// Shared Detail Chip Widget untuk menampilkan informasi dengan color-coded styling
///
/// Features:
/// - Consistent styling across app
/// - Color-coded categories (price, size, variety, location, etc.)
/// - Instagram-style compact design
/// - Support untuk icons dan labels
/// - Responsive sizing dan typography
/// - Theme-aware styling
class DetailChipWidget extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback? onTap;
  final DetailChipSize size;
  final DetailChipStyle style;
  final bool showIcon;

  const DetailChipWidget({
    super.key,
    required this.icon,
    required this.label,
    required this.color,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
    this.showIcon = true,
  });

  /// Price chip untuk budget/pricing information
  const DetailChipWidget.price({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
  }) : icon = Icons.attach_money,
       color = AppColors.success,
       showIcon = true;

  /// Size chip untuk dimensi/ukuran
  const DetailChipWidget.size({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
  }) : icon = Icons.straighten,
       color = AppColors.primary,
       showIcon = true;

  /// Variety chip untuk kategori/jenis
  const DetailChipWidget.variety({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
  }) : icon = Icons.local_offer,
       color = Colors.purple,
       showIcon = true;

  /// Location chip untuk lokasi
  const DetailChipWidget.location({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
  }) : icon = Icons.location_on,
       color = AppColors.warning,
       showIcon = true;

  /// Status chip untuk status information
  const DetailChipWidget.status({
    super.key,
    required this.label,
    required this.color,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
  }) : icon = Icons.circle,
       showIcon = true;

  /// Tag chip untuk hashtags/labels
  const DetailChipWidget.tag({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.small,
    this.style = DetailChipStyle.outlined,
  }) : icon = Icons.tag,
       color = AppColors.neutral,
       showIcon = false;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: DetailChipStyleUtils.getPadding(size),
        decoration: DetailChipStyleUtils.getDecoration(style, color, size),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (showIcon) ...[
              Icon(
                icon,
                size: DetailChipStyleUtils.getIconSize(size),
                color: DetailChipStyleUtils.getContentColor(
                  style,
                  color,
                  isDark,
                ),
              ),
              SizedBox(width: DetailChipStyleUtils.getSpacing(size)),
            ],
            Flexible(
              child: Text(
                label,
                style: _getTextStyle(theme, isDark),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }

  TextStyle? _getTextStyle(ThemeData theme, bool isDark) {
    final baseStyle = size == DetailChipSize.small
        ? theme.textTheme.labelSmall
        : theme.textTheme.bodySmall;

    return baseStyle?.copyWith(
      color: DetailChipStyleUtils.getContentColor(style, color, isDark),
      fontWeight: FontWeight.w500,
      fontSize: DetailChipStyleUtils.getFontSize(size),
    );
  }
}
