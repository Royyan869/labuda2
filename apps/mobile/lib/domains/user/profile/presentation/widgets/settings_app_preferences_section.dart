import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';

class SettingsAppPreferencesSection extends StatelessWidget {
  final void Function(String route)? onNavigate;

  const SettingsAppPreferencesSection({super.key, this.onNavigate});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.tune,
          l10n.appPreferences,
          isDark,
        ),
        const ThemeSelectorTile(),
        const LanguageSelectorTile(),
        if (onNavigate != null)
          _buildSettingsTile(
            icon: Icons.notifications_outlined,
            title: 'Notification Settings',
            subtitle: 'Manage notification preferences',
            onTap: () => onNavigate!('notifications'),
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
  }) {
    return ListTile(
      leading: Icon(
        icon,
        color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
      ),
      title: Text(
        title,
        style: TextStyle(
          color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
        ),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          fontSize: 13,
        ),
      ),
      trailing: Icon(
        Icons.arrow_forward_ios,
        size: 16,
        color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
      ),
      onTap: onTap,
    );
  }
}
