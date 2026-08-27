/// Transaction Preferences Group
///
/// Order notification preferences.
/// Extracted from notification_settings_screen.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/preference_toggle_widget.dart';

// Flutter
import 'package:flutter/material.dart';

class TransactionPreferencesGroup extends StatelessWidget {
  final NotificationPreferenceEntity preferences;
  final Function(bool) onOrderNotificationsChanged;

  const TransactionPreferencesGroup({
    super.key,
    required this.preferences,
    required this.onOrderNotificationsChanged,
  });

  @override
  Widget build(BuildContext context) {
    return PreferenceToggleWidget(
      icon: Icons.shopping_bag_outlined,
      iconColor: Colors.green[700]!,
      title: 'Order Notifications',
      subtitle: 'Your order status updates',
      value: preferences.orderNotifications,
      enabled: preferences.pushEnabled,
      onChanged: onOrderNotificationsChanged,
    );
  }
}
