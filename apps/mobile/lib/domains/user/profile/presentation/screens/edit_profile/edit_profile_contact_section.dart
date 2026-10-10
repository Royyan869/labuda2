import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';

/// Contact & Social Media Fields for Edit Profile.
///
/// Social media has NO visibility toggle: presence of a value is the
/// visibility authority (empty = not displayed, filled = publicly displayed).
class EditProfileContactSection extends StatelessWidget {
  final TextEditingController instagramController;
  final TextEditingController facebookController;
  final TextEditingController tiktokController;
  final TextEditingController twitterController;

  const EditProfileContactSection({
    super.key,
    required this.instagramController,
    required this.facebookController,
    required this.tiktokController,
    required this.twitterController,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Social Media (opsional)',
          style: context.typeRoles.bodyDense.copyWith(
            fontWeight: FontWeight.w600,
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          'Akun yang diisi akan tampil publik di profile. Kosongkan untuk menyembunyikan.',
          style: context.typeRoles.labelMicro.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 12),

        AppTextField(
          controller: instagramController,
          labelText: 'Instagram',
          hintText: '@username atau username',
          prefixIcon: Icons.camera_alt,
        ),
        const SizedBox(height: 16),

        AppTextField(
          controller: facebookController,
          labelText: 'Facebook',
          hintText: 'Page Name atau username',
          prefixIcon: Icons.facebook,
        ),
        const SizedBox(height: 16),

        AppTextField(
          controller: tiktokController,
          labelText: 'TikTok',
          hintText: '@username atau username',
          prefixIcon: Icons.play_circle_outline,
        ),
        const SizedBox(height: 16),

        AppTextField(
          controller: twitterController,
          labelText: 'Twitter',
          hintText: '@username atau username',
          prefixIcon: Icons.chat_bubble_outline,
        ),
      ],
    );
  }
}
