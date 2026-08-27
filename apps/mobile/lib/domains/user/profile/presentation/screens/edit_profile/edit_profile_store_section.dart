import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_url_validator.dart';

/// Store Information Fields for Edit Profile (sellers only)
class EditProfileStoreSection extends StatelessWidget {
  final TextEditingController storeNameController;
  final TextEditingController websiteController;
  final DateTime? establishedDate;
  final ValueChanged<DateTime?> onEstablishedDateChanged;

  const EditProfileStoreSection({
    super.key,
    required this.storeNameController,
    required this.websiteController,
    this.establishedDate,
    required this.onEstablishedDateChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        AppTextField(
          controller: storeNameController,
          labelText: 'Store Name',
          hintText: 'Enter your store name',
          prefixIcon: Icons.store_outlined,
          validator: (value) {
            if (value == null || value.trim().isEmpty) {
              return 'Store name is required';
            }
            return null;
          },
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: websiteController,
          labelText: 'Website',
          hintText: 'https://yourstore.com',
          prefixIcon: Icons.language,
          keyboardType: TextInputType.url,
          validator: (value) =>
              CanonicalUrlValidator.validationMessage(value),
        ),
        const SizedBox(height: 16),
        AppDatePicker.dateOnly(
          labelText: 'Established Date',
          selectedDate: establishedDate,
          onChanged: onEstablishedDateChanged,
        ),
      ],
    );
  }
}
