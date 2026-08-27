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
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (icon != null) ...[
                Icon(icon, size: 20),
                const SizedBox(width: 12),
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
      width: width ?? double.infinity,
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
      case AuthButtonType.primary:
        return ElevatedButton(
          onPressed: isActive ? onPressed : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: isActive
                ? AppColors.primaryRed
                : (isDark
                      ? AppColors.neutralGray600
                      : AppColors.neutralGray300),
            foregroundColor: isActive
                ? AppColors.neutralWhite
                : (isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray500),
            elevation: 0,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
          child: child,
        );

      case AuthButtonType.secondary:
        return OutlinedButton(
          onPressed: isActive ? onPressed : null,
          style: OutlinedButton.styleFrom(
            foregroundColor: isActive
                ? (isDark ? AppColors.neutralWhite : AppColors.neutralGray800)
                : (isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray400),
            side: BorderSide(
              color: isActive
                  ? (isDark ? AppColors.darkGray600 : AppColors.neutralGray300)
                  : (isDark ? AppColors.darkGray600 : AppColors.neutralGray300),
              width: 1.5,
            ),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
          child: child,
        );
    }
  }
}

enum AuthButtonType { primary, secondary }
