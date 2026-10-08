import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/src/theme/app_theme.dart';
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
    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Basic Information',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
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
              padding: const EdgeInsets.all(core.AppMetrics.p8),
              decoration: BoxDecoration(
                color: context.statusColors.info.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(core.AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: AppIconSize.inlineGlyph,
                    color: context.statusColors.info,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Discount code cannot be changed after creation',
                      style: context.typeRoles.bodyDense.copyWith(
                        color: context.statusColors.info,
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
