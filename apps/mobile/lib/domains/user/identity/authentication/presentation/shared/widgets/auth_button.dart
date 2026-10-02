import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Authentication button with loading state
///
/// Provides consistent button styling for auth forms.
/// Supports primary, secondary, and text variants.
///
/// Example usage:
/// ```dart
/// AuthButton.primary(
///   text: 'Sign In',
///   isLoading: controller.isLoading,
///   isEnabled: controller.isFormValid,
///   onPressed: _handleSignIn,
/// )
///
/// AuthButton.social(
///   icon: Icons.account_circle,
///   text: 'Sign in with Google',
///   onPressed: _handleGoogleSignIn,
/// )
/// ```
class AuthButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final bool isLoading;
  final bool isEnabled;
  final AuthButtonType type;
  final double? width;
  final double height;
  final IconData? icon;

  const AuthButton({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.type = AuthButtonType.primary,
    this.width,
    this.height = 52,
    this.icon,
  });

  /// Primary button (filled, red background)
  const AuthButton.primary({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.width,
    this.height = 52,
    this.icon,
  }) : type = AuthButtonType.primary;

  /// Secondary button (outlined, transparent background)
  const AuthButton.secondary({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.isEnabled = true,
    this.width,
    this.height = 52,
    this.icon,
  }) : type = AuthButtonType.secondary;

  /// Social button (outlined with icon)
  const AuthButton.social({
    super.key,
    required this.text,
    required this.icon,
    this.onPressed,
    this.isEnabled = true,
    this.width,
    this.height = 52,
  }) : type = AuthButtonType.secondary,
       isLoading = false;

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
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (icon != null) ...[
                Icon(icon, size: AppIconSize.action),
                const SizedBox(width: 12),
              ],
              Text(
                text,
                style: const TextStyle(
                  fontSize: AppType.s16,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          );

    return SizedBox(
      width: width ?? double.infinity,
      height: height,
      child: _buildButton(context, isActive, child),
    );
  }

  Widget _buildButton(
    BuildContext context,
    bool isActive,
    Widget child,
  ) {
    final scheme = Theme.of(context).colorScheme;
    switch (type) {
      case AuthButtonType.primary:
        return ElevatedButton(
          onPressed: isActive ? onPressed : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: isActive
                ? scheme.primary
                : scheme.surfaceContainerHighest,
            foregroundColor: isActive
                ? scheme.onPrimary
                : scheme.onSurfaceVariant,
            elevation: AppElevation.none,
          ),
          child: child,
        );

      case AuthButtonType.secondary:
        return OutlinedButton(
          onPressed: isActive ? onPressed : null,
          style: OutlinedButton.styleFrom(
            foregroundColor: isActive
                ? scheme.onSurface
                : scheme.onSurfaceVariant,
            side: BorderSide(
              color: scheme.outlineVariant,
              width: 1.5,
            ),
          ),
          child: child,
        );
    }
  }
}

enum AuthButtonType { primary, secondary }
