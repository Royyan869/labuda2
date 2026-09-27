import 'package:flutter/material.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Profile & Identity Section
/// Handles: Profile, Personal Info, Address
class SettingsProfileIdentitySection extends StatelessWidget {
  final Function(String) onNavigate;

  const SettingsProfileIdentitySection({super.key, required this.onNavigate});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.person_outline,
          'Profile & Identity',
          scheme,
        ),
        _buildSettingsTile(
          icon: Icons.person_outline,
          title: l10n.editProfile,
          subtitle: 'Display name, username, bio, and photo',
          onTap: () => onNavigate('editProfile'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.badge_outlined,
          title: 'Personal Information',
          subtitle: 'Date of birth, phone, and KTP verification',
          onTap: () => onNavigate('personalInformation'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.location_on_outlined,
          title: 'Addresses',
          subtitle: 'Manage your addresses',
          onTap: () => onNavigate('address'),
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
