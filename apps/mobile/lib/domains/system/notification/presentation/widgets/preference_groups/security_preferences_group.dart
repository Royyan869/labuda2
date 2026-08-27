/// Security Preferences Group
///
/// Security alert notification preferences.
/// Extracted from notification_settings_screen.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/preference_toggle_widget.dart';

// Flutter
import 'package:flutter/material.dart';

class SecurityPreferencesGroup extends StatelessWidget {
  final NotificationPreferenceEntity preferences;
  final Function(bool) onSecurityAlertsChanged;

  const SecurityPreferencesGroup({
    super.key,
    required this.preferences,
    required this.onSecurityAlertsChanged,
  });

  @override
  Widget build(BuildContext context) {
    return PreferenceToggleWidget(
      icon: Icons.security_outlined,
      iconColor: Colors.orange[700]!,
      title: 'Peringatan Keamanan',
      subtitle: 'Login dari perangkat baru & aktivitas mencurigakan',
      value: preferences.securityAlerts,
      enabled: preferences.pushEnabled,
      onChanged: onSecurityAlertsChanged,
    );
  }
}
