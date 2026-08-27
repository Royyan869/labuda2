import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart' show ProfileAboutData;
import 'package:labuda/domains/user/profile/presentation/widgets/social_media_chip.dart';

/// Contact Information section - displays email, phone, and social media
class AboutSectionContact extends StatelessWidget {
  final ProfileAboutData data;
  final bool isOwnProfile;

  const AboutSectionContact({
    super.key,
    required this.data,
    required this.isOwnProfile,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Email
        if (data.isEmailPublic || isOwnProfile) ...[
          if (data.maskedEmail != null) ...[
            _buildContactRow(
              icon: Icons.email_outlined,
              text: data.maskedEmail!,
              isDark: isDark,
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Phone
        if (data.isPhonePublic || isOwnProfile) ...[
          if (data.maskedPhone != null) ...[
            _buildContactRow(
              icon: Icons.phone_outlined,
              text: data.maskedPhone!,
              isDark: isDark,
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Social Media
        if ((data.isSocialMediaPublic || isOwnProfile) &&
            data.hasSocialMedia) ...[
          Divider(
            color: isDark ? AppColors.neutralGray600 : AppColors.neutralGray300,
          ),
          const SizedBox(height: 8),
          Text(
            'Social Media',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 12),
          _buildSocialMediaLinks(isDark),
        ],
      ],
    );
  }

  Widget _buildContactRow({
    required IconData icon,
    required String text,
    required bool isDark,
  }) {
    return Row(
      children: [
        Icon(
          icon,
          size: 18,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildSocialMediaLinks(bool isDark) {
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      children: [
        if (data.instagramHandle != null)
          SocialMediaChip(
            icon: Icons.camera_alt,
            label: data.instagramHandle!,
            url: _getInstagramUrl(data.instagramHandle!),
          ),
        if (data.facebookHandle != null)
          SocialMediaChip(
            icon: Icons.facebook,
            label: data.facebookHandle!,
            url: _getFacebookUrl(data.facebookHandle!),
          ),
        if (data.tiktokHandle != null)
          SocialMediaChip(
            icon: Icons.play_circle_outline,
            label: data.tiktokHandle!,
            url: _getTiktokUrl(data.tiktokHandle!),
          ),
        if (data.twitterHandle != null)
          SocialMediaChip(
            icon: Icons.chat_bubble_outline,
            label: data.twitterHandle!,
            url: _getTwitterUrl(data.twitterHandle!),
          ),
      ],
    );
  }

  // Social media URL constructors
  String _getInstagramUrl(String handle) {
    final cleanHandle = handle.replaceAll('@', '');
    return 'https://instagram.com/$cleanHandle';
  }

  String _getFacebookUrl(String handle) {
    return 'https://facebook.com/$handle';
  }

  String _getTiktokUrl(String handle) {
    final cleanHandle = handle.replaceAll('@', '');
    return 'https://tiktok.com/@$cleanHandle';
  }

  String _getTwitterUrl(String handle) {
    final cleanHandle = handle.replaceAll('@', '');
    return 'https://twitter.com/$cleanHandle';
  }
}
