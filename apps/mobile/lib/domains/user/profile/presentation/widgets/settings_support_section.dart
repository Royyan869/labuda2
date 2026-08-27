import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/shared.dart'; // R3.1: Import for AppSnackBar
import 'package:labuda/domains/system/support/support.dart';

class SettingsSupportSection extends ConsumerWidget {
  final Function(String) onNavigate;

  const SettingsSupportSection({super.key, required this.onNavigate});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.support_agent,
          l10n.supportLegal,
          isDark,
        ),
        _buildSettingsTile(
          icon: Icons.help_outline,
          title: l10n.helpSupportTitle,
          subtitle: l10n.getHelpContactSupport,
          onTap: () => onNavigate('helpSupport'),
          isDark: isDark,
        ),
        // PHASE 2 HARDENING: Add "My Tickets" entry point
        _buildSettingsTile(
          icon: Icons.confirmation_number_outlined,
          title: 'Tiket Saya',
          subtitle: 'Lihat tiket bantuan Anda',
          onTap: () => _handleMyTicketsTap(context, ref),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.description_outlined,
          title: l10n.termsOfService,
          subtitle: l10n.readTermsConditions,
          onTap: () => onNavigate('termsOfService'),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.privacy_tip_outlined,
          title: l10n.privacyPolicy,
          subtitle: l10n.learnDataProtection,
          onTap: () => onNavigate('privacyPolicy'),
          isDark: isDark,
        ),
        _buildSettingsTile(
          icon: Icons.info_outline,
          title: l10n.aboutLABUDA,
          subtitle: l10n.appVersionInformation,
          onTap: () => onNavigate('about'),
          isDark: isDark,
        ),
      ],
    );
  }

  // PHASE 2 HARDENING: Handle "My Tickets" tap
  void _handleMyTicketsTap(BuildContext context, WidgetRef ref) {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      AppSnackBar.showError(context, 'Silakan login terlebih dahulu');
      return;
    }

    // Open the support ticket list directly so "My Tickets" stays on the
    // support surface instead of dropping into generic chat.
    Navigator.of(context).push(
      MaterialPageRoute(builder: (context) => const SupportTicketsListScreen()),
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
