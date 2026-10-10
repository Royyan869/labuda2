import 'package:flutter/material.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/widgets/wizard/store_name_form_field.dart';

/// Farm Information Fields for Edit Profile (Sellers only).
///
/// The canonical seller surface is exactly: store name + store image (the
/// image is edited in [EditProfileAvatarSection]). Website and established
/// date are not part of the canonical seller profile.
class EditProfileFarmSection extends StatelessWidget {
  final TextEditingController farmNameController;

  const EditProfileFarmSection({super.key, required this.farmNameController});

  @override
  Widget build(BuildContext context) {
    // SINGLE AUTHORITY: canonical store/farm name field shared with the
    // seller registration wizard — same label, hint, and validation.
    return StoreNameFormField(controller: farmNameController);
  }
}
