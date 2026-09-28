import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Status Badge Widget
///
/// Reusable badge for displaying status information.
/// Supports multiple variants and colors.
enum StatusBadgeVariant { default_, outlined, pill, dot }

class StatusBadge extends StatelessWidget {
  final String label;
  final Color? backgroundColor;
  final Color? textColor;
  final StatusBadgeVariant variant;
  final IconData? icon;
  final double? fontSize;
  final EdgeInsetsGeometry? padding;

  const StatusBadge({
    super.key,
    required this.label,
    this.backgroundColor,
    this.textColor,
    this.variant = StatusBadgeVariant.default_,
    this.icon,
    this.fontSize,
    this.padding,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    Color? bgColor = backgroundColor;
    Color? txtColor = textColor;

    if (bgColor == null) {
      switch (variant) {
        case StatusBadgeVariant.default_:
          bgColor = scheme.surfaceContainerHighest;
          break;
        case StatusBadgeVariant.outlined:
          bgColor = Colors.transparent;
          break;
        case StatusBadgeVariant.pill:
          bgColor = scheme.primary.withValues(alpha: 0.12);
          break;
        case StatusBadgeVariant.dot:
          bgColor = scheme.primary;
          break;
      }
    }

    if (txtColor == null) {
      switch (variant) {
        case StatusBadgeVariant.default_:
          txtColor = scheme.onSurface;
          break;
        case StatusBadgeVariant.outlined:
          txtColor = scheme.onSurfaceVariant;
          break;
        case StatusBadgeVariant.pill:
          txtColor = scheme.primary;
          break;
        case StatusBadgeVariant.dot:
          // Dot variant doesn't show text
          break;
      }
    }

    // Dot variant - just a colored circle
    if (variant == StatusBadgeVariant.dot) {
      return Container(
        width: 8,
        height: 8,
        decoration: BoxDecoration(color: bgColor, shape: BoxShape.circle),
      );
    }

    // Default, outlined, and pill variants (dot already returned above)
    final finalBgColor = bgColor;
    final finalTxtColor = txtColor;

    Widget badge = Container(
      padding: padding ?? _getPaddingForVariant(variant),
      decoration: BoxDecoration(
        color: finalBgColor,
        borderRadius: _getBorderRadiusForVariant(variant),
        border: variant == StatusBadgeVariant.outlined
            ? Border.all(
                color: Theme.of(context).colorScheme.outlineVariant,
              )
            : null,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: fontSize ?? 12, color: finalTxtColor),
            const SizedBox(width: 4),
          ],
          Text(
            label,
            style: TextStyle(
              fontSize: fontSize ?? 12,
              fontWeight: FontWeight.w500,
              color: finalTxtColor,
            ),
          ),
        ],
      ),
    );

    return badge;
  }

  BorderRadius _getBorderRadiusForVariant(StatusBadgeVariant variant) {
    switch (variant) {
      case StatusBadgeVariant.pill:
        return BorderRadius.circular(AppShape.pill);
      case StatusBadgeVariant.outlined:
        return BorderRadius.circular(AppShape.r16);
      case StatusBadgeVariant.dot:
        return BorderRadius.circular(AppShape.pill);
      default:
        return BorderRadius.circular(AppShape.r8);
    }
  }

  EdgeInsets _getPaddingForVariant(StatusBadgeVariant variant) {
    switch (variant) {
      case StatusBadgeVariant.pill:
        return const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p6);
      case StatusBadgeVariant.outlined:
        return const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p6);
      default:
        return const EdgeInsets.symmetric(horizontal: AppMetrics.p10, vertical: AppMetrics.p4);
    }
  }

  // Named constructors for common statuses
  factory StatusBadge.success(BuildContext context, String label) {
    return StatusBadge(
      label: label,
      backgroundColor: context.statusColors.success.withValues(alpha: 0.1),
      textColor: context.statusColors.success,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.error(BuildContext context, String label) {
    return StatusBadge(
      label: label,
      backgroundColor: context.statusColors.error.withValues(alpha: 0.1),
      textColor: context.statusColors.error,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.warning(BuildContext context, String label) {
    return StatusBadge(
      label: label,
      backgroundColor: context.statusColors.warning.withValues(alpha: 0.1),
      textColor: context.statusColors.warning,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.info(BuildContext context, String label) {
    return StatusBadge(
      label: label,
      backgroundColor: context.statusColors.info.withValues(alpha: 0.1),
      textColor: context.statusColors.info,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.outlined(String label) {
    return StatusBadge(label: label, variant: StatusBadgeVariant.outlined);
  }

}
