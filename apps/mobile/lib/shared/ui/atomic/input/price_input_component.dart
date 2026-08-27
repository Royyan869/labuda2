import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Atomic component untuk price input dengan currency formatting
/// Single responsibility: Handle price input dengan validation dan format
/// MAKSIMAL 100 LINES - ENFORCED BY GUIDELINES
class PriceInputComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<double>,
        ResettableComponent {
  final double? initialValue;
  final String label;
  final String hint;
  final double? minPrice;
  final double? maxPrice;
  final String currency;
  final void Function(double?)? onChanged;
  final String? Function(double?)? validator;
  final TextEditingController? controller;

  const PriceInputComponent({
    super.key,
    this.initialValue,
    required this.label,
    required this.hint,
    this.minPrice,
    this.maxPrice,
    this.currency = 'IDR',
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
        controller ??
        TextEditingController(
          text: initialValue != null ? _formatPrice(initialValue!) : null,
        );

    return TextFormField(
      controller: textController,
      keyboardType: TextInputType.number,
      textInputAction: TextInputAction.next,
      inputFormatters: [
        FilteringTextInputFormatter.digitsOnly,
        _PriceInputFormatter(),
      ],
      onChanged: (value) {
        final price = _parsePrice(value);
        onChanged?.call(price);
      },
      validator: (value) {
        final price = _parsePrice(value);
        return validator?.call(price) ?? _defaultValidator(price);
      },
      enabled: !isDisabled,
      decoration: InputDecoration(
        labelText: isRequired ? '$label *' : label,
        hintText: hint,
        border: const OutlineInputBorder(),
        prefixText: '$currency ',
        suffixIcon: isRequired
            ? const Icon(Icons.star, size: 12, color: AppColors.error)
            : null,
      ),
    );
  }

  String _formatPrice(double price) {
    return price
        .toStringAsFixed(0)
        .replaceAllMapped(
          RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
          (Match match) => '${match[1]},',
        );
  }

  double? _parsePrice(String? value) {
    if (value == null || value.isEmpty) return null;
    final cleanValue = value.replaceAll(',', '');
    return double.tryParse(cleanValue);
  }

  @override
  String? validate() {
    return validator?.call(getData()) ?? _defaultValidator(getData());
  }

  @override
  double? getData() {
    return _parsePrice(controller?.text);
  }

  @override
  void reset() {
    controller?.clear();
  }

  String? _defaultValidator(double? value) {
    if (isRequired && value == null) {
      return 'Price is required';
    }
    if (value != null && minPrice != null && value < minPrice!) {
      return 'Price must be at least ${_formatPrice(minPrice!)}';
    }
    if (value != null && maxPrice != null && value > maxPrice!) {
      return 'Price must not exceed ${_formatPrice(maxPrice!)}';
    }
    return null;
  }
}

/// Custom formatter untuk price input
class _PriceInputFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    if (newValue.text.isEmpty) {
      return newValue;
    }

    final cleanText = newValue.text.replaceAll(',', '');
    final formattedText = cleanText.replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match match) => '${match[1]},',
    );

    return TextEditingValue(
      text: formattedText,
      selection: TextSelection.collapsed(offset: formattedText.length),
    );
  }
}
