import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable Button dengan styling konsisten
///
/// Features:
/// - Primary, secondary, outlined variants
/// - Loading state dengan indicator
/// - Disabled state dengan visual feedback
/// - Consistent sizing dan styling
/// - Adaptive theming
class AppButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final bool isLoading;
  final bool isEnabled;
  final AppButtonType type;
  final double? width;
  final double height;
  final IconData? icon;

  const AppButton({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.type = AppButtonType.primary,
    this.width,
    this.height = 52,
    this.icon,
  });

  /// Primary button (filled, red background)
  const AppButton.primary({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.width,
    this.height = 52,
    this.icon,
  }) : type = AppButtonType.primary;

  /// Secondary button (outlined, transparent background)
  const AppButton.secondary({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.width,
    this.height = 52,
    this.icon,
  }) : type = AppButtonType.secondary;

  /// Text button (no background, red text)
  const AppButton.text({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.width,
    this.height = 48,
    this.icon,
  }) : type = AppButtonType.text;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isActive = isEnabled && !isLoading;

    Widget child = isLoading
        ? const SizedBox(
            height: 20,
            width: 20,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(AppColors.neutralWhite),
            ),
          )
        : Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (icon != null) ...[
                Icon(icon, size: 20),
                const SizedBox(width: 8),
              ],
              Text(
                text,
                style: const TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          );

    return SizedBox(
      width: width,
      height: height,
      child: _buildButton(context, isDark, isActive, child),
    );
  }

  Widget _buildButton(
    BuildContext context,
    bool isDark,
    bool isActive,
    Widget child,
  ) {
    switch (type) {
      case AppButtonType.primary:
        return ElevatedButton(
          onPressed: isActive ? onPressed : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: isActive
                ? AppColors.primaryRed
                : (isDark ? AppColors.darkGray600 : AppColors.neutralGray300),
            foregroundColor: isActive
                ? AppColors.neutralWhite
                : (isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray500),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
            elevation: isActive ? 4 : 0,
            shadowColor: isActive
                ? AppColors.primaryRed.withValues(alpha: 0.3)
                : Colors.transparent,
          ),
          child: child,
        );

      case AppButtonType.secondary:
        return OutlinedButton(
          onPressed: isActive ? onPressed : null,
          style: OutlinedButton.styleFrom(
            foregroundColor: isActive
                ? AppColors.primaryRed
                : (isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray400),
            side: BorderSide(
              color: isActive
                  ? AppColors.primaryRed
                  : (isDark ? AppColors.darkGray600 : AppColors.neutralGray300),
              width: 1.5,
            ),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
          child: child,
        );

      case AppButtonType.text:
        return TextButton(
          onPressed: isActive ? onPressed : null,
          style: TextButton.styleFrom(
            foregroundColor: isActive
                ? AppColors.primaryRed
                : (isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray400),
          ),
          child: child,
        );
    }
  }
}

enum AppButtonType { primary, secondary, text }
