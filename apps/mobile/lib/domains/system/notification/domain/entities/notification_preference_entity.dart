/// Notification Preference Entity
///
/// Pure Dart business object representing user notification preferences.
/// NO external dependencies (no Firebase, no Flutter).
///
/// BATCH N2: Removed listingRecommendations and auctionNotifications (ghost features)
/// - Listing recommendations: never implemented
/// - Auction notifications: backend does not emit auction notification events
///
/// Size: < 100 lines (per GUIDELINES)
class NotificationPreferenceEntity {
  final String userId;
  final bool pushEnabled;
  final bool orderNotifications;
  final bool chatNotifications;
  final bool securityAlerts;
  final bool marketingNotifications;

  const NotificationPreferenceEntity({
    required this.userId,
    required this.pushEnabled,
    required this.orderNotifications,
    required this.chatNotifications,
    required this.securityAlerts,
    required this.marketingNotifications,
  });

  /// Default preferences for new users
  factory NotificationPreferenceEntity.defaultPrefs(String userId) {
    return NotificationPreferenceEntity(
      userId: userId,
      pushEnabled: true,
      orderNotifications: true,
      chatNotifications: true,
      securityAlerts: true,
      marketingNotifications: false,
    );
  }

  /// Create copy with updated fields
  NotificationPreferenceEntity copyWith({
    String? userId,
    bool? pushEnabled,
    bool? orderNotifications,
    bool? chatNotifications,
    bool? securityAlerts,
    bool? marketingNotifications,
  }) {
    return NotificationPreferenceEntity(
      userId: userId ?? this.userId,
      pushEnabled: pushEnabled ?? this.pushEnabled,
      orderNotifications: orderNotifications ?? this.orderNotifications,
      chatNotifications: chatNotifications ?? this.chatNotifications,
      securityAlerts: securityAlerts ?? this.securityAlerts,
      marketingNotifications:
          marketingNotifications ?? this.marketingNotifications,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is NotificationPreferenceEntity &&
        other.userId == userId &&
        other.pushEnabled == pushEnabled &&
        other.orderNotifications == orderNotifications &&
        other.chatNotifications == chatNotifications &&
        other.securityAlerts == securityAlerts &&
        other.marketingNotifications == marketingNotifications;
  }

  @override
  int get hashCode {
    return userId.hashCode ^
        pushEnabled.hashCode ^
        orderNotifications.hashCode ^
        chatNotifications.hashCode ^
        securityAlerts.hashCode ^
        marketingNotifications.hashCode;
  }

  @override
  String toString() {
    return 'NotificationPreferenceEntity(userId: $userId, pushEnabled: $pushEnabled, orderNotifications: $orderNotifications, chatNotifications: $chatNotifications, securityAlerts: $securityAlerts)';
  }
}
