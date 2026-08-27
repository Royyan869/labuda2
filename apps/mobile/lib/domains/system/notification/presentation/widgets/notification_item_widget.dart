/// Notification Item Widget
///
/// Professional notification item dengan:
/// - Visual indicator untuk unread
/// - Icon berdasarkan notification type
/// - Timestamp relative
/// - Tap handling
///
/// Size: < 200 lines (per GUIDELINES)
library;

import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/services/notification_display_service.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationItemWidget extends StatelessWidget {
  final NotificationEntity notification;
  final VoidCallback onTap;
  final VoidCallback? onDismissed;

  const NotificationItemWidget({
    super.key,
    required this.notification,
    required this.onTap,
    this.onDismissed,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    // Get display metadata from domain service
    final displayService = const NotificationDisplayService();
    final displayMetadata = displayService.getDisplayMetadata(
      notification.type,
    );

    return Material(
      color: notification.isRead
          ? theme.colorScheme.surface
          : (isDark
                ? theme.colorScheme.primaryContainer.withValues(alpha: 0.3)
                : theme.colorScheme.primaryContainer.withValues(alpha: 0.5)),
      child: InkWell(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Icon
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: _mapColor(
                    displayMetadata.color,
                  ).withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(
                  _mapIcon(displayMetadata.icon),
                  color: _mapColor(displayMetadata.color),
                  size: 24,
                ),
              ),
              const SizedBox(width: 12),

              // Content
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Title
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            notification.title,
                            style: theme.textTheme.bodyLarge?.copyWith(
                              fontWeight: notification.isRead
                                  ? FontWeight.w500
                                  : FontWeight.w600,
                              height: 1.3,
                            ),
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        if (!notification.isRead) ...[
                          const SizedBox(width: 8),
                          Container(
                            width: 8,
                            height: 8,
                            decoration: BoxDecoration(
                              color: theme.colorScheme.primary,
                              shape: BoxShape.circle,
                            ),
                          ),
                        ],
                      ],
                    ),
                    const SizedBox(height: 4),

                    // Body
                    Text(
                      notification.body,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        height: 1.4,
                        color: theme.colorScheme.onSurface.withValues(
                          alpha: 0.7,
                        ),
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 6),

                    // Timestamp & Badge
                    Row(
                      children: [
                        Icon(
                          Icons.access_time,
                          size: 14,
                          color: theme.colorScheme.onSurface.withValues(
                            alpha: 0.5,
                          ),
                        ),
                        const SizedBox(width: 4),
                        Text(
                          notification.timeAgo,
                          style: theme.textTheme.bodySmall?.copyWith(
                            fontWeight: FontWeight.w500,
                            color: theme.colorScheme.onSurface.withValues(
                              alpha: 0.6,
                            ),
                          ),
                        ),
                        if (notification.requiresAction) ...[
                          const SizedBox(width: 8),
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: Colors.red.withValues(alpha: 0.15),
                              borderRadius: BorderRadius.circular(4),
                              border: Border.all(
                                color: Colors.red.withValues(alpha: 0.3),
                                width: 1,
                              ),
                            ),
                            child: const Text(
                              'Perlu tindakan',
                              style: TextStyle(
                                fontSize: 10,
                                fontWeight: FontWeight.w700,
                                color: Colors.red,
                                letterSpacing: 0.3,
                              ),
                            ),
                          ),
                        ],
                        if (notification.isRecent &&
                            !notification.requiresAction) ...[
                          const SizedBox(width: 8),
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: Colors.orange.withValues(alpha: 0.2),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: const Text(
                              'BARU',
                              style: TextStyle(
                                fontSize: 11,
                                fontWeight: FontWeight.w700,
                                color: Colors.orange,
                                letterSpacing: 0.5,
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Map domain icon enum to Flutter IconData
  IconData _mapIcon(NotificationDisplayIcon icon) {
    switch (icon) {
      case NotificationDisplayIcon.shoppingBag:
        return Icons.shopping_bag_outlined;
      case NotificationDisplayIcon.cancel:
        return Icons.cancel_outlined;
      case NotificationDisplayIcon.assignmentReturn:
        return Icons.assignment_return_outlined;
      case NotificationDisplayIcon.checkCircle:
        return Icons.check_circle_outline;
      case NotificationDisplayIcon.error:
        return Icons.error_outline;
      case NotificationDisplayIcon.hourglassEmpty:
        return Icons.hourglass_empty_outlined;
      case NotificationDisplayIcon.verifiedUser:
        return Icons.verified_user_outlined;
      case NotificationDisplayIcon.chat:
        return Icons.chat_bubble_outline;
      case NotificationDisplayIcon.security:
        return Icons.security_outlined;
      case NotificationDisplayIcon.favorite:
        return Icons.favorite_outline;
      case NotificationDisplayIcon.comment:
        return Icons.comment_outlined;
      case NotificationDisplayIcon.alternateEmail:
        return Icons.alternate_email_outlined;
      case NotificationDisplayIcon.article:
        return Icons.article_outlined;
      case NotificationDisplayIcon.campaign:
        return Icons.campaign_outlined;
      case NotificationDisplayIcon.build:
        return Icons.build_outlined;
      case NotificationDisplayIcon.supportAgent:
        return Icons.support_agent_outlined;
      case NotificationDisplayIcon.warning:
        return Icons.warning_amber_outlined;
      case NotificationDisplayIcon.block:
        return Icons.block_outlined;
      case NotificationDisplayIcon.delete:
        return Icons.delete_outline;
      case NotificationDisplayIcon.gavel:
        return Icons.gavel_outlined;
    }
  }

  /// Map domain color enum to Flutter Color
  Color _mapColor(NotificationDisplayColor color) {
    switch (color) {
      case NotificationDisplayColor.green:
        return Colors.green[700]!;
      case NotificationDisplayColor.red:
        return Colors.red[700]!;
      case NotificationDisplayColor.orange:
        return Colors.orange[700]!;
      case NotificationDisplayColor.blue:
        return Colors.blue[700]!;
      case NotificationDisplayColor.pink:
        return Colors.pink[700]!;
      case NotificationDisplayColor.indigo:
        return Colors.indigo[700]!;
      case NotificationDisplayColor.deepOrange:
        return Colors.deepOrange[700]!;
      case NotificationDisplayColor.teal:
        return Colors.teal[700]!;
      case NotificationDisplayColor.cyan:
        return Colors.cyan[700]!;
      case NotificationDisplayColor.grey:
        return Colors.grey[700]!;
      case NotificationDisplayColor.purple:
        return Colors.purple[700]!;
    }
  }
}
