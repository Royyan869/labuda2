import 'package:flutter/services.dart';
import 'package:intl/intl.dart';

/// Currency input formatter for Indonesian Rupiah
/// Formats number with thousands separator using dots (1.000.000)
class ThousandsSeparatorInputFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    if (newValue.text.isEmpty) {
      return newValue;
    }

    // Remove any non-digit characters
    final numericValue = newValue.text.replaceAll(RegExp(r'[^\d]'), '');

    if (numericValue.isEmpty) {
      return const TextEditingValue(text: '');
    }

    final number = int.tryParse(numericValue);
    if (number == null) {
      return oldValue;
    }

    // Format with thousand separators (Indonesian format with dots)
    final formattedText = NumberFormat(
      '#,###',
      'id_ID',
    ).format(number).replaceAll(',', '.');

    return TextEditingValue(
      text: formattedText,
      selection: TextSelection.collapsed(offset: formattedText.length),
    );
  }
}
