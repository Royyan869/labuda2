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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    Color? bgColor = backgroundColor;
    Color? txtColor = textColor;

    if (bgColor == null) {
      switch (variant) {
        case StatusBadgeVariant.default_:
          bgColor = isDark
              ? AppColors.neutralGray700
              : AppColors.neutralGray100;
          break;
        case StatusBadgeVariant.outlined:
          bgColor = Colors.transparent;
          break;
        case StatusBadgeVariant.pill:
          bgColor = AppColors.primaryRed.withValues(alpha: 0.1);
          break;
        case StatusBadgeVariant.dot:
          bgColor = AppColors.primaryRed;
          break;
      }
    }

    if (txtColor == null) {
      switch (variant) {
        case StatusBadgeVariant.default_:
          txtColor = isDark ? AppColors.neutralWhite : AppColors.neutralGray900;
          break;
        case StatusBadgeVariant.outlined:
          txtColor = isDark
              ? AppColors.neutralGray300
              : AppColors.neutralGray700;
          break;
        case StatusBadgeVariant.pill:
          txtColor = AppColors.primaryRed;
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
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray300,
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
        return BorderRadius.circular(999);
      case StatusBadgeVariant.outlined:
        return BorderRadius.circular(16);
      case StatusBadgeVariant.dot:
        return BorderRadius.circular(999);
      default:
        return BorderRadius.circular(8);
    }
  }

  EdgeInsets _getPaddingForVariant(StatusBadgeVariant variant) {
    switch (variant) {
      case StatusBadgeVariant.pill:
        return const EdgeInsets.symmetric(horizontal: 12, vertical: 6);
      case StatusBadgeVariant.outlined:
        return const EdgeInsets.symmetric(horizontal: 12, vertical: 6);
      default:
        return const EdgeInsets.symmetric(horizontal: 10, vertical: 4);
    }
  }

  // Named constructors for common statuses
  factory StatusBadge.success(String label) {
    return StatusBadge(
      label: label,
      backgroundColor: AppColors.statusSuccess.withValues(alpha: 0.1),
      textColor: AppColors.statusSuccess,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.error(String label) {
    return StatusBadge(
      label: label,
      backgroundColor: AppColors.statusError.withValues(alpha: 0.1),
      textColor: AppColors.statusError,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.warning(String label) {
    return StatusBadge(
      label: label,
      backgroundColor: AppColors.statusWarning.withValues(alpha: 0.1),
      textColor: AppColors.statusWarning,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.info(String label) {
    return StatusBadge(
      label: label,
      backgroundColor: AppColors.statusInfo.withValues(alpha: 0.1),
      textColor: AppColors.statusInfo,
      variant: StatusBadgeVariant.pill,
    );
  }

  factory StatusBadge.outlined(String label) {
    return StatusBadge(label: label, variant: StatusBadgeVariant.outlined);
  }

  factory StatusBadge.dot({Color? color}) {
    return StatusBadge(
      label: '',
      backgroundColor: color ?? AppColors.primaryRed,
      variant: StatusBadgeVariant.dot,
    );
  }
}
