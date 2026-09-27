import 'package:flutter/material.dart';
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
    final scheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Email
        if (data.isEmailPublic || isOwnProfile) ...[
          if (data.maskedEmail != null) ...[
            _buildContactRow(
              icon: Icons.email_outlined,
              text: data.maskedEmail!,
              scheme: scheme,
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
              scheme: scheme,
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Social Media
        if ((data.isSocialMediaPublic || isOwnProfile) &&
            data.hasSocialMedia) ...[
          Divider(
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 8),
          Text(
            'Social Media',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 12),
          _buildSocialMediaLinks(),
        ],
      ],
    );
  }

  Widget _buildContactRow({
    required IconData icon,
    required String text,
    required ColorScheme scheme,
  }) {
    return Row(
      children: [
        Icon(
          icon,
          size: 18,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: 14,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildSocialMediaLinks() {
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
