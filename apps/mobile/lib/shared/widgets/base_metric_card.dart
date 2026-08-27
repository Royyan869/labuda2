import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Base Metric Card Widget
///
/// Reusable metric card for displaying key metrics.
/// Commonly used in dashboard, seller stats, collection stats, etc.
class BaseMetricCard extends StatelessWidget {
  final String label;
  final String value;
  final String? subtitle;
  final IconData? icon;
  final Color? iconColor;
  final Color? backgroundColor;
  final VoidCallback? onTap;
  final bool showTrend;
  final String? trendValue;
  final bool isPositiveTrend;
  final Widget? trailing;
  final EdgeInsetsGeometry? padding;
  final double? width;

  const BaseMetricCard({
    super.key,
    required this.label,
    required this.value,
    this.subtitle,
    this.icon,
    this.iconColor,
    this.backgroundColor,
    this.onTap,
    this.showTrend = false,
    this.trendValue,
    this.isPositiveTrend = true,
    this.trailing,
    this.padding,
    this.width,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      width: width,
      padding: padding ?? const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color:
            backgroundColor ??
            (isDark ? AppColors.neutralGray800 : AppColors.neutralWhite),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray200,
          width: 1,
        ),
        boxShadow: [
          BoxShadow(
            color: (isDark ? AppColors.neutralBlack : AppColors.neutralGray900)
                .withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Label and trend row
          Row(
            children: [
              Expanded(
                child: Text(
                  label,
                  style: AppTypography.caption.copyWith(
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ),
              if (showTrend && trendValue != null) ...[
                const SizedBox(width: 4),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 6,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: isPositiveTrend
                        ? AppColors.statusSuccess.withValues(alpha: 0.1)
                        : AppColors.statusError.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(999),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        isPositiveTrend
                            ? Icons.arrow_upward
                            : Icons.arrow_downward,
                        size: 10,
                        color: isPositiveTrend
                            ? AppColors.statusSuccess
                            : AppColors.statusError,
                      ),
                      const SizedBox(width: 2),
                      Text(
                        trendValue!,
                        style: AppTypography.labelSmall.copyWith(
                          color: isPositiveTrend
                              ? AppColors.statusSuccess
                              : AppColors.statusError,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
              ?trailing,
            ],
          ),
          const SizedBox(height: 8),

          // Value and icon row
          Row(
            children: [
              if (icon != null) ...[
                Icon(
                  icon,
                  size: 24,
                  color:
                      iconColor ??
                      (isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray500),
                ),
                const SizedBox(width: 12),
              ],
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      value,
                      style: AppTypography.h3.copyWith(
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    if (subtitle != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        subtitle!,
                        style: AppTypography.caption.copyWith(
                          color: isDark
                              ? AppColors.neutralGray500
                              : AppColors.neutralGray400,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  /// Compact variant with smaller padding and no border
  factory BaseMetricCard.compact({
    Key? key,
    required String label,
    required String value,
    String? subtitle,
    IconData? icon,
    Color? iconColor,
    VoidCallback? onTap,
    double? width,
  }) {
    return BaseMetricCard(
      key: key,
      label: label,
      value: value,
      subtitle: subtitle,
      icon: icon,
      iconColor: iconColor,
      onTap: onTap,
      width: width,
      padding: const EdgeInsets.all(12),
    );
  }

  /// Minimal variant - just value and label
  factory BaseMetricCard.minimal({
    Key? key,
    required String label,
    required String value,
    IconData? icon,
    Color? iconColor,
    double? width,
  }) {
    return BaseMetricCard(
      key: key,
      label: label,
      value: value,
      icon: icon,
      iconColor: iconColor,
      width: width,
      padding: const EdgeInsets.all(10),
    );
  }
}
