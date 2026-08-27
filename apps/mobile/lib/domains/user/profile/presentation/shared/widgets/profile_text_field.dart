import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Profile text field for consistent form styling
///
/// Provides consistent styling for all profile form fields.
/// Uses existing TextField with profile-specific defaults.
///
/// ## 🚨 ARCHITECTURE LOCK - READ BEFORE USING 🚨
///
/// ✅ USE this widget for:
/// - Standard text input fields (name, email, phone, etc.)
/// - Fields with synchronous validation (validator function)
/// - Fields that need consistent profile styling
///
/// ❌ DO NOT use this widget for:
/// - Password fields → Create specialized widget if needed
/// - Async verification (phone verification check) → Create custom widget
/// - Fields with complex inline state → Create custom widget
///
/// ## When to Create a Custom Widget
///
/// If your field needs:
/// - Async verification with loading states → Custom widget (e.g., `PhoneVerificationField`)
/// - Real-time validation indicators → Custom widget with inline state
/// - Complex focus-based UI → Custom widget managing own state
///
/// Pattern for async verification widgets:
/// ```dart
/// class PhoneVerificationField extends StatefulWidget {
///   // Widget manages own async state:
///   // - bool _isVerifying
///   // - bool _isVerified
///   //
///   // Callback aggregated result to parent:
///   // widget.onVerificationChanged(isVerified)
/// }
/// ```
///
/// Example usage:
/// ```dart
/// // Phone field
/// ProfileTextField.phone(
///   controller: _phoneController,
///   validator: (value) => validatePhone(value),
/// )
///
/// // Email field (read-only display)
/// ProfileTextField.email(
///   value: userEmail,
///   readOnly: true,
/// )
///
/// // Custom field
/// ProfileTextField(
///   controller: _controller,
///   labelText: 'Custom',
///   prefixIcon: Icons.custom,
/// )
/// ```
///
/// @see REFACTOR_UI.md section 11 for non-auth form guidelines
class ProfileTextField extends StatelessWidget {
  final TextEditingController? controller;
  final String? value;
  final String? labelText;
  final String? hintText;
  final String? Function(String?)? validator;
  final TextInputType? keyboardType;
  final bool obscureText;
  final bool enabled;
  final bool readOnly;
  final IconData? prefixIcon;
  final Widget? suffixIcon;
  final VoidCallback? onSuffixTap;
  final FocusNode? focusNode;
  final void Function(String)? onChanged;
  final int? maxLines;
  final int? maxLength;
  final TextInputAction? textInputAction;
  final TextCapitalization textCapitalization;

  const ProfileTextField({
    super.key,
    this.controller,
    this.value,
    this.labelText,
    this.hintText,
    this.validator,
    this.keyboardType,
    this.obscureText = false,
    this.enabled = true,
    this.readOnly = false,
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

  /// Phone field with common defaults
  factory ProfileTextField.phone({
    Key? key,
    TextEditingController? controller,
    String? labelText,
    String? hintText,
    String? Function(String?)? validator,
    bool enabled = true,
    FocusNode? focusNode,
    void Function(String)? onChanged,
  }) {
    return ProfileTextField(
      key: key,
      controller: controller,
      labelText: labelText ?? 'Phone Number',
      hintText: hintText ?? '081234567890',
      keyboardType: TextInputType.phone,
      prefixIcon: Icons.phone_outlined,
      validator: validator,
      enabled: enabled,
      focusNode: focusNode,
      onChanged: onChanged,
    );
  }

  /// Email field (typically read-only for login email)
  factory ProfileTextField.email({
    Key? key,
    String? value,
    TextEditingController? controller,
    String? labelText,
    String? hintText,
    bool readOnly = true,
    bool enabled = true,
  }) {
    return ProfileTextField(
      key: key,
      value: value,
      controller: controller,
      labelText: labelText ?? 'Email',
      hintText: hintText ?? 'name@email.com',
      keyboardType: TextInputType.emailAddress,
      prefixIcon: Icons.email_outlined,
      readOnly: readOnly,
      enabled: enabled,
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // If value is provided (for read-only display), use TextFormField with initial value
    if (value != null && readOnly) {
      return TextFormField(
        initialValue: value,
        keyboardType: keyboardType,
        readOnly: true,
        enabled: false,
        style: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w500,
          color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
        ),
        decoration: _buildDecoration(context, isDark),
      );
    }

    return TextFormField(
      controller: controller,
      focusNode: focusNode,
      keyboardType: keyboardType,
      obscureText: obscureText,
      maxLines: maxLines,
      maxLength: maxLength,
      enabled: enabled,
      readOnly: readOnly,
      onChanged: onChanged,
      textCapitalization: textCapitalization,
      textInputAction: textInputAction,
      style: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w500,
        color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
      ),
      decoration: _buildDecoration(context, isDark),
      validator: validator,
    );
  }

  InputDecoration _buildDecoration(BuildContext context, bool isDark) {
    return InputDecoration(
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
      disabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      filled: true,
      fillColor: readOnly
          ? (isDark ? AppColors.darkGray800 : AppColors.neutralGray50)
          : (isDark ? AppColors.darkGray700 : AppColors.neutralGray50),
    );
  }
}
