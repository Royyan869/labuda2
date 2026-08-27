import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Atomic component untuk title input field
/// Single responsibility: Handle title input dengan validation
/// MAKSIMAL 100 LINES - ENFORCED BY GUIDELINES
class TitleInputComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<String>,
        ResettableComponent {
  final String? initialValue;
  final String label;
  final String hint;
  final int maxLength;
  final void Function(String)? onChanged;
  final String? Function(String?)? validator;
  final TextEditingController? controller;

  const TitleInputComponent({
    super.key,
    this.initialValue,
    required this.label,
    required this.hint,
    this.maxLength = 100,
    this.onChanged,
    this.validator,
    this.controller,
    super.componentId,
    super.isRequired,
    super.errorMessage,
    super.isLoading,
    super.isDisabled,
  });

  @override
  Widget buildContent(BuildContext context) {
    final textController =
        controller ?? TextEditingController(text: initialValue);

    return TextFormField(
      controller: textController,
      maxLength: maxLength,
      maxLengthEnforcement: MaxLengthEnforcement.enforced,
      textInputAction: TextInputAction.next,
      textCapitalization: TextCapitalization.sentences,
      onChanged: onChanged,
      validator: validator ?? _defaultValidator,
      enabled: !isDisabled,
      decoration: InputDecoration(
        labelText: isRequired ? '$label *' : label,
        hintText: hint,
        border: const OutlineInputBorder(),
        counterText: '', // Hide character counter
        suffixIcon: isRequired
            ? const Icon(Icons.star, size: 12, color: AppColors.error)
            : null,
      ),
    );
  }

  @override
  String? validate() {
    return validator?.call(getData()) ?? _defaultValidator(getData());
  }

  @override
  String? getData() {
    return controller?.text ?? initialValue;
  }

  @override
  void reset() {
    controller?.clear();
  }

  String? _defaultValidator(String? value) {
    if (isRequired && (value?.trim().isEmpty ?? true)) {
      return 'Title is required';
    }
    if (value != null && value.length < 3) {
      return 'Title must be at least 3 characters';
    }
    if (value != null && value.length > maxLength) {
      return 'Title must not exceed $maxLength characters';
    }
    return null;
  }
}
