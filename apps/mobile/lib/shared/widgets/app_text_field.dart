import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// THE generic text-field producer — the ONE wrapper for ordinary data-entry
/// text fields (owner decision 2026-10-05, Input/Form convergence).
///
/// It owns only the CONTENT of a field (label/hint/prefix/suffix/error slot)
/// and forwards interaction state; the border/fill/geometry/state contract
/// lives in `inputDecorationTheme` (AppTheme) — one form-field authority. It
/// does NOT own password behaviour: password fields are the auth domain's
/// `AuthPasswordField` / `AuthConfirmPasswordField`.
class AppTextField extends StatelessWidget {
  final TextEditingController? controller;
  final String? labelText;
  final String? hintText;
  final String? Function(String?)? validator;
  final TextInputType keyboardType;
  final IconData? prefixIcon;
  final Widget? suffixIcon;
  final String? prefixText;
  final String? suffixText;
  final FocusNode? focusNode;
  final void Function(String)? onChanged;
  final int? maxLines;
  final int? maxLength;
  final bool enabled;

  /// Shows the current value but blocks editing WITHOUT the disabled look.
  /// Business read-only (e.g. an immutable identity) uses this, not `enabled`.
  final bool readOnly;

  final TextCapitalization textCapitalization;
  final List<TextInputFormatter>? inputFormatters;
  final TextInputAction? textInputAction;

  /// Initial text for a controller-less field. Live requirement: the
  /// create-auction/create-for-sale numeric fields seed values this way.
  /// Never pass together with [controller] (Flutter asserts on that pair).
  final String? initialValue;

  /// Server/operation error text shown through the canonical decoration.
  /// Live requirement: `withdraw_dialog` surfaces its withdrawal failure on
  /// the amount field; the auth screens surface backend rejections here.
  final String? errorText;

  /// Focus the field when it is built. Live requirement: the negotiation
  /// offer sheet opens with its money field focused so the buyer can type
  /// the offer immediately.
  final bool autofocus;

  /// Helper line rendered under the field. Live requirement: the
  /// for-sale stock field explains the unique-vs-stocked distinction.
  final String? helperText;

  const AppTextField({
    super.key,
    this.controller,
    this.labelText,
    this.hintText,
    this.validator,
    this.keyboardType = TextInputType.text,
    this.prefixIcon,
    this.suffixIcon,
    this.prefixText,
    this.suffixText,
    this.focusNode,
    this.onChanged,
    this.maxLines = 1,
    this.maxLength,
    this.enabled = true,
    this.readOnly = false,
    this.textCapitalization = TextCapitalization.none,
    this.inputFormatters,
    this.textInputAction,
    this.initialValue,
    this.errorText,
    this.autofocus = false,
    this.helperText,
  });

  /// Email field with common defaults.
  const AppTextField.email({
    super.key,
    this.controller,
    this.labelText = 'Email',
    this.hintText = 'nama@email.com',
    this.validator,
    this.focusNode,
    this.onChanged,
    this.enabled = true,
    this.readOnly = false,
    this.errorText,
    this.textInputAction,
  }) : keyboardType = TextInputType.emailAddress,
       prefixIcon = Icons.email_outlined,
       suffixIcon = null,
       prefixText = null,
       suffixText = null,
       maxLines = 1,
       maxLength = null,
       textCapitalization = TextCapitalization.none,
       inputFormatters = null,
       initialValue = null,
       autofocus = false,
       helperText = null;

  /// Build label widget with red asterisk if needed.
  Widget? _buildLabel(BuildContext context) {
    if (labelText == null) return null;
    final text = labelText!;
    if (!text.endsWith(' *')) return null;

    final scheme = Theme.of(context).colorScheme;
    return RichText(
      text: TextSpan(
        text: text.substring(0, text.length - 2),
        style: Theme.of(
          context,
        ).textTheme.bodyLarge?.copyWith(color: scheme.onSurface),
        children: [
          TextSpan(text: ' *', style: TextStyle(color: scheme.error)),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final customLabel = _buildLabel(context);

    // The border/fill/geometry/state contract lives in
    // `inputDecorationTheme` (AppTheme) — one form-field authority. This
    // widget only states the content it owns (label/hint/icons/error slot);
    // restating borders, fill, hint or icon colours here would fork it.
    return TextFormField(
      controller: controller,
      focusNode: focusNode,
      autofocus: autofocus,
      initialValue: initialValue,
      keyboardType: keyboardType,
      maxLines: maxLines,
      maxLength: maxLength,
      enabled: enabled,
      readOnly: readOnly,
      onChanged: onChanged,
      textCapitalization: textCapitalization,
      inputFormatters: inputFormatters,
      textInputAction: textInputAction,
      decoration: InputDecoration(
        label: customLabel,
        labelText: customLabel == null ? labelText : null,
        hintText: hintText,
        errorText: errorText,
        helperText: helperText,
        floatingLabelBehavior: FloatingLabelBehavior.always,
        alignLabelWithHint: maxLines != null && maxLines! > 1,
        prefixIcon: maxLines != null && maxLines! > 1
            ? null
            : (prefixIcon != null ? Icon(prefixIcon) : null),
        prefixText: prefixText,
        suffixText: suffixText,
        suffixIcon: suffixIcon,
      ),
      validator: validator,
    );
  }
}
