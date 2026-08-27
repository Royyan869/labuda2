/// General Preferences Group
///
/// Master toggle for all notifications.
/// Extracted from notification_settings_screen.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/preference_toggle_widget.dart';

// Flutter
import 'package:flutter/material.dart';

class GeneralPreferencesGroup extends StatelessWidget {
  final NotificationPreferenceEntity preferences;
  final Function(bool) onPushEnabledChanged;

  const GeneralPreferencesGroup({
    super.key,
    required this.preferences,
    required this.onPushEnabledChanged,
  });

  @override
  Widget build(BuildContext context) {
    return PreferenceToggleWidget(
      icon: Icons.notifications_active,
      iconColor: AppColors.primaryRed,
      title: 'Enable Push Notifications',
      subtitle: 'Receive notifications for all activities',
      value: preferences.pushEnabled,
      onChanged: onPushEnabledChanged,
    );
  }
}
