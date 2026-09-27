import 'package:flutter/material.dart';
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
    final scheme = Theme.of(context).colorScheme;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.security_outlined,
          'Security & Privacy',
          scheme,
        ),
        _buildSettingsTile(
          icon: Icons.security_outlined,
          title: l10n.security,
          subtitle: 'Password and active sessions',
          onTap: () => onNavigate('security'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.block,
          title: 'Blocked Users',
          subtitle: 'Manage blocked accounts',
          onTap: () => onNavigate('blockedUsers'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.report_outlined,
          title: 'My Reports',
          subtitle: 'View your submitted reports and status',
          onTap: () => onNavigate('myReports'),
          scheme: scheme,
        ),
      ],
    );
  }

  Widget _buildSectionHeaderWithIcon(
    BuildContext context,
    IconData icon,
    String title,
    ColorScheme scheme,
  ) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Row(
        children: [
          Icon(
            icon,
            size: 20,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(width: 8),
          Text(
            title,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
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
    required ColorScheme scheme,
    Color? textColor,
  }) {
    return ListTile(
      leading: Icon(
        icon,
        color:
            textColor ??
            (scheme.onSurfaceVariant),
      ),
      title: Text(
        title,
        style: TextStyle(
          color:
              textColor ??
              (scheme.onSurface),
        ),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: Icon(
        Icons.chevron_right,
        color: scheme.onSurfaceVariant,
      ),
      onTap: onTap,
    );
  }
}
