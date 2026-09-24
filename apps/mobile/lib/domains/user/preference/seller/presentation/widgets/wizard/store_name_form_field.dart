import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';

/// SINGLE AUTHORITY (client-side) for the seller store/farm name input.
///
/// Canonical copy and validation for the seller identity name live here and
/// are shared by BOTH writers of `seller_profiles.store_name`:
/// - Seller registration wizard (`SellerWizardStep2Widget`)
/// - Edit profile (`EditProfileFarmSection` / `EditProfileStoreSection`)
///
/// Backend remains the sole write authority via
/// POST /seller/onboarding and PATCH /seller/profile; this widget only keeps
/// the input contract (label, hint, non-empty validation) consistent.
class StoreNameFormField extends StatelessWidget {
  /// Canonical field label used everywhere the seller name is edited.
  static const String canonicalLabel = 'Nama Toko/Farm';

  /// Canonical hint text.
  static const String canonicalHint = 'Example: Mutiara Koi Farm';

  /// Canonical validation message.
  static const String canonicalRequiredMessage = 'Nama toko/farm wajib diisi';

  /// Canonical non-empty validation for the seller store name. The backend
  /// (`store_name is required`) is the write authority; this mirrors it
  /// client-side so both surfaces reject the same input.
  static String? validate(String? value) {
    if (value == null || value.trim().isEmpty) {
      return canonicalRequiredMessage;
    }
    return null;
  }

  final TextEditingController controller;
  final ValueChanged<String>? onChanged;

  const StoreNameFormField({
    super.key,
    required this.controller,
    this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return AppTextField(
      controller: controller,
      labelText: '$canonicalLabel *',
      hintText: canonicalHint,
      prefixIcon: Icons.store_outlined,
      textCapitalization: TextCapitalization.sentences,
      validator: validate,
      onChanged: onChanged,
    );
  }
}
