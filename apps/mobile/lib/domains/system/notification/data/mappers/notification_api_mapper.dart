// Notification API Mapper
// Converts between API models and Domain entities

// External
import 'package:labuda/core/interfaces/i_notification_trigger.dart';

// Internal
import 'package:labuda/domains/system/notification/data/models/api/notification_api_models.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';

class NotificationApiMapper {
  // ============================================================================
  // Notification Response → Domain Entity
  // ============================================================================

  /// Map NotificationResponse to NotificationEntity
  static NotificationEntity toEntity(NotificationResponse response) {
    return NotificationEntity(
      id: response.id,
      userId: response.userId,
      type: _mapNotificationType(response.type),
      title: response.title,
      body: response.body,
      data: response.data ?? {},
      isRead: response.isRead,
      createdAt: response.createdAt,
    );
  }

  /// Map list of NotificationResponse to list of NotificationEntity
  static List<NotificationEntity> toEntityList(
    List<NotificationResponse> responses,
  ) {
    return responses.map((r) => toEntity(r)).toList();
  }

  // ============================================================================
  // Notification Preferences Response → Domain Entity
  // ============================================================================

  /// Map NotificationPreferencesResponse to NotificationPreferenceEntity
  /// [userId] is required since backend response doesn't include it
  ///
  /// BATCH N2: Ghost field handling
  /// - auctionNotifications from backend is ignored (not mapped to entity)
  /// - listingRecommendations not in backend (client-side only, now removed)
  static NotificationPreferenceEntity toPreferenceEntity(
    NotificationPreferencesResponse response,
    String userId,
  ) {
    return NotificationPreferenceEntity(
      userId: userId,
      pushEnabled: response.pushEnabled,
      orderNotifications: response.orderNotifications,
      chatNotifications: response.chatNotifications,
      securityAlerts: response.securityAlerts,
      marketingNotifications: response.marketingNotifications,
      // Ghost fields ignored:
      // - auctionNotifications: backend has field but doesn't emit notifications
      // - listingRecommendations: never implemented
    );
  }

  // ============================================================================
  // Domain Entity → Update Request
  // ============================================================================

  /// Map NotificationPreferenceEntity to UpdateNotificationPreferencesRequest
  ///
  /// BATCH N2: Ghost field handling
  /// - auctionNotifications set to false in request (backend has field but unused)
  /// - listingRecommendations not sent (backend doesn't have it)
  static UpdateNotificationPreferencesRequest toUpdateRequest(
    NotificationPreferenceEntity entity,
  ) {
    return UpdateNotificationPreferencesRequest(
      pushEnabled: entity.pushEnabled,
      orderNotifications: entity.orderNotifications,
      chatNotifications: entity.chatNotifications,
      securityAlerts: entity.securityAlerts,
      marketingNotifications: entity.marketingNotifications,
      auctionNotifications: false, // Ghost field: always false
      // Other fields not used by Flutter entity, letting backend use defaults
    );
  }

  // ============================================================================
  // NotificationType Enum Mapping
  // ============================================================================

  /// Map backend notification type string to NotificationType enum.
  ///
  /// Matches on .value (the backend wire string), falling back to
  /// announcement for unknown types so new backend types don't crash the app.
  static NotificationType _mapNotificationType(String type) {
    return NotificationType.fromString(type);
  }

  static String notificationTypeToString(NotificationType type) {
    return type.value;
  }
}
