import 'package:hishumi/core/src/theme/app_theme.dart';
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
  /// Caller-bound tint. [DetailChipWidget.size] leaves it null so the default
  /// resolves from the scheme in build — the primary is a theme decision,
  /// never a raw brand token bound by this widget (same hex, both modes).
  final Color? color;

  /// Theme role used when [color] is null; callers that bind a colour
  /// ([DetailChipWidget.status] / [DetailChipWidget.tag]) ignore it.
  final DetailChipTone tone;
  final VoidCallback? onTap;
  final DetailChipSize size;
  final DetailChipStyle style;
  final bool showIcon;

  const DetailChipWidget({
    super.key,
    required this.icon,
    required this.label,
    required this.color,
    this.tone = DetailChipTone.primary,
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
    this.tone = DetailChipTone.success,
  }) : icon = Icons.attach_money,
       color = null,
       showIcon = true;

  /// Size chip untuk dimensi/ukuran
  const DetailChipWidget.size({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
    this.tone = DetailChipTone.primary,
  }) : icon = Icons.straighten,
       color = null,
       showIcon = true;

  /// Variety chip untuk kategori/jenis
  const DetailChipWidget.variety({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
    this.tone = DetailChipTone.tertiary,
  }) : icon = Icons.local_offer,
       color = null,
       showIcon = true;

  /// Location chip untuk lokasi
  const DetailChipWidget.location({
    super.key,
    required this.label,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
    this.tone = DetailChipTone.warning,
  }) : icon = Icons.location_on,
       color = null,
       showIcon = true;

  /// Status chip untuk status information
  const DetailChipWidget.status({
    super.key,
    required this.label,
    required this.color,
    this.onTap,
    this.size = DetailChipSize.medium,
    this.style = DetailChipStyle.filled,
    this.tone = DetailChipTone.primary,
  }) : icon = Icons.circle,
       showIcon = true;

  /// Tag chip untuk hashtags/labels.
  ///
  /// The tint color is caller-bound (no dormant palette default): a fixed
  /// gray cannot stay readable in both modes, so the caller owns the chip
  /// identity like [DetailChipWidget.status] does.
  const DetailChipWidget.tag({
    super.key,
    required this.label,
    required this.color,
    this.onTap,
    this.size = DetailChipSize.small,
    this.style = DetailChipStyle.outlined,
    this.tone = DetailChipTone.primary,
  }) : icon = Icons.tag,
       showIcon = false;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    // Nullable by design: an unset tint is resolved from the theme role,
    // so the chip never owns a colour decision.
    final tint = color ?? _resolveTone(context.statusColors, scheme);

    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: DetailChipStyleUtils.getPadding(size),
        decoration: DetailChipStyleUtils.getDecoration(style, tint, size),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (showIcon) ...[
              Icon(
                icon,
                size: DetailChipStyleUtils.getIconSize(size),
                color: DetailChipStyleUtils.getContentColor(
                  style,
                  tint,
                  scheme,
                ),
              ),
              SizedBox(width: DetailChipStyleUtils.getSpacing(size)),
            ],
            Flexible(
              child: Text(
                label,
                style: _getTextStyle(theme, scheme, tint),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Color _resolveTone(AppStatusColors status, ColorScheme scheme) =>
      switch (tone) {
        DetailChipTone.success => status.success,
        DetailChipTone.warning => status.warning,
        DetailChipTone.primary => scheme.primary,
        DetailChipTone.tertiary => scheme.tertiary,
      };

  TextStyle? _getTextStyle(ThemeData theme, ColorScheme scheme, Color tint) {
    final baseStyle = size == DetailChipSize.small
        ? theme.textTheme.labelSmall
        : theme.textTheme.bodySmall;

    return baseStyle?.copyWith(
      color: DetailChipStyleUtils.getContentColor(style, tint, scheme),
      fontWeight: FontWeight.w500,
      fontSize: DetailChipStyleUtils.getFontSize(size),
    );
  }
}
