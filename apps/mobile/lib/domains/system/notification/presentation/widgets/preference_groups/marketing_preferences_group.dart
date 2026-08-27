/// Marketing Preferences Group
///
/// Marketing and promotional notification preferences.
/// Extracted from notification_settings_screen.
///
/// BATCH N2: Removed listingRecommendations and auctionNotifications (ghost features)
/// - Listing recommendations: never implemented
/// - Auction notifications: backend does not emit auction notification events
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/preference_toggle_widget.dart';

// Flutter
import 'package:flutter/material.dart';

class MarketingPreferencesGroup extends StatelessWidget {
  final NotificationPreferenceEntity preferences;
  final Function(bool) onMarketingNotificationsChanged;

  const MarketingPreferencesGroup({
    super.key,
    required this.preferences,
    required this.onMarketingNotificationsChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        PreferenceToggleWidget(
          icon: Icons.campaign_outlined,
          iconColor: Colors.cyan[700]!,
          title: 'Promotions & Announcements',
          subtitle: 'Special offers and latest news',
          value: preferences.marketingNotifications,
          enabled: preferences.pushEnabled,
          onChanged: onMarketingNotificationsChanged,
        ),
      ],
    );
  }
}
