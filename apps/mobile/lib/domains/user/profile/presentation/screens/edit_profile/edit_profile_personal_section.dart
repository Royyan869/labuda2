import 'package:flutter/material.dart';
import 'package:hishumi/shared/shared.dart';

/// Personal Information Fields for Edit Profile
class EditProfilePersonalSection extends StatelessWidget {
  final TextEditingController usernameController;
  final TextEditingController bioController;
  final ValueChanged<String>? onChanged;

  const EditProfilePersonalSection({
    super.key,
    required this.usernameController,
    required this.bioController,
    this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Username is CANONICAL user identity and IMMUTABLE after registration
        // (business truth A–H). It is displayed READ-ONLY — the user can see and
        // select it but can never edit it — and must NOT look like a disabled
        // field (owner decision 2026-10-05: disabled and read-only differ).
        // The save handler also omits it from the profile-update payload, so
        // no username mutation is ever attempted here.
        AppTextField(
          controller: usernameController,
          labelText: 'Username',
          helperText: 'Username is permanent and cannot be changed',
          prefixIcon: Icons.lock_outline,
          readOnly: true,
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: bioController,
          labelText: 'Bio',
          hintText: 'Tell us about yourself',
          prefixIcon: Icons.info_outline,
          maxLines: 3,
          onChanged: onChanged,
        ),
      ],
    );
  }
}
