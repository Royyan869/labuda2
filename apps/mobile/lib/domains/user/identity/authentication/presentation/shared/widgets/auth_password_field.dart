import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/helpers/canonical_password_match.dart';

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
          // Border/fill/geometry AND hint/label/icon state come from
          // `inputDecorationTheme` (AppTheme) — the one form-field authority.
          // This wrapper only owns auth content (label/hint/lock + visibility
          // toggle).
          decoration: InputDecoration(
            labelText: widget.labelText ?? 'Password',
            hintText: widget.hintText ?? 'Enter your password',
            floatingLabelBehavior: FloatingLabelBehavior.always,
            prefixIcon: const Icon(Icons.lock_outline),
            suffixIcon: IconButton(
              onPressed: widget.onToggleVisibility,
              icon: Icon(
                widget.isPasswordVisible
                    ? Icons.visibility_off
                    : Icons.visibility,
                semanticLabel: widget.isPasswordVisible
                    ? 'Sembunyikan kata sandi'
                    : 'Tampilkan kata sandi',
              ),
            ),
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
    // Canonical confirm-password matching (one authority): exact equality of
    // TRIMMED values. Empty confirmation (either side) is the neutral state
    // and renders no indicator. The submit gates use the same rule, so the
    // indicator and the gate can never disagree.
    final password = widget.passwordController?.text ?? '';
    final confirmPassword = widget.controller?.text ?? '';
    final hasConfirm = confirmPassword.trim().isNotEmpty;
    final isMatch = CanonicalPasswordMatch.matches(confirmPassword, password);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        TextFormField(
          controller: widget.controller,
          focusNode: widget.focusNode,
          obscureText: !widget.isVisible,
          enabled: widget.enabled,
          textInputAction: TextInputAction.done,
          // Border/fill/geometry AND hint/label/icon state come from
          // `inputDecorationTheme` (AppTheme) — the one form-field authority.
          decoration: InputDecoration(
            labelText: widget.labelText ?? 'Confirm Password',
            hintText: widget.hintText ?? 'Re-enter your password',
            floatingLabelBehavior: FloatingLabelBehavior.always,
            prefixIcon: const Icon(Icons.lock_outline),
            suffixIcon: IconButton(
              onPressed: widget.onToggleVisibility,
              icon: Icon(
                widget.isVisible ? Icons.visibility_off : Icons.visibility,
                semanticLabel: widget.isVisible
                    ? 'Sembunyikan kata sandi'
                    : 'Tampilkan kata sandi',
              ),
            ),
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
          color: isMatch ? context.statusColors.success : scheme.outlineVariant,
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
              style: context.typeRoles.bodyDense.copyWith(
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
