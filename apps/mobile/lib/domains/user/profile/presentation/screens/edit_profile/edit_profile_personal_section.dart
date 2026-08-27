import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';

/// Personal Information Fields for Edit Profile
class EditProfilePersonalSection extends StatelessWidget {
  final TextEditingController usernameController;
  final TextEditingController bioController;

  const EditProfilePersonalSection({
    super.key,
    required this.usernameController,
    required this.bioController,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Username is CANONICAL user identity and IMMUTABLE after registration
        // (business truth A–H). It is displayed read-only so the user can see
        // it but can never submit a change through the normal Edit Profile UI.
        // The save handler also omits it from the profile-update payload, so
        // no username mutation is ever attempted here.
        AbsorbPointer(
          absorbing: true,
          child: AppTextField(
            controller: usernameController,
            labelText: 'Username',
            prefixIcon: Icons.lock_outline,
            enabled: false,
          ),
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: bioController,
          labelText: 'Bio',
          hintText: 'Tell us about yourself',
          prefixIcon: Icons.info_outline,
          maxLines: 3,
        ),
      ],
    );
  }
}
