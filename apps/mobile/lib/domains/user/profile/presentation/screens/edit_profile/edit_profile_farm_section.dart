import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_url_validator.dart';

/// Farm Information Fields for Edit Profile (Sellers only)
class EditProfileFarmSection extends StatelessWidget {
  final TextEditingController farmNameController;
  final TextEditingController websiteController;
  final DateTime? establishedDate;
  final ValueChanged<DateTime?> onEstablishedDateChanged;

  const EditProfileFarmSection({
    super.key,
    required this.farmNameController,
    required this.websiteController,
    this.establishedDate,
    required this.onEstablishedDateChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        AppTextField(
          controller: farmNameController,
          labelText: 'Farm Name',
          hintText: 'Enter your farm name',
          prefixIcon: Icons.store_outlined,
          validator: (value) {
            if (value == null || value.trim().isEmpty) {
              return 'Farm name is required';
            }
            return null;
          },
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: websiteController,
          labelText: 'Website',
          hintText: 'https://yourfarm.com',
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
