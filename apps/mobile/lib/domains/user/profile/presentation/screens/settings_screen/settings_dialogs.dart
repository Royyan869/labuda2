import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';

/// About LABUDA dialog
class SettingsAboutDialog extends StatelessWidget {
  const SettingsAboutDialog({super.key});

  static void show(BuildContext context) {
    showDialog(
      context: context,
      builder: (context) => const SettingsAboutDialog(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final currentYear = DateTime.now().year.toString();

    return AlertDialog(
      title: Text(l10n.aboutLABUDA),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(l10n.koiSocialCommercePlatform),
          const SizedBox(height: 8),
          Text(l10n.version),
          const SizedBox(height: 8),
          Text(l10n.copyrightLabudaTeam(currentYear)),
          const SizedBox(height: 16),
          Text(l10n.labudaDescription, style: const TextStyle(fontSize: 14)),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.close),
        ),
      ],
    );
  }
}

/// Sign out confirmation dialog
class SettingsSignOutDialog extends StatelessWidget {
  final VoidCallback onSignOut;

  const SettingsSignOutDialog({super.key, required this.onSignOut});

  static void show(BuildContext context, VoidCallback onSignOut) {
    showDialog(
      context: context,
      builder: (context) => SettingsSignOutDialog(onSignOut: onSignOut),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return AlertDialog(
      title: Text(l10n.signOut),
      content: Text(l10n.signOutConfirm),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.cancel),
        ),
        TextButton(
          onPressed: () {
            Navigator.of(context).pop();
            onSignOut();
          },
          child: Text(
            l10n.signOut,
            style: TextStyle(color: AppColors.primaryRed),
          ),
        ),
      ],
    );
  }
}
