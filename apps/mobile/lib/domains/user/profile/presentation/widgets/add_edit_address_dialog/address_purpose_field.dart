import 'package:labuda/core/core.dart';
import 'package:flutter/material.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Purpose selection field for address form
class AddressPurposeField extends StatelessWidget {
  final AddressPurpose? selectedPurpose;
  final AddressPurpose? forcedPurpose;
  final ValueChanged<AddressPurpose?> onPurposeChanged;

  const AddressPurposeField({
    super.key,
    required this.selectedPurpose,
    this.forcedPurpose,
    required this.onPurposeChanged,
    
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    if (forcedPurpose != null) {
      return _buildLockedPurposeIndicator(scheme);
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildLabel('Address Purpose', scheme),
        const SizedBox(height: 8),
        _buildPurposeDropdown(context, scheme),
      ],
    );
  }

  Widget _buildLabel(String text, ColorScheme scheme) {
    return Text(
      text,
      style: TextStyle(
        fontSize: AppType.s14,
        fontWeight: FontWeight.w600,
        color: scheme.onSurfaceVariant,
      ),
    );
  }

  Widget _buildPurposeDropdown(BuildContext context, ColorScheme scheme) {
    return DropdownButtonFormField<AddressPurpose>(
      initialValue: selectedPurpose,
      decoration: _inputDecoration(context, scheme, 'Select address purpose'),
      dropdownColor: scheme.onSurfaceVariant,
      items: AddressPurpose.values.map((purpose) {
        return DropdownMenuItem(
          value: purpose,
          child: Text(
            purpose.label,
            style: TextStyle(
              color: scheme.onSurfaceVariant,
            ),
          ),
        );
      }).toList(),
      onChanged: onPurposeChanged,
      validator: (value) {
        if (value == null) {
          return 'Please select a purpose';
        }
        return null;
      },
    );
  }

  Widget _buildLockedPurposeIndicator(ColorScheme scheme) {
    final purpose = forcedPurpose!;
    final isShipping = purpose == AddressPurpose.shipping;

    return Container(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12, horizontal: AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.onSurfaceVariant,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: scheme.outlineVariant,
        ),
      ),
      child: Row(
        children: [
          Icon(
            isShipping
                ? Icons.local_shipping_outlined
                : Icons.storefront_outlined,
            size: 20,
            color: scheme.primary,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  isShipping
                      ? 'Recipient Address (Buyer)'
                      : 'Sender Address (Seller)',
                  style: TextStyle(
                    fontSize: AppType.s14,
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  isShipping
                      ? 'Destination address for shipping'
                      : 'Origin address for shipping',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  InputDecoration _inputDecoration(BuildContext context, ColorScheme scheme, String hintText) {
    return InputDecoration(
      hintText: hintText,
      filled: true,
      fillColor: scheme.onSurfaceVariant,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        borderSide: BorderSide(
          color: scheme.outlineVariant,
        ),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        borderSide: BorderSide(
          color: scheme.outlineVariant,
        ),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        borderSide: BorderSide(color: scheme.primary, width: 2),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        borderSide: BorderSide(color: context.statusColors.error),
      ),
    );
  }
}
