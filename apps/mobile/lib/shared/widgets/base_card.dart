import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Base Card Widget
///
/// Reusable card component with consistent styling.
/// Provides common card behavior with InkWell for tap handling.
class BaseCard extends StatelessWidget {
  final Widget child;
  final VoidCallback? onTap;
  final EdgeInsetsGeometry? padding;
  final EdgeInsetsGeometry? margin;
  final double? elevation;
  final Color? backgroundColor;
  final Color? borderColor;
  final double? borderRadius;
  final bool isRounded;
  final bool showBorder;
  final bool showShadow;
  final double? width;
  final double? height;

  const BaseCard({
    super.key,
    required this.child,
    this.onTap,
    this.padding,
    this.margin,
    this.elevation,
    this.backgroundColor,
    this.borderColor,
    this.borderRadius,
    this.isRounded = true,
    this.showBorder = false,
    this.showShadow = true,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final card = Container(
      width: width,
      height: height,
      margin: margin,
      padding: padding ?? const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color:
            backgroundColor ??
            (isDark ? AppColors.neutralGray800 : AppColors.neutralWhite),
        borderRadius: BorderRadius.circular(borderRadius ?? 12),
        border: showBorder
            ? Border.all(
                color:
                    borderColor ??
                    (isDark
                        ? AppColors.neutralGray700
                        : AppColors.neutralGray200),
              )
            : null,
        boxShadow: showShadow
            ? [
                BoxShadow(
                  color:
                      (isDark
                              ? AppColors.neutralBlack
                              : AppColors.neutralGray900)
                          .withValues(alpha: 0.05),
                  blurRadius: 10,
                  offset: const Offset(0, 2),
                ),
              ]
            : null,
      ),
      child: child,
    );

    if (onTap != null) {
      return Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(borderRadius ?? 12),
          child: card,
        ),
      );
    }

    return card;
  }

  /// Named constructors for common variants
  static BaseCard compact({
    Key? key,
    required Widget child,
    VoidCallback? onTap,
    EdgeInsetsGeometry? margin,
  }) {
    return BaseCard(
      key: key,
      onTap: onTap,
      margin: margin,
      padding: const EdgeInsets.all(12),
      borderRadius: 12,
      child: child,
    );
  }

  static BaseCard spacious({
    Key? key,
    required Widget child,
    VoidCallback? onTap,
    EdgeInsetsGeometry? margin,
  }) {
    return BaseCard(
      key: key,
      onTap: onTap,
      margin: margin,
      padding: const EdgeInsets.all(20),
      child: child,
    );
  }

  static BaseCard bordered({
    Key? key,
    required Widget child,
    VoidCallback? onTap,
    Color? borderColor,
    EdgeInsetsGeometry? margin,
  }) {
    return BaseCard(
      key: key,
      onTap: onTap,
      margin: margin,
      showBorder: true,
      showShadow: false,
      borderColor: borderColor,
      child: child,
    );
  }
}
