import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Action buttons for add/edit address dialog
class AddressDialogActions extends StatelessWidget {
  final bool isEdit;
  final bool isLoading;
  final VoidCallback onCancel;
  final VoidCallback onSubmit;

  const AddressDialogActions({
    super.key,
    required this.isEdit,
    required this.isLoading,
    required this.onCancel,
    required this.onSubmit,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        color: scheme.onSurfaceVariant,
        borderRadius: const BorderRadius.only(
          bottomLeft: Radius.circular(AppShape.r20),
          bottomRight: Radius.circular(AppShape.r20),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: AppButton.secondary(text: 'Cancel', onPressed: onCancel),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: AppButton.primary(
              text: isEdit ? 'Update' : 'Add Address',
              onPressed: isLoading ? null : onSubmit,
              isLoading: isLoading,
            ),
          ),
        ],
      ),
    );
  }
}
