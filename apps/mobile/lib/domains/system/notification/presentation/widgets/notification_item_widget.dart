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
import 'package:labuda/core/core.dart' hide NotificationEntity;

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

    // Get display metadata from domain service
    final displayService = const NotificationDisplayService();
    final displayMetadata = displayService.getDisplayMetadata(
      notification.type,
    );

    return Material(
      color: notification.isRead
          ? theme.colorScheme.surface
          : theme.colorScheme.primaryContainer.withValues(alpha: 0.4),
      child: InkWell(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p12,
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Icon
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: _mapColor(
                    context,
                    displayMetadata.color,
                    theme.colorScheme,
                  ).withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(AppShape.r12),
                ),
                child: Icon(
                  _mapIcon(displayMetadata.icon),
                  color: _mapColor(
                    context,
                    displayMetadata.color,
                    theme.colorScheme,
                  ),
                  size: AppIconSize.header,
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
                          size: AppIconSize.inlineGlyph,
                          color: theme.colorScheme.onSurface.withValues(
                            alpha: 0.5,
                          ),
                        ),
                        const SizedBox(width: 4),
                        // Dynamic metadata receives the remaining width. It hugs
                        // when short and truncates to one line when the row is
                        // constrained, so a long relative timestamp can never
                        // overflow. Flexible (not Expanded) keeps the clock icon
                        // and the status badge intrinsic and ADJACENT — the
                        // badge must stay visible and must not be pushed away.
                        Flexible(
                          child: Text(
                            notification.timeAgo,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: theme.textTheme.bodySmall?.copyWith(
                              fontWeight: FontWeight.w500,
                              color: theme.colorScheme.onSurface.withValues(
                                alpha: 0.6,
                              ),
                            ),
                          ),
                        ),
                        if (notification.requiresAction) ...[
                          const SizedBox(width: 8),
                          // Status badge: bounded visual, but its LABEL is
                          // dynamic ("Perlu tindakan" is far longer than
                          // "BARU"), so it is flex-constrained too and
                          // truncates to one line as a last resort. It stays
                          // adjacent to the timestamp and is never dropped.
                          Flexible(
                            child: Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppMetrics.p8,
                                vertical: AppMetrics.p4,
                              ),
                              decoration: BoxDecoration(
                                color: theme.colorScheme.error.withValues(
                                  alpha: 0.12,
                                ),
                                borderRadius: BorderRadius.circular(
                                  AppShape.r4,
                                ),
                                border: Border.all(
                                  color: theme.colorScheme.error.withValues(
                                    alpha: 0.3,
                                  ),
                                  width: 1,
                                ),
                              ),
                              child: Text(
                                'Perlu tindakan',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: context.typeRoles.labelMicro.copyWith(
                                  fontWeight: FontWeight.w700,
                                  color: theme.colorScheme.error,
                                  letterSpacing: 0.3,
                                ),
                              ),
                            ),
                          ),
                        ],
                        if (notification.isRecent &&
                            !notification.requiresAction) ...[
                          const SizedBox(width: 8),
                          // Same rule as the action badge: dynamic label, so it
                          // is flex-constrained and truncates to one line only
                          // when the row is genuinely too narrow.
                          Flexible(
                            child: Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppMetrics.p8,
                                vertical: AppMetrics.p4,
                              ),
                              decoration: BoxDecoration(
                                color: context.statusColors.warning.withValues(
                                  alpha: 0.12,
                                ),
                                borderRadius: BorderRadius.circular(
                                  AppShape.r4,
                                ),
                              ),
                              child: Text(
                                'BARU',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: context.typeRoles.labelMicro.copyWith(
                                  fontWeight: FontWeight.w700,
                                  color: context.statusColors.warning,
                                  letterSpacing: 0.5,
                                ),
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
  Color _mapColor(
    BuildContext context,
    NotificationDisplayColor color,
    ColorScheme scheme,
  ) {
    switch (color) {
      case NotificationDisplayColor.green:
      case NotificationDisplayColor.teal:
        return context.statusColors.success;
      case NotificationDisplayColor.red:
      case NotificationDisplayColor.deepOrange:
        return scheme.error;
      case NotificationDisplayColor.orange:
        return context.statusColors.warning;
      case NotificationDisplayColor.blue:
      case NotificationDisplayColor.indigo:
      case NotificationDisplayColor.cyan:
        return scheme.secondary;
      case NotificationDisplayColor.pink:
      case NotificationDisplayColor.purple:
        return scheme.primary;
      case NotificationDisplayColor.grey:
        return scheme.onSurfaceVariant;
    }
  }
}
