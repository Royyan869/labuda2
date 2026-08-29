// Notification API Models
// DTOs matching Go backend communication domain

import 'package:json_annotation/json_annotation.dart';

part 'notification_api_models.g.dart';

// ============================================================================
// Core Notification DTOs
// ============================================================================

/// Notification response from API
@JsonSerializable(createFactory: false)
class NotificationResponse {
  final String id;
  final String userId;
  final String type; // NotificationType enum as string
  final String title;
  final String body;
  final String? imageUrl;
  final Map<String, dynamic>? data; // Navigation data
  final bool isRead;
  final DateTime? readAt;
  final String? groupKey; // For notification grouping
  final DateTime createdAt;

  const NotificationResponse({
    required this.id,
    required this.userId,
    required this.type,
    required this.title,
    required this.body,
    this.imageUrl,
    this.data,
    required this.isRead,
    this.readAt,
    this.groupKey,
    required this.createdAt,
  });

  factory NotificationResponse.fromJson(Map<String, dynamic> json) {
    return NotificationResponse(
      id: json['id'] as String? ?? '',
      userId: json['user_id'] as String? ?? json['userId'] as String? ?? '',
      type: json['type'] as String? ?? '',
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      imageUrl: json['image_url'] as String? ?? json['imageUrl'] as String?,
      data: json['data'] as Map<String, dynamic>?,
      isRead: json['is_read'] as bool? ?? json['isRead'] as bool? ?? false,
      readAt: (json['read_at'] as String? ?? json['readAt'] as String?) != null
          ? DateTime.tryParse(
              (json['read_at'] as String? ?? json['readAt'] as String?)!,
            )
          : null,
      groupKey: json['group_key'] as String? ?? json['groupKey'] as String?,
      createdAt:
          DateTime.tryParse(
            json['created_at'] as String? ?? json['createdAt'] as String? ?? '',
          ) ??
          DateTime.now(),
    );
  }
  Map<String, dynamic> toJson() => _$NotificationResponseToJson(this);
}

/// List notifications response with pagination
@JsonSerializable(createFactory: false)
class ListNotificationsResponse {
  final List<NotificationResponse> notifications;
  final int totalCount;
  final int unreadCount;
  final int page;
  final int perPage;

  const ListNotificationsResponse({
    required this.notifications,
    required this.totalCount,
    required this.unreadCount,
    required this.page,
    required this.perPage,
  });

  factory ListNotificationsResponse.fromJson(Map<String, dynamic> json) {
    final list = (json['notifications'] as List<dynamic>? ?? const [])
        .whereType<Map<String, dynamic>>()
        .map(NotificationResponse.fromJson)
        .toList();
    return ListNotificationsResponse(
      notifications: list,
      totalCount:
          json['total_count'] as int? ??
          json['totalCount'] as int? ??
          list.length,
      unreadCount:
          json['unread_count'] as int? ?? json['unreadCount'] as int? ?? 0,
      page:
          json['page'] as int? ??
          (((json['offset'] as int? ?? 0) ~/ (json['limit'] as int? ?? 20)) +
              1),
      perPage:
          json['per_page'] as int? ??
          json['perPage'] as int? ??
          json['limit'] as int? ??
          20,
    );
  }
  Map<String, dynamic> toJson() => _$ListNotificationsResponseToJson(this);
}

/// Mark notification as read request
@JsonSerializable()
class MarkNotificationReadRequest {
  final List<String>? notificationIds; // Specific IDs to mark
  final bool? all; // Mark all as read

  const MarkNotificationReadRequest({this.notificationIds, this.all});

  factory MarkNotificationReadRequest.fromJson(Map<String, dynamic> json) =>
      _$MarkNotificationReadRequestFromJson(json);
  Map<String, dynamic> toJson() => _$MarkNotificationReadRequestToJson(this);
}

/// Mark notifications as read by entity request
/// Used for cross-domain sync (e.g., chat read → chat notifications read)
class MarkAsReadByEntityRequest {
  final String entityType; // e.g., "chat"
  final String entityId; // UUID of the entity

  const MarkAsReadByEntityRequest({
    required this.entityType,
    required this.entityId,
  });

  factory MarkAsReadByEntityRequest.fromJson(Map<String, dynamic> json) =>
      MarkAsReadByEntityRequest(
        entityType: json['entity_type'] as String? ?? '',
        entityId: json['entity_id'] as String? ?? '',
      );
  Map<String, dynamic> toJson() => {
    'entity_type': entityType,
    'entity_id': entityId,
  };
}

