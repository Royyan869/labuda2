import 'package:flutter/material.dart';
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
            child: OutlinedButton(
              onPressed: onCancel,
              child: const Text('Cancel'),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: ElevatedButton(
              onPressed: isLoading ? null : onSubmit,
              child: isLoading
                  ? SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        valueColor: AlwaysStoppedAnimation(
                          Theme.of(context).colorScheme.onPrimary,
                        ),
                      ),
                    )
                  : Text(isEdit ? 'Update' : 'Add Address'),
            ),
          ),
        ],
      ),
    );
  }
}
