import 'package:flutter/material.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
      padding: const EdgeInsets.only(left: AppMetrics.p16, right: AppMetrics.p16, top: AppMetrics.p12, bottom: AppMetrics.p12),
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
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppShape.r12),
            ),
          ),
        ),
      ),
    );
  }
}