/// Unread count response from canonical backend endpoint.
class UnreadCountResponse {
  final int count;

  const UnreadCountResponse({required this.count});

  factory UnreadCountResponse.fromJson(Map<String, dynamic> json) {
    return UnreadCountResponse(count: json['count'] as int? ?? 0);
  }
}

// ============================================================================
// FCM Token DTOs
// ============================================================================

/// Register FCM token request
class RegisterFCMTokenRequest {
  final String token;
  final String platform; // "android" | "ios" | "web"
  final String? deviceId;
  final String? deviceName;
  final String? appVersion;

  const RegisterFCMTokenRequest({
    required this.token,
    required this.platform,
    this.deviceId,
    this.deviceName,
    this.appVersion,
  });

  factory RegisterFCMTokenRequest.fromJson(
    Map<String, dynamic> json,
  ) => RegisterFCMTokenRequest(
    token: json['token'] as String? ?? '',
    platform: json['platform'] as String? ?? '',
    deviceId: json['device_id'] as String? ?? json['deviceId'] as String?,
    deviceName: json['device_name'] as String? ?? json['deviceName'] as String?,
    appVersion: json['app_version'] as String? ?? json['appVersion'] as String?,
  );
  Map<String, dynamic> toJson() => {
    'token': token,
    'platform': platform,
    if (deviceId != null) 'device_id': deviceId,
    if (deviceName != null) 'device_name': deviceName,
    if (appVersion != null) 'app_version': appVersion,
  };
}

/// FCM token response
@JsonSerializable(createFactory: false)
class FCMTokenResponse {
  final String id;
  final String userId;
  final String token;
  final String platform;
  final String? deviceId;
  final String? deviceName;
  final String? appVersion;
  final bool isActive;
  final DateTime? lastUsedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  const FCMTokenResponse({
    required this.id,
    required this.userId,
    required this.token,
    required this.platform,
    this.deviceId,
    this.deviceName,
    this.appVersion,
    required this.isActive,
    this.lastUsedAt,
    required this.createdAt,
    required this.updatedAt,
  });

  factory FCMTokenResponse.fromJson(
    Map<String, dynamic> json,
  ) => FCMTokenResponse(
    // Canonical backend currently returns minimal {success, token_id}.
    id: json['token_id'] as String? ?? json['id'] as String? ?? '',
    userId: json['user_id'] as String? ?? json['userId'] as String? ?? '',
    token: json['token'] as String? ?? '',
    platform: json['platform'] as String? ?? '',
    deviceId: json['device_id'] as String? ?? json['deviceId'] as String?,
    deviceName: json['device_name'] as String? ?? json['deviceName'] as String?,
    appVersion: json['app_version'] as String? ?? json['appVersion'] as String?,
    isActive: json['is_active'] as bool? ?? json['isActive'] as bool? ?? true,
    lastUsedAt:
        (json['last_used_at'] as String? ?? json['lastUsedAt'] as String?) !=
            null
        ? DateTime.tryParse(
            (json['last_used_at'] as String? ?? json['lastUsedAt'] as String?)!,
          )
        : null,
    createdAt:
        DateTime.tryParse(
          json['created_at'] as String? ?? json['createdAt'] as String? ?? '',
        ) ??
        DateTime.now(),
    updatedAt:
        DateTime.tryParse(
          json['updated_at'] as String? ?? json['updatedAt'] as String? ?? '',
        ) ??
        DateTime.now(),
  );
  Map<String, dynamic> toJson() => _$FCMTokenResponseToJson(this);
}

/// List FCM tokens response
@JsonSerializable()
class ListFCMTokensResponse {
  final List<FCMTokenResponse> tokens;
  final int total;

  const ListFCMTokensResponse({required this.tokens, required this.total});

  factory ListFCMTokensResponse.fromJson(Map<String, dynamic> json) =>
      _$ListFCMTokensResponseFromJson(json);
  Map<String, dynamic> toJson() => _$ListFCMTokensResponseToJson(this);
}

// ============================================================================
// Notification Preferences DTOs
// ============================================================================

/// Notification preferences response
@JsonSerializable()
class NotificationPreferencesResponse {
  // Global settings
  final bool pushEnabled;
  final bool emailEnabled;
  final bool quietHoursEnabled;
  final String? quietHoursStart; // "22:00"
  final String? quietHoursEnd; // "07:00"

