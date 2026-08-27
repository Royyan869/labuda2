import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';

class UsernameField extends ConsumerStatefulWidget {
  final TextEditingController controller;
  final bool isDark;
  final Function(bool isValid, bool isAvailable) onValidationChanged;

  const UsernameField({
    super.key,
    required this.controller,
    required this.isDark,
    required this.onValidationChanged,
  });

  @override
  ConsumerState<UsernameField> createState() => _UsernameFieldState();
}

class _UsernameFieldState extends ConsumerState<UsernameField> {
  UsernameCheckResult? _checkResult;
  bool _isChecking = false;

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

  /// Registration-time realtime feedback.
  ///
  /// Local validation establishes exactly three states: empty, invalid
  /// format, or valid format (via the canonical backend authority
  /// [CanonicalUsernameValidator]). It NEVER claims "available" — reserved
  /// names, taken names, and final acceptance are backend authority and are
  /// surfaced only when the authenticated exchange responds.
  void _onUsernameChanged() {
    final canonical = CanonicalUsernameValidator.normalize(widget.controller.text);

    if (canonical == null) {
      setState(() {
        _checkResult = null;
        _isChecking = false;
      });
      widget.onValidationChanged(false, false);
      return;
    }

    if (!CanonicalUsernameValidator.isValid(canonical)) {
      if (mounted) {
        setState(() {
          _checkResult = UsernameCheckResult.invalid(
            'Username must be 3-30 chars: lowercase letters, numbers, '
            'and underscores only',
          );
          _isChecking = false;
        });
        widget.onValidationChanged(false, false);
      }
      return;
    }

    if (mounted) {
      setState(() {
        _checkResult = UsernameCheckResult.validFormat();
        _isChecking = false;
      });
      // Valid format only — availability is NOT claimed locally.
      widget.onValidationChanged(true, false);
    }
  }

  Color get _getBorderColor {
    if (_checkResult == null) {
      return widget.isDark ? AppColors.darkGray600 : AppColors.neutralGray300;
    }

    switch (_checkResult!.status) {
      case UsernameCheckStatus.available:
        return AppColors.success;
      case UsernameCheckStatus.unavailable:
      case UsernameCheckStatus.invalid:
      case UsernameCheckStatus.error:
        return AppColors.error;
      case UsernameCheckStatus.checking:
      case UsernameCheckStatus.validFormat:
      case UsernameCheckStatus.idle:
        return widget.isDark ? AppColors.darkGray600 : AppColors.neutralGray300;
    }
  }

  Widget? get _getSuffixIcon {
    if (_isChecking) {
      return const Padding(
        padding: EdgeInsets.all(12),
        child: SizedBox(
          width: 16,
          height: 16,
          child: CircularProgressIndicator(
            strokeWidth: 2,
            valueColor: AlwaysStoppedAnimation<Color>(AppColors.primaryRed),
          ),
        ),
      );
    }

    if (_checkResult == null) return null;

    switch (_checkResult!.status) {
      case UsernameCheckStatus.available:
        return const Icon(
          Icons.check_circle,
          color: AppColors.success,
          size: 20,
        );
      case UsernameCheckStatus.unavailable:
      case UsernameCheckStatus.invalid:
      case UsernameCheckStatus.error:
        return const Icon(Icons.error, color: AppColors.error, size: 20);
      case UsernameCheckStatus.checking:
      case UsernameCheckStatus.validFormat:
      case UsernameCheckStatus.idle:
        return null;
    }
  }

  String? get _getHelperText {
    if (_checkResult == null) return 'Unique username for your profile';
    if (_checkResult!.message?.isNotEmpty == true) {
      return _checkResult!.message;
    }
    return null;
  }

  Color get _getHelperTextColor {
    if (_checkResult == null) {
      return widget.isDark
          ? AppColors.neutralGray500
          : AppColors.neutralGray400;
    }

    switch (_checkResult!.status) {
      case UsernameCheckStatus.available:
        return AppColors.success;
      case UsernameCheckStatus.unavailable:
      case UsernameCheckStatus.invalid:
      case UsernameCheckStatus.error:
        return AppColors.error;
      case UsernameCheckStatus.checking:
        return AppColors.primaryRed;
      case UsernameCheckStatus.validFormat:
      case UsernameCheckStatus.idle:
        return widget.isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
    }
  }

  @override
  Widget build(BuildContext context) {
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
              color: widget.isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            suffixIcon: _getSuffixIcon,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: _getBorderColor),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: _getBorderColor),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(color: _getBorderColor, width: 2),
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
            fillColor: widget.isDark
                ? AppColors.darkGray700
                : AppColors.neutralGray50,
          ),
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Username cannot be empty';
            }
            if (_checkResult != null && !_checkResult!.isValid) {
              return _checkResult!.message;
            }
            if (_isChecking) {
              return 'Still checking username availability';
            }
            if (_checkResult != null &&
                _checkResult!.status == UsernameCheckStatus.error) {
              return null;
            }
            if (_checkResult != null &&
                !_checkResult!.isAvailable &&
                _checkResult!.status != UsernameCheckStatus.validFormat) {
              return _checkResult!.message;
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
                      color: _getHelperTextColor,
                      fontWeight: _checkResult?.isAvailable == true
                          ? FontWeight.w500
                          : FontWeight.normal,
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
