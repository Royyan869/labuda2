import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_text_field.dart';

/// Report Description Field Widget
///
/// Optional text field for users to provide additional context about their report.
/// Limited to 500 characters with a visible counter.
class ReportDescriptionField extends StatefulWidget {
  final String? initialValue;
  final Function(String) onChanged;
  final bool isEnabled;

  const ReportDescriptionField({
    super.key,
    this.initialValue,
    required this.onChanged,
    this.isEnabled = true,
  });

  @override
  State<ReportDescriptionField> createState() => _ReportDescriptionFieldState();
}

class _ReportDescriptionFieldState extends State<ReportDescriptionField> {
  late final TextEditingController _controller;
  final int _maxLength = 500;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final currentLength = _controller.text.length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              'Additional details (optional)',
              style: context.typeRoles.bodyDense.copyWith(
                fontWeight: FontWeight.w500,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            Text(
              '$currentLength/$_maxLength',
              style: context.typeRoles.labelMicro.copyWith(
                color: currentLength > _maxLength * 0.9
                    ? context.statusColors.warning
                    : Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        AppTextField(
          controller: _controller,
          enabled: widget.isEnabled,
          maxLines: 4,
          maxLength: _maxLength,
          onChanged: widget.isEnabled ? widget.onChanged : null,
          hintText:
              'Provide more context to help us understand the issue...',
        ),
        const SizedBox(height: 8),
        Text(
          'Please don\'t include personal information like phone numbers or addresses.',
          style: context.typeRoles.labelMicro.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}
