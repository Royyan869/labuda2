import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart' show ProfileAboutData;
import 'package:intl/intl.dart';

/// About section - displays bio, location, join date, last active
class AboutSectionAbout extends StatelessWidget {
  final ProfileAboutData data;

  const AboutSectionAbout({super.key, required this.data});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Bio
        if (data.bio.isNotEmpty) ...[
          Text(
            data.bio,
            style: TextStyle(
              fontSize: 14,
              height: 1.5,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),
        ],

        // Location
        if (data.location != null) ...[
          _buildInfoRow(
            icon: Icons.location_on_outlined,
            text: data.location!,
            isDark: isDark,
          ),
          const SizedBox(height: 8),
        ],

        // Join date
        _buildInfoRow(
          icon: Icons.calendar_today_outlined,
          text: _formatJoinDate(data.joinedAt),
          isDark: isDark,
        ),

        // Last active
        if (data.lastActiveAt != null) ...[
          const SizedBox(height: 8),
          _buildInfoRow(
            icon: Icons.access_time,
            text: _formatLastActive(data.lastActiveAt!),
            isDark: isDark,
          ),
        ],
      ],
    );
  }

  Widget _buildInfoRow({
    required IconData icon,
    required String text,
    required bool isDark,
  }) {
    return Row(
      children: [
        Icon(
          icon,
          size: 16,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
        ),
        const SizedBox(width: 8),
        Text(
          text,
          style: TextStyle(
            fontSize: 14,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          ),
        ),
      ],
    );
  }

  String _formatJoinDate(DateTime date) {
    return 'Joined ${DateFormat('MMMM yyyy').format(date)}';
  }

  String _formatLastActive(DateTime date) {
    final now = DateTime.now();
    final diff = now.difference(date);

    if (diff.inMinutes < 1) return 'Last active just now';
    if (diff.inMinutes < 60) return 'Last active ${diff.inMinutes} minutes ago';
    if (diff.inHours < 24) return 'Last active ${diff.inHours} hours ago';
    if (diff.inDays < 7) return 'Last active ${diff.inDays} days ago';

    return 'Last active on ${DateFormat('MMM d, yyyy').format(date)}';
  }
}
