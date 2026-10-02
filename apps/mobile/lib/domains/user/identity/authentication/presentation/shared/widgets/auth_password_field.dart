import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Authentication password field with visibility toggle
///
/// Provides consistent password input field with show/hide toggle.
/// Can optionally show a password strength indicator.
///
/// Example usage:
/// ```dart
/// AuthPasswordField(
///   controller: _passwordController,
///   labelText: 'Password',
///   isPasswordVisible: controller.isPasswordVisible,
///   onToggleVisibility: controller.togglePasswordVisibility,
///   validator: (value) => validatePassword(value),
/// )
/// ```
class AuthPasswordField extends StatefulWidget {
  final TextEditingController? controller;
  final String? labelText;
  final String? hintText;
  final String? Function(String?)? validator;
  final bool isPasswordVisible;
  final VoidCallback onToggleVisibility;
  final bool enabled;
  final FocusNode? focusNode;
  final void Function(String)? onChanged;
  final TextInputAction? textInputAction;
  final Widget? strengthIndicator;

  const AuthPasswordField({
    super.key,
    this.controller,
    this.labelText,
    this.hintText,
    this.validator,
    required this.isPasswordVisible,
    required this.onToggleVisibility,
    this.enabled = true,
    this.focusNode,
    this.onChanged,
    this.textInputAction,
    this.strengthIndicator,
  });

  @override
  State<AuthPasswordField> createState() => _AuthPasswordFieldState();
}

class _AuthPasswordFieldState extends State<AuthPasswordField> {
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        TextFormField(
          controller: widget.controller,
          focusNode: widget.focusNode,
          obscureText: !widget.isPasswordVisible,
          enabled: widget.enabled,
          onChanged: widget.onChanged,
          textInputAction: widget.textInputAction,
          decoration: InputDecoration(
            labelText: widget.labelText ?? 'Password',
            hintText: widget.hintText ?? 'Enter your password',
            floatingLabelBehavior: FloatingLabelBehavior.always,
            hintStyle: TextStyle(
              color: scheme.onSurfaceVariant.withValues(alpha: 0.6),
            ),
            prefixIcon: Icon(
              Icons.lock_outline,
              color: scheme.onSurfaceVariant,
            ),
            suffixIcon: IconButton(
              onPressed: widget.onToggleVisibility,
              icon: Icon(
                widget.isPasswordVisible
                    ? Icons.visibility_off
                    : Icons.visibility,
                color: scheme.onSurfaceVariant,
              ),
              tooltip: widget.isPasswordVisible
                  ? 'Hide password'
                  : 'Show password',
            ),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(
                color: scheme.outlineVariant,
              ),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(
                color: scheme.outlineVariant,
              ),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(
                color: scheme.primary,
                width: 2,
              ),
            ),
            errorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(color: scheme.error),
            ),
            focusedErrorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(color: scheme.error, width: 2),
            ),
            filled: true,
            fillColor: scheme.surfaceContainerHigh,
          ),
          validator: widget.validator,
        ),
        if (widget.strengthIndicator != null) ...[
          const SizedBox(height: 8),
          widget.strengthIndicator!,
        ],
      ],
    );
  }
}

/// Confirm password field with match indicator
///
/// Example usage:
/// ```dart
/// AuthConfirmPasswordField(
///   controller: _confirmPasswordController,
///   passwordController: _passwordController,
///   isVisible: controller.isConfirmPasswordVisible,
///   onToggleVisibility: controller.toggleConfirmPasswordVisibility,
/// )
/// ```
class AuthConfirmPasswordField extends StatefulWidget {
  final TextEditingController? controller;
  final TextEditingController? passwordController;
  final String? labelText;
  final String? hintText;
  final String? Function(String?)? validator;
  final bool isVisible;
  final VoidCallback onToggleVisibility;
  final bool enabled;
  final FocusNode? focusNode;
  final bool showMatchIndicator;

  const AuthConfirmPasswordField({
    super.key,
    this.controller,
    this.passwordController,
    this.labelText,
    this.hintText,
    this.validator,
    required this.isVisible,
    required this.onToggleVisibility,
    this.enabled = true,
    this.focusNode,
    this.showMatchIndicator = true,
  });

  @override
  State<AuthConfirmPasswordField> createState() =>
      _AuthConfirmPasswordFieldState();
}

