import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Authentication text field wrapper around AppTextField
///
/// Provides consistent styling for all authentication form fields.
/// Uses existing AppTextField with auth-specific defaults.
///
/// ## 🚨 ARCHITECTURE LOCK - READ BEFORE USING 🚨
///
/// ✅ USE this widget for:
/// - Standard text input fields (name, email, username, etc.)
/// - Fields with synchronous validation (validator function)
/// - Fields that need consistent auth styling
///
/// ❌ DO NOT use this widget for:
/// - Password fields → Use `AuthPasswordField` instead
/// - Async validation (username availability check) → Create custom widget
/// - Fields with complex inline state → Create custom widget
///
/// ## When to Create a Custom Widget
///
/// If your field needs:
/// - Async validation with loading states → Custom widget (e.g., `UsernameField`)
/// - Real-time strength indicators → Custom widget with inline state
/// - Complex focus-based UI → Custom widget managing own state
///
/// Pattern for async validation widgets:
/// ```dart
/// class UsernameField extends StatefulWidget {
///   // Widget manages own async state:
///   // - UsernameCheckResult? _checkResult
///   // - bool _isChecking
///   //
///   // Callback aggregated result to parent:
///   // widget.onValidationChanged(isValid, isAvailable)
/// }
/// ```
///
/// Example usage:
/// ```dart
/// // Email field
/// AuthTextField.email(
///   controller: _emailController,
///   validator: (value) => validateEmail(value),
/// )
///
/// // Username field (sync validation only)
/// AuthTextField.username(
///   controller: _usernameController,
/// )
///
/// // Custom field
/// AuthTextField(
///   controller: _controller,
///   labelText: 'Custom',
///   prefixIcon: Icons.custom,
/// )
/// ```
///
/// @see REFACTOR_UI.md section 10.9 for reusable component decision tree
class AuthTextField extends StatelessWidget {
  final TextEditingController? controller;
  final String? labelText;
  final String? hintText;
  final String? Function(String?)? validator;
  final TextInputType? keyboardType;
  final bool obscureText;
  final bool enabled;
  final IconData? prefixIcon;
  final Widget? suffixIcon;
  final VoidCallback? onSuffixTap;
  final FocusNode? focusNode;
  final void Function(String)? onChanged;
  final int? maxLines;
  final int? maxLength;
  final TextInputAction? textInputAction;
  final TextCapitalization textCapitalization;

  const AuthTextField({
    super.key,
    this.controller,
    this.labelText,
    this.hintText,
    this.validator,
    this.keyboardType,
    this.obscureText = false,
    this.enabled = true,
    this.prefixIcon,
    this.suffixIcon,
    this.onSuffixTap,
    this.focusNode,
    this.onChanged,
    this.maxLines = 1,
    this.maxLength,
    this.textInputAction,
    this.textCapitalization = TextCapitalization.none,
  });

  /// Email field with common defaults
  factory AuthTextField.email({
    Key? key,
    TextEditingController? controller,
    String? labelText,
    String? hintText,
    String? Function(String?)? validator,
    bool enabled = true,
    FocusNode? focusNode,
    void Function(String)? onChanged,
  }) {
    return AuthTextField(
      key: key,
      controller: controller,
      labelText: labelText ?? 'Email',
      hintText: hintText ?? 'name@email.com',
      keyboardType: TextInputType.emailAddress,
      prefixIcon: Icons.email_outlined,
      validator: validator,
      enabled: enabled,
      focusNode: focusNode,
      onChanged: onChanged,
    );
  }

  /// Username field with common defaults
  factory AuthTextField.username({
    Key? key,
    TextEditingController? controller,
    String? labelText,
    String? hintText,
    String? Function(String?)? validator,
    bool enabled = true,
    FocusNode? focusNode,
    void Function(String)? onChanged,
  }) {
    return AuthTextField(
      key: key,
      controller: controller,
      labelText: labelText ?? 'Username',
      hintText: hintText ?? 'Enter username',
      keyboardType: TextInputType.text,
      prefixIcon: Icons.alternate_email,
      validator: validator,
      enabled: enabled,
      focusNode: focusNode,
      onChanged: onChanged,
      textCapitalization: TextCapitalization.none,
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return TextFormField(
      controller: controller,
      focusNode: focusNode,
      keyboardType: keyboardType,
      obscureText: obscureText,
      maxLines: maxLines,
      maxLength: maxLength,
      enabled: enabled,
      onChanged: onChanged,
      textCapitalization: textCapitalization,
      textInputAction: textInputAction,
      decoration: InputDecoration(
        labelText: labelText,
        hintText: hintText,
        floatingLabelBehavior: FloatingLabelBehavior.always,
        hintStyle: TextStyle(
          color: isDark
              ? AppColors.neutralGray400.withValues(alpha: 0.6)
              : AppColors.neutralGray500.withValues(alpha: 0.6),
        ),
        prefixIcon: prefixIcon != null
            ? Icon(
                prefixIcon,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              )
            : null,
        suffixIcon: suffixIcon,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.primaryRed, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.error),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.error, width: 2),
        ),
        filled: true,
        fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
      ),
      validator: validator,
    );
  }
}
