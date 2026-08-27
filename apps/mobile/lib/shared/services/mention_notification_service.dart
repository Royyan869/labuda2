import 'package:labuda/core/interfaces/i_notification_trigger.dart';

/// Mention Notification Service
///
/// Centralized service untuk send mention notifications.
/// Digunakan oleh semua modul yang support mentions (chat, comment, content).
class MentionNotificationService {
  final INotificationTrigger notificationTrigger;

  MentionNotificationService({required this.notificationTrigger});

  /// Send mention notification ke semua mentioned users
  ///
  /// Parameters:
  /// - mentionedUserIds: List of user IDs yang di-mention
  /// - authorId: ID user yang mention
  /// - authorName: Nama user yang mention
  /// - contentType: Tipe content (content, comment, chat)
  /// - contentPreview: Preview content (max 100 chars)
  /// - contentId: ID dari content (untuk navigation)
  /// - blockedUserIds: Set of user IDs that have blocked the author (for filtering)
  Future<void> sendMentionNotifications({
    required List<String> mentionedUserIds,
    required String authorId,
    required String authorName,
    required String contentType,
    required String contentPreview,
    required String contentId,
    String? chatId,
    Set<String>? blockedUserIds,
  }) async {
    if (mentionedUserIds.isEmpty) return;

    // Remove author from mentioned users (don't notify yourself)
    var usersToNotify = mentionedUserIds.where((id) => id != authorId).toList();

    if (usersToNotify.isEmpty) return;

    // BLOCK FILTERING: Remove users who have blocked the author
    // This prevents harassment via mentions
    if (blockedUserIds != null && blockedUserIds.isNotEmpty) {
      usersToNotify = usersToNotify.where((userId) {
        // Don't send notification to users who have blocked the author
        return !blockedUserIds.contains(userId);
      }).toList();
    }

    if (usersToNotify.isEmpty) return;

    // Build notification data based on content type
    final data = <String, dynamic>{
      'contentType': contentType,
      'contentId': contentId,
      'authorId': authorId,
      'authorName': authorName,
    };

    // Add specific navigation params based on content type
    switch (contentType) {
      case 'chat':
        data['screen'] = '/chat/$chatId';
        data['chatId'] = chatId;
        break;
      case 'comment':
      case 'content':
        data['screen'] = '/content/$contentId';
        data['contentId'] = contentId;
        break;
    }

    // Send batch notification
    await notificationTrigger.sendNotificationBatch(
      userIds: usersToNotify,
      type: NotificationType.mention,
      title: _buildTitle(authorName, contentType),
      body: _buildBody(contentPreview),
      data: data,
    );
  }

  /// Build notification title based on content type
  String _buildTitle(String authorName, String contentType) {
    switch (contentType) {
      case 'chat':
        return '$authorName mentioned you in a message';
      case 'comment':
        return '$authorName mentioned you in a comment';
      case 'content':
        return '$authorName mentioned you in content';
      default:
        return '$authorName mentioned you';
    }
  }

  /// Build notification body (content preview)
  String _buildBody(String contentPreview) {
    // Trim to max 100 characters
    if (contentPreview.length > 100) {
      return '${contentPreview.substring(0, 97)}...';
    }
    return contentPreview;
  }
}
