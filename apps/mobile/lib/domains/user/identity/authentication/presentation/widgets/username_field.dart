import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';

/// Canonical username input — the single username field used by BOTH the
/// sign-up screen and the complete-profile screen (one widget, one language,
/// no second authority).
///
/// 🔒 CANONICAL USERNAME AUTHORITY:
/// - FORMAT validation is LOCAL via [CanonicalUsernameValidator] (mirrors the
///   backend identityusername rules) and auto-lowercases as the user types.
/// - AVAILABILITY (taken / reserved / final acceptance) is BACKEND authority,
///   decided only at the transactional moment (exchange / complete-profile).
///   There is NO client-side availability pre-check — this widget NEVER
///   claims "available"; backend rejections surface inline via the owning
///   screen's error slot.
class UsernameField extends StatefulWidget {
  final TextEditingController controller;

  /// Local format-only result: (isValidFormat, isFilled). Availability is
  /// never claimed here — the second parameter is always false and exists
  /// only to keep existing call sites compiling.
  final void Function(bool isValidFormat, bool isAvailable) onValidationChanged;

  const UsernameField({
    super.key,
    required this.controller,
    required this.onValidationChanged,
  });

  @override
  State<UsernameField> createState() => _UsernameFieldState();
}

class _UsernameFieldState extends State<UsernameField> {
  /// Local format-only feedback: null = empty (neutral), false = invalid
  /// format (red), true = valid format (neutral — "available" is never
  /// claimed locally).
  bool? _formatValid;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_onUsernameChanged);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onUsernameChanged);
    super.dispose();
  }

  void _onUsernameChanged() {
    final raw = widget.controller.text;
    // Auto-lowercase as the user types: canonical usernames are lowercase
    // only (backend identityusername rules), so the field must never show an
    // "uppercase rejected" warning for input the user could not have known
    // to avoid. toLowerCase preserves length for the allowed charset, so the
    // selection offset stays valid. Re-fires the listener once (idempotent).
    final lowered = raw.toLowerCase();
    if (lowered != raw) {
      final offset = widget.controller.selection.baseOffset;
      final safeOffset =
          offset < 0 ? lowered.length : offset.clamp(0, lowered.length);
      widget.controller.value = TextEditingValue(
        text: lowered,
        selection: TextSelection.collapsed(offset: safeOffset),
      );
      return;
    }

    if (raw.isEmpty) {
      if (mounted && _formatValid != null) {
        setState(() => _formatValid = null);
      }
      widget.onValidationChanged(false, false);
      return;
    }

    final canonical = CanonicalUsernameValidator.normalize(raw);
    final valid = canonical != null && CanonicalUsernameValidator.isValid(canonical);

    if (mounted && valid != _formatValid) {
      setState(() => _formatValid = valid);
    }
    widget.onValidationChanged(valid, false);
  }

  Color _getBorderColor(BuildContext context) {
    if (_formatValid == false) return AppColors.error;
    return Theme.of(context).colorScheme.outlineVariant;
  }

  Widget? get _getSuffixIcon {
    if (_formatValid == true) {
      return const Icon(Icons.check_circle, color: AppColors.success, size: 20);
    }
    if (_formatValid == false) {
      return const Icon(Icons.error, color: AppColors.error, size: 20);
    }
    return null;
  }

  String? get _getHelperText {
    if (_formatValid == false) {
      return 'Username must be 3-30 chars: lowercase letters, numbers, '
          'and underscores only';
    }
    return 'Unique username for your profile';
  }

  Color _getHelperTextColor(BuildContext context) {
    if (_formatValid == false) return AppColors.error;
    return Theme.of(context).colorScheme.onSurfaceVariant;
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        TextFormField(
          controller: widget.controller,
          decoration: InputDecoration(
            labelText: 'Username',
            hintText: 'Choose a unique username',
            prefixIcon: Icon(
              Icons.alternate_email,
              color: scheme.onSurfaceVariant,
            ),
            suffixIcon: _getSuffixIcon,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: _getBorderColor(context)),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: _getBorderColor(context)),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: _getBorderColor(context), width: 2),
            ),
            errorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: scheme.error),
            ),
            focusedErrorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: scheme.error, width: 2),
            ),
            filled: true,
            fillColor: scheme.surfaceContainerHigh,
          ),
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Username cannot be empty';
            }
            final canonical = CanonicalUsernameValidator.normalize(value);
            if (canonical == null ||
                !CanonicalUsernameValidator.isValid(canonical)) {
              return 'Username must be 3-30 chars: lowercase letters, '
                  'numbers, and underscores only';
            }
            return null;
          },
        ),
        SizedBox(
          height: _getHelperText != null ? 20 : 0,
          child: _getHelperText != null
              ? Padding(
                  padding: const EdgeInsets.only(left: 12, top: 4),
                  child: Text(
                    _getHelperText!,
                    style: TextStyle(
                      fontSize: 12,
                      color: _getHelperTextColor(context),
                      fontWeight: FontWeight.normal,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                )
              : null,
        ),
      ],
    );
  }
}