class _AuthConfirmPasswordFieldState extends State<AuthConfirmPasswordField> {
  /// Listens to BOTH the confirm controller and the password controller so the
  /// match indicator updates in real time when EITHER field changes — no blur,
  /// no submit, no Form.validate() required.
  TextEditingController? _confirmController;
  TextEditingController? _passwordController;

  @override
  void initState() {
    super.initState();
    _attachListeners();
  }

  @override
  void didUpdateWidget(AuthConfirmPasswordField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller ||
        oldWidget.passwordController != widget.passwordController) {
      _detachListeners();
      _attachListeners();
    }
  }

  @override
  void dispose() {
    _detachListeners();
    super.dispose();
  }

  void _attachListeners() {
    _confirmController = widget.controller;
    _passwordController = widget.passwordController;
    _confirmController?.addListener(_onPasswordInputChanged);
    _passwordController?.addListener(_onPasswordInputChanged);
  }

  void _detachListeners() {
    _confirmController?.removeListener(_onPasswordInputChanged);
    _passwordController?.removeListener(_onPasswordInputChanged);
    _confirmController = null;
    _passwordController = null;
  }

  void _onPasswordInputChanged() {
    if (mounted) {
      setState(() {});
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    // Canonical confirm-password matching: exact equality of TRIMMED values.
    //
    //   confirm empty (either side)              → neutral (no indicator)
    //   password empty + confirm non-empty       → NOT MATCH
    //   both non-empty + equal (trimmed)         → MATCH
    //   both non-empty + different (trimmed)     → NOT MATCH
    //
    // Stage 4D: the indicator and the submit gates (sign_up _isFormValid,
    // security validator) now agree — all compare trimmed text, so trailing
    // whitespace cannot make the indicator say "match" while the gate stays
    // disabled (or vice versa).
    final password = widget.passwordController?.text.trim() ?? '';
    final confirmPassword = widget.controller?.text.trim() ?? '';
    final hasConfirm = confirmPassword.isNotEmpty;
    final isMatch = hasConfirm && password == confirmPassword;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        TextFormField(
          controller: widget.controller,
          focusNode: widget.focusNode,
          obscureText: !widget.isVisible,
          enabled: widget.enabled,
          textInputAction: TextInputAction.done,
          decoration: InputDecoration(
            labelText: widget.labelText ?? 'Confirm Password',
            hintText: widget.hintText ?? 'Re-enter your password',
            floatingLabelBehavior: FloatingLabelBehavior.always,
            hintStyle: TextStyle(
              color: scheme.onSurfaceVariant.withValues(alpha: 0.6),
            ),
            prefixIcon: Icon(
              Icons.lock_outline,
              color: scheme.onSurfaceVariant,
            ),
            suffixIcon: IconButton(
              onPressed: widget.onToggleVisibility,
              icon: Icon(
                widget.isVisible ? Icons.visibility_off : Icons.visibility,
                color: scheme.onSurfaceVariant,
              ),
              tooltip: widget.isVisible ? 'Hide password' : 'Show password',
            ),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(
                color: scheme.outlineVariant,
              ),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(
                color: scheme.outlineVariant,
              ),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(
                color: scheme.primary,
                width: 2,
              ),
            ),
            errorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(color: scheme.error),
            ),
            focusedErrorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
              borderSide: BorderSide(color: scheme.error, width: 2),
            ),
            filled: true,
            fillColor: scheme.surfaceContainerHigh,
          ),
          validator: widget.validator,
        ),
        if (widget.showMatchIndicator && hasConfirm) ...[
          const SizedBox(height: 8),
          _buildMatchIndicator(context, isMatch),
        ],
      ],
    );
  }

  Widget _buildMatchIndicator(BuildContext context, bool isMatch) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: isMatch
              ? context.statusColors.success
              : scheme.outlineVariant,
        ),
      ),
      child: Row(
        children: [
          Icon(
            isMatch ? Icons.check_circle : Icons.cancel,
            size: AppIconSize.action,
            color: isMatch ? context.statusColors.success : scheme.error,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              isMatch ? 'Passwords match' : 'Passwords do not match',
              style: TextStyle(
                fontSize: AppType.s14,
                fontWeight: FontWeight.w500,
                color: isMatch ? context.statusColors.success : scheme.error,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
