import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

class SettingsAppPreferencesSection extends StatelessWidget {
  final void Function(String route)? onNavigate;

  const SettingsAppPreferencesSection({super.key, this.onNavigate});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.tune,
          l10n.appPreferences,
          scheme,
        ),
        const ThemeSelectorTile(),
        const LanguageSelectorTile(),
        if (onNavigate != null)
          _buildSettingsTile(
            context,
            icon: Icons.notifications_outlined,
            title: 'Notification Settings',
            subtitle: 'Manage notification preferences',
            onTap: () => onNavigate!('notifications'),
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
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p8,
      ),
      child: Row(
        children: [
          Icon(icon, size: AppIconSize.action, color: scheme.onSurfaceVariant),
          const SizedBox(width: 8),
          Text(
            title,
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSettingsTile(
    BuildContext context, {
    required IconData icon,
    required String title,
    required String subtitle,
    required VoidCallback onTap,
    required ColorScheme scheme,
  }) {
    return ListTile(
      leading: Icon(icon, color: scheme.onSurfaceVariant),
      title: Text(title, style: TextStyle(color: scheme.onSurface)),
      subtitle: Text(
        subtitle,
        style: context.typeRoles.bodyDense.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: Icon(
        Icons.arrow_forward_ios,
        size: AppIconSize.inlineGlyph,
        color: scheme.onSurfaceVariant,
      ),
      onTap: onTap,
    );
  }
}
