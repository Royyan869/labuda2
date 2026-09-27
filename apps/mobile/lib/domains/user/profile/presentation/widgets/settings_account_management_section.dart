import 'package:flutter/material.dart';
import 'package:labuda/generated/app_localizations.dart';

class SettingsAccountManagementSection extends StatelessWidget {
  final VoidCallback onSignOut;

  const SettingsAccountManagementSection({super.key, required this.onSignOut});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.manage_accounts,
          l10n.accountManagement,
          scheme,
        ),
        _buildSettingsTile(
          icon: Icons.logout_outlined,
          title: l10n.signOut,
          subtitle: l10n.signOutAccount,
          onTap: onSignOut,
          textColor: scheme.primary,
          scheme: scheme,
        ),
        const SizedBox(height: 32),
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
      leading: Icon(icon, color: textColor),
      title: Text(title, style: TextStyle(color: textColor)),
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
