import 'package:flutter/material.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Sticky add address button at the bottom of the screen
class AddressStickyButton extends StatelessWidget {
  final AddressPurpose purpose;
  final int addressCount;
  final VoidCallback onPressed;

  const AddressStickyButton({
    super.key,
    required this.purpose,
    required this.addressCount,
    
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    // Max 10 addresses per purpose - hide button if limit reached
    if (addressCount >= 10) {
      return const SizedBox.shrink();
    }

    return Container(
      padding: const EdgeInsets.only(left: 16, right: 16, top: 12, bottom: 12),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          top: BorderSide(
            color: scheme.outlineVariant,
          ),
        ),
      ),
      child: SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: onPressed,
          icon: const Icon(Icons.add_location_alt),
          label: Text('Add ${purpose.label}'),
          style: ElevatedButton.styleFrom(
            backgroundColor: scheme.primary,
            foregroundColor: scheme.onPrimary,
            padding: const EdgeInsets.symmetric(vertical: 14),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
        ),
      ),
    );
  }
}
