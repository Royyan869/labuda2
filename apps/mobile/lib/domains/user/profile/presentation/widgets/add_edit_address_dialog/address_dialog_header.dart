import 'package:flutter/material.dart';

/// Header for add/edit address dialog
class AddressDialogHeader extends StatelessWidget {
  final bool isEdit;
  final VoidCallback onClose;

  const AddressDialogHeader({
    super.key,
    required this.isEdit,
    required this.onClose,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: scheme.onSurfaceVariant,
        borderRadius: const BorderRadius.only(
          topLeft: Radius.circular(20),
          topRight: Radius.circular(20),
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: scheme.primary.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
            ),
child: Icon(
               Icons.location_on,
               color: scheme.primary,
              size: 24,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              isEdit ? 'Edit Address' : 'Add New Address',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          IconButton(
            onPressed: onClose,
            icon: Icon(
              Icons.close,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
