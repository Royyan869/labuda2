/// Notification Preference Model
///
/// Data model untuk notification preferences.
/// Converts between JSON dan domain entities.
///
/// FIRESTORE SUNSET (2025-02-20): Firestore methods removed.
/// Now uses JSON for Backend API communication.
///
/// BATCH N2: Removed listingRecommendations and auctionNotifications (ghost features)
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';

class NotificationPreferenceModel {
  final String userId;
  final bool pushEnabled;
  final bool orderNotifications;
  final bool chatNotifications;
  final bool securityAlerts;
  final bool marketingNotifications;

  NotificationPreferenceModel({
    required this.userId,
    required this.pushEnabled,
    required this.orderNotifications,
    required this.chatNotifications,
    required this.securityAlerts,
    required this.marketingNotifications,
  });

  /// Convert to domain entity
  NotificationPreferenceEntity toEntity() {
    return NotificationPreferenceEntity(
      userId: userId,
      pushEnabled: pushEnabled,
      orderNotifications: orderNotifications,
      chatNotifications: chatNotifications,
      securityAlerts: securityAlerts,
      marketingNotifications: marketingNotifications,
    );
  }

  /// Create model from domain entity
  factory NotificationPreferenceModel.fromEntity(
    NotificationPreferenceEntity entity,
  ) {
    return NotificationPreferenceModel(
      userId: entity.userId,
      pushEnabled: entity.pushEnabled,
      orderNotifications: entity.orderNotifications,
      chatNotifications: entity.chatNotifications,
      securityAlerts: entity.securityAlerts,
      marketingNotifications: entity.marketingNotifications,
    );
  }

  /// Create from JSON (Backend API)
  factory NotificationPreferenceModel.fromJson(Map<String, dynamic> json) {
    return NotificationPreferenceModel(
      userId: json['userId'] as String,
      pushEnabled: json['pushEnabled'] as bool? ?? true,
      orderNotifications: json['orderNotifications'] as bool? ?? true,
      chatNotifications: json['chatNotifications'] as bool? ?? true,
      securityAlerts: json['securityAlerts'] as bool? ?? true,
      marketingNotifications: json['marketingNotifications'] as bool? ?? false,
    );
  }

  /// Convert to JSON (Backend API)
  Map<String, dynamic> toJson() {
    return {
      'userId': userId,
      'pushEnabled': pushEnabled,
      'orderNotifications': orderNotifications,
      'chatNotifications': chatNotifications,
      'securityAlerts': securityAlerts,
      'marketingNotifications': marketingNotifications,
    };
  }

  @override
  String toString() {
    return 'NotificationPreferenceModel(userId: $userId, pushEnabled: $pushEnabled)';
  }
}
