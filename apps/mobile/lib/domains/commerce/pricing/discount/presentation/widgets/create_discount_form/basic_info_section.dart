import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_text_field.dart';

/// Section untuk basic info discount (kode & deskripsi)
class BasicInfoSection extends StatefulWidget {
  final String code;
  final String description;
  final bool isEditMode;
  final ValueChanged<String> onCodeChanged;
  final ValueChanged<String> onDescriptionChanged;

  const BasicInfoSection({
    super.key,
    required this.code,
    required this.description,
    required this.isEditMode,
    required this.onCodeChanged,
    required this.onDescriptionChanged,
  });

  @override
  State<BasicInfoSection> createState() => _BasicInfoSectionState();
}

class _BasicInfoSectionState extends State<BasicInfoSection> {
  late TextEditingController _codeController;
  late TextEditingController _descriptionController;

  @override
  void initState() {
    super.initState();
    _codeController = TextEditingController(text: widget.code);
    _descriptionController = TextEditingController(text: widget.description);
  }

  @override
  void dispose() {
    _codeController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Basic Information',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),

          // Kode Diskon
          AppTextField(
            controller: _codeController,
            labelText: 'Discount Code *',
            hintText: 'Example: KOHAKU50',
            prefixIcon: Icons.discount,
            textCapitalization: TextCapitalization.characters,
            maxLength: 20,
            enabled: !widget.isEditMode, // Code cannot be changed during edit
            onChanged: (value) {
              widget.onCodeChanged(value.toUpperCase());
            },
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return 'Discount code is required';
              }
              if (value.trim().length < 3) {
                return 'Minimum 3 characters';
              }
              return null;
            },
          ),
          const SizedBox(height: 16),

          // Deskripsi
          AppTextField(
            controller: _descriptionController,
            labelText: 'Description *',
            hintText: 'Example: 50% discount for all Kohaku',
            prefixIcon: Icons.description,
            maxLines: 3,
            maxLength: 200,
            onChanged: widget.onDescriptionChanged,
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return 'Description is required';
              }
              return null;
            },
          ),

          if (widget.isEditMode) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: core.AppColors.statusInfo.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: core.AppColors.statusInfo,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Discount code cannot be changed after creation',
                      style: TextStyle(
                        fontSize: 12,
                        color: core.AppColors.statusInfo,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
