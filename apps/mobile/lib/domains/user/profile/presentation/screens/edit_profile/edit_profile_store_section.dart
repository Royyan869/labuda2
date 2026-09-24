import 'package:flutter/material.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/store_name_form_field.dart';
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
        // SINGLE AUTHORITY: canonical store/farm name field shared with the
        // seller registration wizard — same label, hint, and validation.
        StoreNameFormField(controller: storeNameController),
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
