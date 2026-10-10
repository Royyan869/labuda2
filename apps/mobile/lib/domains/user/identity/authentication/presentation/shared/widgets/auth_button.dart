import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';

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
    this.height = AppContentSize.control,
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
    this.height = AppContentSize.control,
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
    this.height = AppContentSize.control,
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
    this.height = AppContentSize.control,
  }) : type = AuthButtonType.secondary,
       isLoading = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final isActive = isEnabled && !isLoading;

    // Fill / ink / disabled treatment / geometry are owned by the button themes
    // (AppTheme). This component owns only the auth-domain composition: the
    // loading spinner, the enabled gate, and the icon+label row.
    final Widget child = isLoading
        ? SizedBox(
            height: 20,
            width: 20,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(
                scheme.onSurfaceVariant,
              ),
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
                style: Theme.of(
                  context,
                ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
              ),
            ],
          );

    return SizedBox(
      width: width ?? double.infinity,
      height: height,
      child: switch (type) {
        AuthButtonType.primary => ElevatedButton(
          onPressed: isActive ? onPressed : null,
          child: child,
        ),
        AuthButtonType.secondary => OutlinedButton(
          onPressed: isActive ? onPressed : null,
          child: child,
        ),
      },
    );
  }
}

enum AuthButtonType { primary, secondary }
