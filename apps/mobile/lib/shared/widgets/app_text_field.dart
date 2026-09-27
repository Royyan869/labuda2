import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Reusable Text Field dengan styling konsisten sesuai LABUDA design
///
/// Features:
/// - Adaptive theming (dark/light)
/// - Consistent styling dengan auth fields
/// - Built-in validation
/// - Password field support dengan show/hide
/// - Prefix/suffix icon support
/// - Prefix/suffix text support (e.g., "Rp " for currency)
class AppTextField extends StatefulWidget {
  final TextEditingController? controller;
  final String? labelText;
  final String? hintText;
  final String? Function(String?)? validator;
  final TextInputType keyboardType;
  final bool obscureText;
  final bool isPassword;
  final IconData? prefixIcon;
  final Widget? suffixIcon;
  final String? prefixText;
  final String? suffixText;
  final FocusNode? focusNode;
  final void Function(String)? onChanged;
  final int? maxLines;
  final int? maxLength;
  final bool enabled;
  final TextCapitalization textCapitalization;
  final List<TextInputFormatter>? inputFormatters;

  const AppTextField({
    super.key,
    this.controller,
    this.labelText,
    this.hintText,
    this.validator,
    this.keyboardType = TextInputType.text,
    this.obscureText = false,
    this.isPassword = false,
    this.prefixIcon,
    this.suffixIcon,
    this.prefixText,
    this.suffixText,
    this.focusNode,
    this.onChanged,
    this.maxLines = 1,
    this.maxLength,
    this.enabled = true,
    this.textCapitalization = TextCapitalization.none,
    this.inputFormatters,
  });

  /// Factory untuk email field
  const AppTextField.email({
    super.key,
    this.controller,
    this.labelText = 'Email',
    this.hintText = 'nama@email.com',
    this.validator,
    this.focusNode,
    this.onChanged,
    this.enabled = true,
  }) : keyboardType = TextInputType.emailAddress,
       obscureText = false,
       isPassword = false,
       prefixIcon = Icons.email_outlined,
       suffixIcon = null,
       prefixText = null,
       suffixText = null,
       maxLines = 1,
       maxLength = null,
       textCapitalization = TextCapitalization.none,
       inputFormatters = null;

  /// Factory untuk password field
  const AppTextField.password({
    super.key,
    this.controller,
    this.labelText = 'Password',
    this.hintText = 'Masukkan password',
    this.validator,
    this.focusNode,
    this.onChanged,
    this.enabled = true,
  }) : keyboardType = TextInputType.text,
       obscureText = false, // Will be handled internally
       isPassword = true,
       prefixIcon = Icons.lock_outline,
       suffixIcon = null, // Will be handled internally
       prefixText = null,
       suffixText = null,
       maxLines = 1,
       maxLength = null,
       textCapitalization = TextCapitalization.none,
       inputFormatters = null;

  @override
  State<AppTextField> createState() => _AppTextFieldState();
}

class _AppTextFieldState extends State<AppTextField> {
  late bool _obscureText;

  @override
  void initState() {
    super.initState();
    _obscureText = widget.isPassword;
  }

  /// Build label widget with red asterisk if needed
  Widget? _buildLabel(BuildContext context) {
    if (widget.labelText == null) return null;

    final labelText = widget.labelText!;

    // Check if label ends with " *"
    if (labelText.endsWith(' *')) {
      final textWithoutAsterisk = labelText.substring(0, labelText.length - 2);
      final scheme = Theme.of(context).colorScheme;
      return RichText(
        text: TextSpan(
          text: textWithoutAsterisk,
          style: TextStyle(color: scheme.onSurface, fontSize: 16),
          children: [
            TextSpan(
              text: ' *',
              style: TextStyle(color: scheme.error),
            ),
          ],
        ),
      );
    }

    return null; // Use labelText parameter instead
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final customLabel = _buildLabel(context);

    return TextFormField(
      controller: widget.controller,
      focusNode: widget.focusNode,
      keyboardType: widget.keyboardType,
      obscureText: _obscureText,
      maxLines: widget.maxLines,
      maxLength: widget.maxLength,
      enabled: widget.enabled,
      onChanged: widget.onChanged,
      textCapitalization: widget.textCapitalization,
      inputFormatters: widget.inputFormatters,
      decoration: InputDecoration(
        label: customLabel,
        labelText: customLabel == null ? widget.labelText : null,
        hintText: widget.hintText,
        floatingLabelBehavior: FloatingLabelBehavior.always,
        hintStyle: TextStyle(color: scheme.onSurfaceVariant),
        alignLabelWithHint: widget.maxLines != null && widget.maxLines! > 1,
        prefixIcon: widget.maxLines != null && widget.maxLines! > 1
            ? null
            : (widget.prefixIcon != null
                  ? Icon(
                      widget.prefixIcon,
                      color: scheme.onSurfaceVariant,
                    )
                  : null),
        prefixText: widget.prefixText,
        suffixText: widget.suffixText,
        suffixIcon: widget.isPassword
            ? IconButton(
                onPressed: () {
                  setState(() {
                    _obscureText = !_obscureText;
                  });
                },
                icon: Icon(
                  _obscureText ? Icons.visibility : Icons.visibility_off,
                  color: scheme.onSurfaceVariant,
                ),
              )
            : widget.suffixIcon,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: scheme.outlineVariant),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: scheme.outlineVariant),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: scheme.primary, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: scheme.error),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: scheme.error, width: 2),
        ),
      ),
      validator: widget.validator,
    );
  }
}
