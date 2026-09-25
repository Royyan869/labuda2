import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Security & Privacy Section
/// Handles: Security, Privacy Settings, Blocked Users
///
/// Presence privacy is server-derived via `show_activity_status` – no local toggle.
class SettingsSecurityPrivacySection extends StatelessWidget {
  final Function(String) onNavigate;

  const SettingsSecurityPrivacySection({super.key, required this.onNavigate});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.security_outlined,
          'Security & Privacy',
          isDark,
        ),
        _buildSettingsTile(
          icon: Icons.security_outlined,
          title: l10n.security,
          subtitle: 'Password and active sessions',
          onTap: () => onNavigate('security'),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.block,
          title: 'Blocked Users',
          subtitle: 'Manage blocked accounts',
          onTap: () => onNavigate('blockedUsers'),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.report_outlined,
          title: 'My Reports',
          subtitle: 'View your submitted reports and status',
          onTap: () => onNavigate('myReports'),
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