  // Category preferences
  final bool chatNotifications;
  final bool orderNotifications;
  final bool auctionNotifications;
  final bool socialNotifications;
  final bool paymentNotifications;
  final bool securityAlerts;
  final bool marketingNotifications;

  // Detailed preferences
  final bool newMessageSound;
  final bool newMessageVibrate;
  final bool showMessagePreview;

  const NotificationPreferencesResponse({
    required this.pushEnabled,
    required this.emailEnabled,
    required this.quietHoursEnabled,
    this.quietHoursStart,
    this.quietHoursEnd,
    required this.chatNotifications,
    required this.orderNotifications,
    required this.auctionNotifications,
    required this.socialNotifications,
    required this.paymentNotifications,
    required this.securityAlerts,
    required this.marketingNotifications,
    required this.newMessageSound,
    required this.newMessageVibrate,
    required this.showMessagePreview,
  });

  factory NotificationPreferencesResponse.fromJson(Map<String, dynamic> json) =>
      _$NotificationPreferencesResponseFromJson(json);
  Map<String, dynamic> toJson() =>
      _$NotificationPreferencesResponseToJson(this);
}

/// Update notification preferences request
@JsonSerializable()
class UpdateNotificationPreferencesRequest {
  // All fields optional for partial updates
  final bool? pushEnabled;
  final bool? emailEnabled;
  final bool? quietHoursEnabled;
  final String? quietHoursStart;
  final String? quietHoursEnd;
  final bool? chatNotifications;
  final bool? orderNotifications;
  final bool? auctionNotifications;
  final bool? socialNotifications;
  final bool? paymentNotifications;
  final bool? securityAlerts;
  final bool? marketingNotifications;
  final bool? newMessageSound;
  final bool? newMessageVibrate;
  final bool? showMessagePreview;

  const UpdateNotificationPreferencesRequest({
    this.pushEnabled,
    this.emailEnabled,
    this.quietHoursEnabled,
    this.quietHoursStart,
    this.quietHoursEnd,
    this.chatNotifications,
    this.orderNotifications,
    this.auctionNotifications,
    this.socialNotifications,
    this.paymentNotifications,
    this.securityAlerts,
    this.marketingNotifications,
    this.newMessageSound,
    this.newMessageVibrate,
    this.showMessagePreview,
  });

  factory UpdateNotificationPreferencesRequest.fromJson(
    Map<String, dynamic> json,
  ) => _$UpdateNotificationPreferencesRequestFromJson(json);
  Map<String, dynamic> toJson() =>
      _$UpdateNotificationPreferencesRequestToJson(this);
}

// ============================================================================
// Push Notification DTOs (for internal use / triggers)
// ============================================================================

/// Send push notification request (for notification triggers)
@JsonSerializable()
class SendPushNotificationRequest {
  final String userId;
  final String type;
  final String title;
  final String body;
  final String? imageUrl;
  final Map<String, dynamic>? data;
  final String? groupKey;
  final String? priority; // "high" | "normal" | "low"
  final int? ttl; // Time to live in seconds

  const SendPushNotificationRequest({
    required this.userId,
    required this.type,
    required this.title,
    required this.body,
    this.imageUrl,
    this.data,
    this.groupKey,
    this.priority,
    this.ttl,
  });

  factory SendPushNotificationRequest.fromJson(Map<String, dynamic> json) =>
      _$SendPushNotificationRequestFromJson(json);
  Map<String, dynamic> toJson() => _$SendPushNotificationRequestToJson(this);
}

/// Send batch push notification request
@JsonSerializable()
class SendBatchPushRequest {
  final List<String> userIds; // Max 1000
  final String type;
  final String title;
  final String body;
  final String? imageUrl;
  final Map<String, dynamic>? data;
  final String? groupKey;
  final String? priority;
  final int? ttl;

  const SendBatchPushRequest({
    required this.userIds,
    required this.type,
    required this.title,
    required this.body,
    this.imageUrl,
    this.data,
    this.groupKey,
    this.priority,
    this.ttl,
  });

  factory SendBatchPushRequest.fromJson(Map<String, dynamic> json) =>
      _$SendBatchPushRequestFromJson(json);
  Map<String, dynamic> toJson() => _$SendBatchPushRequestToJson(this);
}
