import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final currentLength = _controller.text.length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              'Additional details (optional)',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),
            Text(
              '$currentLength/$_maxLength',
              style: TextStyle(
                fontSize: 12,
                color: currentLength > _maxLength * 0.9
                    ? AppColors.warning
                    : AppColors.neutralGray500,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        TextFormField(
          controller: _controller,
          enabled: widget.isEnabled,
          maxLines: 4,
          maxLength: _maxLength,
          onChanged: widget.isEnabled ? widget.onChanged : null,
          decoration: InputDecoration(
            hintText: 'Provide more context to help us understand the issue...',
            hintStyle: TextStyle(
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
            ),
            filled: true,
            fillColor: isDark
                ? AppColors.darkGray700
                : AppColors.neutralGray100,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray300,
              ),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray300,
              ),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(
                color: AppColors.primaryBlue,
                width: 2,
              ),
            ),
            disabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
              ),
            ),
            contentPadding: const EdgeInsets.all(16),
          ),
          style: TextStyle(
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'Please don\'t include personal information like phone numbers or addresses.',
          style: TextStyle(fontSize: 12, color: AppColors.neutralGray500),
        ),
      ],
    );
  }
}
