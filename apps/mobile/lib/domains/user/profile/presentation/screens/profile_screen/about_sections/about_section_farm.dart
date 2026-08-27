import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart' show ProfileAboutData;
import 'package:labuda/domains/user/profile/presentation/widgets/profile_info_row.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

/// Farm Info section - displays farm details for sellers
class AboutSectionFarm extends StatelessWidget {
  final ProfileAboutData data;

  const AboutSectionFarm({super.key, required this.data});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final farmInfo = data.farmInfo!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (farmInfo.farmName.isNotEmpty)
          ProfileInfoRow(label: 'Farm Name', value: farmInfo.farmName),

        if (farmInfo.establishedDate != null)
          ProfileInfoRow(
            label: 'Established Since',
            value: DateFormat('yyyy').format(farmInfo.establishedDate!),
          ),

        if (farmInfo.specialties != null && farmInfo.specialties!.isNotEmpty)
          ProfileInfoRow(
            label: 'Specialties',
            value: farmInfo.specialties!.join(', '),
          ),

        // Canonical seller description comes from AuthUser.bio
        if (data.bio.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            'Description',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            data.bio,
            style: TextStyle(
              fontSize: 13,
              height: 1.5,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],

        // Website
        if (farmInfo.farmWebsite != null &&
            farmInfo.farmWebsite!.isNotEmpty) ...[
          const SizedBox(height: 12),
          InkWell(
            onTap: () => _launchUrl(farmInfo.farmWebsite!),
            child: Row(
              children: [
                const Icon(Icons.language, size: 16, color: AppColors.primary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    farmInfo.farmWebsite!,
                    style: const TextStyle(
                      fontSize: 14,
                      color: AppColors.primary,
                      decoration: TextDecoration.underline,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Future<void> _launchUrl(String urlString) async {
    final url = Uri.parse(
      urlString.startsWith('http') ? urlString : 'https://$urlString',
    );
    if (await canLaunchUrl(url)) {
      await launchUrl(url, mode: LaunchMode.externalApplication);
    }
  }
}
