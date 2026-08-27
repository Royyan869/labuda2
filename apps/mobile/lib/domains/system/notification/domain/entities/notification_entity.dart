import 'package:labuda/core/interfaces/i_notification_trigger.dart';

/// Notification Entity
///
/// Pure Dart business object representing a notification.
/// NO external dependencies (no Firebase, no Flutter).
///
/// Size: < 150 lines (per GUIDELINES)
class NotificationEntity {
  final String id;
  final String userId;
  final NotificationType type;
  final String title;
  final String body;
  final Map<String, dynamic>? data;
  final bool isRead;
  final DateTime createdAt;

  const NotificationEntity({
    required this.id,
    required this.userId,
    required this.type,
    required this.title,
    required this.body,
    this.data,
    required this.isRead,
    required this.createdAt,
  });

  /// Create copy with updated fields
  NotificationEntity copyWith({
    String? id,
    String? userId,
    NotificationType? type,
    String? title,
    String? body,
    Map<String, dynamic>? data,
    bool? isRead,
    DateTime? createdAt,
  }) {
    return NotificationEntity(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      type: type ?? this.type,
      title: title ?? this.title,
      body: body ?? this.body,
      data: data ?? this.data,
      isRead: isRead ?? this.isRead,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  /// Mark notification as read
  NotificationEntity markAsRead() {
    return copyWith(isRead: true);
  }

  /// Get screen route from notification data
  String? get screenRoute => data?['screen'] as String?;

  /// Get navigation params from notification data
  Map<String, dynamic>? get navigationParams =>
      data?['params'] as Map<String, dynamic>?;

  /// Check if notification is recent (< 24 hours)
  bool get isRecent {
    final now = DateTime.now();
    final difference = now.difference(createdAt);
    return difference.inHours < 24;
  }

  /// Check if notification requires user action
  bool get requiresAction {
    // Moderation notifications that need attention
    if ((type == NotificationType.moderationContentRemoved ||
            type == NotificationType.moderationCommentRemoved ||
            type == NotificationType.moderationListingRemoved ||
            type == NotificationType.moderationAuctionRemoved ||
            type == NotificationType.moderationUserSuspended ||
            type == NotificationType.moderationWarningIssued) &&
        !isRead) {
      return true;
    }

    // Support tickets waiting for user response
    if (type == NotificationType.supportTicketCreated) {
      final status = data?['status'] as String?;
      return status == 'open' && !isRead;
    }
    if (type == NotificationType.supportTicketWaitingUser) {
      return !isRead;
    }

    return false;
  }

  /// Get time ago string
  String get timeAgo {
    final now = DateTime.now();
    final difference = now.difference(createdAt);

    if (difference.inDays > 7) {
      return '${difference.inDays ~/ 7} weeks ago';
    } else if (difference.inDays > 0) {
      return '${difference.inDays} days ago';
    } else if (difference.inHours > 0) {
      return '${difference.inHours} hours ago';
    } else if (difference.inMinutes > 0) {
      return '${difference.inMinutes} minutes ago';
    } else {
      return 'Just now';
    }
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is NotificationEntity &&
        other.id == id &&
        other.userId == userId &&
        other.type == type &&
        other.title == title &&
        other.body == body &&
        other.isRead == isRead &&
        other.createdAt == createdAt;
  }

  @override
  int get hashCode {
    return id.hashCode ^
        userId.hashCode ^
        type.hashCode ^
        title.hashCode ^
        body.hashCode ^
        isRead.hashCode ^
        createdAt.hashCode;
  }

  @override
  String toString() {
    return 'NotificationEntity(id: $id, userId: $userId, type: $type, title: $title, isRead: $isRead, createdAt: $createdAt)';
  }
}
