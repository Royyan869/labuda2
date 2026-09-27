import 'package:flutter/material.dart';

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
    final scheme = Theme.of(context).colorScheme;
    final isActive = isEnabled && !isLoading;

    Widget child = isLoading
        ? SizedBox(
            height: 20,
            width: 20,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(scheme.onPrimary),
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
      child: _buildButton(context, scheme, isActive, child),
    );
  }

  Widget _buildButton(
    BuildContext context,
    ColorScheme scheme,
    bool isActive,
    Widget child,
  ) {
    switch (type) {
      case AppButtonType.primary:
        return ElevatedButton(
          onPressed: isActive ? onPressed : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: isActive
                ? scheme.primary
                : scheme.surfaceContainerHighest,
            foregroundColor: isActive
                ? scheme.onPrimary
                : scheme.onSurfaceVariant,
            disabledBackgroundColor: scheme.surfaceContainerHighest,
            disabledForegroundColor: scheme.onSurfaceVariant,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
            elevation: isActive ? 4 : 0,
            shadowColor: isActive
                ? scheme.primary.withValues(alpha: 0.3)
                : Colors.transparent,
          ),
          child: child,
        );

      case AppButtonType.secondary:
        return OutlinedButton(
          onPressed: isActive ? onPressed : null,
          style: OutlinedButton.styleFrom(
            foregroundColor: isActive
                ? scheme.primary
                : scheme.onSurfaceVariant,
            side: BorderSide(
              color: isActive ? scheme.primary : scheme.outline,
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
                ? scheme.primary
                : scheme.onSurfaceVariant,
          ),
          child: child,
        );
    }
  }
}

enum AppButtonType { primary, secondary, text }
