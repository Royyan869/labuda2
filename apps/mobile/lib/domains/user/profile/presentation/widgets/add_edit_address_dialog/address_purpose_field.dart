import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Purpose selection field for address form
class AddressPurposeField extends StatelessWidget {
  final AddressPurpose? selectedPurpose;
  final AddressPurpose? forcedPurpose;
  final ValueChanged<AddressPurpose?> onPurposeChanged;
  final bool isDark;

  const AddressPurposeField({
    super.key,
    required this.selectedPurpose,
    this.forcedPurpose,
    required this.onPurposeChanged,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    if (forcedPurpose != null) {
      return _buildLockedPurposeIndicator();
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildLabel('Address Purpose'),
        const SizedBox(height: 8),
        _buildPurposeDropdown(),
      ],
    );
  }

  Widget _buildLabel(String text) {
    return Text(
      text,
      style: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
      ),
    );
  }

  Widget _buildPurposeDropdown() {
    return DropdownButtonFormField<AddressPurpose>(
      initialValue: selectedPurpose,
      decoration: _inputDecoration('Select address purpose'),
      dropdownColor: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      items: AddressPurpose.values.map((purpose) {
        return DropdownMenuItem(
          value: purpose,
          child: Text(
            purpose.label,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
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

  Widget _buildLockedPurposeIndicator() {
    final purpose = forcedPurpose!;
    final isShipping = purpose == AddressPurpose.shipping;

    return Container(
      padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Row(
        children: [
          Icon(
            isShipping
                ? Icons.local_shipping_outlined
                : Icons.storefront_outlined,
            size: 20,
            color: AppColors.primaryRed,
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
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  isShipping
                      ? 'Destination address for shipping'
                      : 'Origin address for shipping',
                  style: TextStyle(
                    fontSize: 12,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  InputDecoration _inputDecoration(String hintText) {
    return InputDecoration(
      hintText: hintText,
      filled: true,
      fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(color: AppColors.primaryRed, width: 2),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(color: AppColors.error),
      ),
    );
  }
}
