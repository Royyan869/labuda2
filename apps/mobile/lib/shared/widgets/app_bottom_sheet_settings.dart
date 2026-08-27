import 'package:flutter/material.dart';
import 'app_bottom_sheet_actions.dart';

/// Settings Item Class
class SettingsItem {
  final String title;
  final String? subtitle;
  final IconData? icon;
  final Color? iconColor;
  final VoidCallback? onTap;
  final String? badge;

  const SettingsItem({
    required this.title,
    this.subtitle,
    this.icon,
    this.iconColor,
    this.onTap,
    this.badge,
  });
}

/// AppBottomSheet for settings
class AppBottomSheetSettings {
  /// Show settings bottom sheet
  static Future<void> showSettings({
    required BuildContext context,
    String title = 'Settings',
    List<SettingsItem>? customSettings,
  }) {
    return AppBottomSheetActions.showActions(
      context: context,
      title: title,
      actions:
          customSettings
              ?.map(
                (setting) => BottomSheetAction<void>(
                  title: setting.title,
                  subtitle: setting.subtitle,
                  icon: setting.icon,
                  onPressed: () {
                    Navigator.of(context).pop();
                    setting.onTap?.call();
                  },
                ),
              )
              .toList() ??
          _getDefaultSettings(),
    );
  }

  /// Get default settings options
  static List<BottomSheetAction<void>> _getDefaultSettings() {
    return [
      const BottomSheetAction<void>(
        title: 'Notifications',
        subtitle: 'Manage your notification preferences',
        icon: Icons.notifications_outlined,
      ),
      const BottomSheetAction<void>(
        title: 'Privacy',
        subtitle: 'Control your privacy settings',
        icon: Icons.privacy_tip_outlined,
      ),
      const BottomSheetAction<void>(
        title: 'Theme',
        subtitle: 'Change app appearance',
        icon: Icons.palette_outlined,
      ),
      const BottomSheetAction<void>(
        title: 'Language',
        subtitle: 'Select your preferred language',
        icon: Icons.language_outlined,
      ),
      const BottomSheetAction<void>(
        title: 'About',
        subtitle: 'App information and version',
        icon: Icons.info_outlined,
      ),
    ];
  }
}
