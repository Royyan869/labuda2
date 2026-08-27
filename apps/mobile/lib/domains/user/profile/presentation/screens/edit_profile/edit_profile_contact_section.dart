import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Contact & Social Media Fields for Edit Profile
class EditProfileContactSection extends StatelessWidget {
  final bool isEmailPublic;
  final bool isPhonePublic;
  final bool isSocialMediaPublic;
  final ValueChanged<bool> onEmailPublicChanged;
  final ValueChanged<bool> onPhonePublicChanged;
  final ValueChanged<bool> onSocialMediaPublicChanged;
  final TextEditingController instagramController;
  final TextEditingController facebookController;
  final TextEditingController tiktokController;
  final TextEditingController twitterController;

  const EditProfileContactSection({
    super.key,
    required this.isEmailPublic,
    required this.isPhonePublic,
    required this.isSocialMediaPublic,
    required this.onEmailPublicChanged,
    required this.onPhonePublicChanged,
    required this.onSocialMediaPublicChanged,
    required this.instagramController,
    required this.facebookController,
    required this.tiktokController,
    required this.twitterController,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Privacy Toggles Section
        Text(
          'Privacy Settings',
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
          ),
        ),
        const SizedBox(height: 8),

        _buildPrivacyToggle(
          title: 'Make Email Public',
          subtitle: 'Others can see your email address',
          value: isEmailPublic,
          onChanged: onEmailPublicChanged,
          isDark: isDark,
        ),

        _buildPrivacyToggle(
          title: 'Make Phone Public',
          subtitle: 'Others can see your phone number',
          value: isPhonePublic,
          onChanged: onPhonePublicChanged,
          isDark: isDark,
        ),

        const SizedBox(height: 24),

        // Social Media Section
        Text(
          'Social Media',
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
          ),
        ),
        const SizedBox(height: 8),

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
        const SizedBox(height: 16),

        _buildPrivacyToggle(
          title: 'Make Social Media Public',
          subtitle: 'Others can see your social media links',
          value: isSocialMediaPublic,
          onChanged: onSocialMediaPublicChanged,
          isDark: isDark,
        ),
      ],
    );
  }

  Widget _buildPrivacyToggle({
    required String title,
    required String subtitle,
    required bool value,
    required ValueChanged<bool> onChanged,
    required bool isDark,
  }) {
    return SwitchListTile(
      title: Text(title),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          fontSize: 12,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
      ),
      value: value,
      onChanged: onChanged,
      activeTrackColor: AppColors.primary.withValues(alpha: 0.5),
      activeThumbColor: AppColors.primary,
      contentPadding: EdgeInsets.zero,
    );
  }
}
