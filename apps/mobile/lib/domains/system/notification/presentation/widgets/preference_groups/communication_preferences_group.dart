/// Communication Preferences Group
///
/// Chat notification preferences.
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

class CommunicationPreferencesGroup extends StatelessWidget {
  final NotificationPreferenceEntity preferences;
  final Function(bool) onChatNotificationsChanged;

  const CommunicationPreferencesGroup({
    super.key,
    required this.preferences,
    required this.onChatNotificationsChanged,
  });

  @override
  Widget build(BuildContext context) {
    return PreferenceToggleWidget(
      icon: Icons.chat_bubble_outline,
      iconColor: AppColors.primaryRed,
      title: 'Chat Notifications',
      subtitle: 'New messages from sellers or buyers',
      value: preferences.chatNotifications,
      enabled: preferences.pushEnabled,
      onChanged: onChatNotificationsChanged,
    );
  }
}
