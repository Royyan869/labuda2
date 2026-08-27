import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
        borderRadius: const BorderRadius.only(
          bottomLeft: Radius.circular(20),
          bottomRight: Radius.circular(20),
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
