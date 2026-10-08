import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/generated/app_localizations.dart';

class SettingsSupportSection extends ConsumerWidget {
  final Function(String) onNavigate;

  const SettingsSupportSection({super.key, required this.onNavigate});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.support_agent,
          l10n.supportLegal,
          scheme,
        ),
        _buildSettingsTile(
          icon: Icons.help_outline,
          title: l10n.helpSupportTitle,
          subtitle: l10n.getHelpContactSupport,
          onTap: () => onNavigate('helpSupport'),
          scheme: scheme,
        ),
        // PHASE 2 HARDENING: Add "My Tickets" entry point
        _buildSettingsTile(
          icon: Icons.confirmation_number_outlined,
          title: 'Tiket Saya',
          subtitle: 'Lihat tiket bantuan Anda',
          onTap: () => _handleMyTicketsTap(context, ref),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.description_outlined,
          title: l10n.termsOfService,
          subtitle: l10n.readTermsConditions,
          onTap: () => onNavigate('termsOfService'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.privacy_tip_outlined,
          title: l10n.privacyPolicy,
          subtitle: l10n.learnDataProtection,
          onTap: () => onNavigate('privacyPolicy'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          icon: Icons.info_outline,
          title: l10n.aboutLABUDA,
          subtitle: l10n.appVersionInformation,
          onTap: () => onNavigate('about'),
          scheme: scheme,
        ),
      ],
    );
  }

  // PHASE 2 HARDENING: Handle "My Tickets" tap
  void _handleMyTicketsTap(BuildContext context, WidgetRef ref) {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

    // Open the support ticket list directly so "My Tickets" stays on the
    // support surface instead of dropping into generic chat.
    context.push(RoutePaths.supportTickets);
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

  Widget _buildSettingsTile({
    required IconData icon,
    required String title,
    required String subtitle,
    required VoidCallback onTap,
    required ColorScheme scheme,
    Color? textColor,
  }) {
    return ListTile(
      leading: Icon(icon, color: textColor ?? (scheme.onSurfaceVariant)),
      title: Text(
        title,
        style: TextStyle(color: textColor ?? (scheme.onSurface)),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(color: scheme.onSurfaceVariant),
      ),
      trailing: Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
      onTap: onTap,
    );
  }
}
