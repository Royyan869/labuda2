import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Atomic component untuk description input field
/// Single responsibility: Handle multi-line description input
/// MAKSIMAL 100 LINES - ENFORCED BY GUIDELINES
class DescriptionInputComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<String>,
        ResettableComponent {
  final String? initialValue;
  final String label;
  final String hint;
  final int maxLines;
  final int maxLength;
  final bool enableRichText;
  final void Function(String)? onChanged;
  final String? Function(String?)? validator;
  final TextEditingController? controller;

  const DescriptionInputComponent({
    super.key,
    this.initialValue,
    required this.label,
    required this.hint,
    this.maxLines = 3,
    this.maxLength = 500,
    this.enableRichText = false,
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

    if (enableRichText) {
      return _buildRichTextInput(textController);
    }

    return _buildSimpleTextInput(textController);
  }

  Widget _buildSimpleTextInput(TextEditingController controller) {
    return TextFormField(
      controller: controller,
      maxLines: maxLines,
      maxLength: maxLength,
      maxLengthEnforcement: MaxLengthEnforcement.enforced,
      textInputAction: TextInputAction.newline,
      textCapitalization: TextCapitalization.sentences,
      onChanged: onChanged,
      validator: validator ?? _defaultValidator,
      enabled: !isDisabled,
      decoration: InputDecoration(
        labelText: isRequired ? '$label *' : label,
        hintText: hint,
        border: const OutlineInputBorder(),
        alignLabelWithHint: true,
        suffixIcon: isRequired
            ? const Padding(
                padding: EdgeInsets.only(top: 8),
                child: Icon(Icons.star, size: 12, color: AppColors.error),
              )
            : null,
      ),
    );
  }

  Widget _buildRichTextInput(TextEditingController controller) {
    // Placeholder untuk rich text editor
    // Implementasi rich text bisa ditambah nanti
    return Column(
      children: [
        _buildSimpleTextInput(controller),
        const SizedBox(height: 8),
        Row(
          children: [
            IconButton(
              onPressed: isDisabled ? null : () => _addBold(controller),
              icon: const Icon(Icons.format_bold),
            ),
            IconButton(
              onPressed: isDisabled ? null : () => _addItalic(controller),
              icon: const Icon(Icons.format_italic),
            ),
            IconButton(
              onPressed: isDisabled ? null : () => _addLink(controller),
              icon: const Icon(Icons.link),
            ),
          ],
        ),
      ],
    );
  }

  void _addBold(TextEditingController controller) {
    // Simple bold formatting
    final text = controller.text;
    final selection = controller.selection;
    if (selection.isValid) {
      final selectedText = text.substring(selection.start, selection.end);
      final newText = text.replaceRange(
        selection.start,
        selection.end,
        '**$selectedText**',
      );
      controller.text = newText;
    }
  }

  void _addItalic(TextEditingController controller) {
    // Simple italic formatting
    final text = controller.text;
    final selection = controller.selection;
    if (selection.isValid) {
      final selectedText = text.substring(selection.start, selection.end);
      final newText = text.replaceRange(
        selection.start,
        selection.end,
        '*$selectedText*',
      );
      controller.text = newText;
    }
  }

  void _addLink(TextEditingController controller) {
    // Simple link formatting
    final text = controller.text;
    final selection = controller.selection;
    if (selection.isValid) {
      final selectedText = text.substring(selection.start, selection.end);
      final newText = text.replaceRange(
        selection.start,
        selection.end,
        '[$selectedText](url)',
      );
      controller.text = newText;
    }
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
      return 'Description is required';
    }
    if (value != null && value.length > maxLength) {
      return 'Description must not exceed $maxLength characters';
    }
    return null;
  }
}
