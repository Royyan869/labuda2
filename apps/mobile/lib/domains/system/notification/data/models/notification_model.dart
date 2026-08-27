/// Notification Model
///
/// Data model untuk notification entities.
/// Converts between JSON dan domain entities.
///
/// FIRESTORE SUNSET (2025-02-20): Firestore methods removed.
/// Now uses JSON for Backend API communication.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';

class NotificationModel {
  final String id;
  final String userId;
  final String type;
  final String title;
  final String body;
  final Map<String, dynamic>? data;
  final bool isRead;
  final DateTime createdAt;

  NotificationModel({
    required this.id,
    required this.userId,
    required this.type,
    required this.title,
    required this.body,
    this.data,
    required this.isRead,
    required this.createdAt,
  });

  /// Convert to domain entity
  NotificationEntity toEntity() {
    return NotificationEntity(
      id: id,
      userId: userId,
      type: NotificationType.fromString(type),
      title: title,
      body: body,
      data: data,
      isRead: isRead,
      createdAt: createdAt,
    );
  }

  /// Create model from domain entity
  factory NotificationModel.fromEntity(NotificationEntity entity) {
    return NotificationModel(
      id: entity.id,
      userId: entity.userId,
      type: entity.type.value,
      title: entity.title,
      body: entity.body,
      data: entity.data,
      isRead: entity.isRead,
      createdAt: entity.createdAt,
    );
  }

  /// Create from JSON (Backend API)
  factory NotificationModel.fromJson(Map<String, dynamic> json) {
    return NotificationModel(
      id: json['id'] as String,
      userId: json['userId'] as String,
      type: json['type'] as String,
      title: json['title'] as String,
      body: json['body'] as String,
      data: json['data'] as Map<String, dynamic>?,
      isRead: json['isRead'] as bool? ?? false,
      createdAt: json['createdAt'] != null
          ? DateTime.parse(json['createdAt'] as String)
          : DateTime.now(),
    );
  }

  /// Convert to JSON (Backend API)
  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'userId': userId,
      'type': type,
      'title': title,
      'body': body,
      'data': data,
      'isRead': isRead,
      'createdAt': createdAt.toIso8601String(),
    };
  }

  @override
  String toString() {
    return 'NotificationModel(id: $id, userId: $userId, type: $type, title: $title, isRead: $isRead)';
  }
}
