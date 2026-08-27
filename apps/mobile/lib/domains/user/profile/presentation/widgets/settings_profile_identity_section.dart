import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Profile & Identity Section
/// Handles: Profile, Personal Info, Address
class SettingsProfileIdentitySection extends StatelessWidget {
  final Function(String) onNavigate;

  const SettingsProfileIdentitySection({super.key, required this.onNavigate});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.person_outline,
          'Profile & Identity',
          isDark,
        ),
        _buildSettingsTile(
          icon: Icons.person_outline,
          title: l10n.editProfile,
          subtitle: 'Display name, username, bio, and photo',
          onTap: () => onNavigate('editProfile'),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.badge_outlined,
          title: 'Personal Information',
          subtitle: 'Date of birth, phone, and KTP verification',
          onTap: () => onNavigate('personalInformation'),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.location_on_outlined,
          title: 'Address & Contact',
          subtitle: 'Manage shipping addresses',
          onTap: () => onNavigate('address'),
          isDark: isDark,
        ),
      ],
    );
  }

  Widget _buildSectionHeaderWithIcon(
    BuildContext context,
    IconData icon,
    String title,
    bool isDark,
  ) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Row(
        children: [
          Icon(
            icon,
            size: 20,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          const SizedBox(width: 8),
          Text(
            title,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSettingsTile({
    required IconData icon,
    required String title,
    required String subtitle,
    required VoidCallback onTap,
    required bool isDark,
    Color? textColor,
  }) {
    return ListTile(
      leading: Icon(
        icon,
        color:
            textColor ??
            (isDark ? AppColors.neutralGray300 : AppColors.neutralGray700),
      ),
      title: Text(
        title,
        style: TextStyle(
          color:
              textColor ??
              (isDark ? AppColors.neutralWhite : AppColors.neutralGray900),
        ),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray600,
        ),
      ),
      trailing: Icon(
        Icons.chevron_right,
        color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
      ),
      onTap: onTap,
    );
  }
}
