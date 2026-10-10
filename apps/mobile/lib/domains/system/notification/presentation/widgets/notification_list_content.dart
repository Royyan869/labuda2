/// Notification List Content
///
/// Renders the loaded notification list grouped by date, with pull-to-refresh
/// and the non-destructive refresh progress / inline refresh-error banner.
/// The page-level loading/error/empty states are owned by the screen; this
/// widget only renders a non-empty collection.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/presentation/widgets/notification_dismissible_item.dart';

import 'package:flutter/material.dart';

class NotificationListContent extends ConsumerWidget {
  final String userId;
  final List<NotificationEntity> notifications;
  final Function(NotificationEntity) onNotificationTap;

  /// True while a refresh is in flight with data already on screen.
  final bool isRefreshing;

  /// True when the last refresh failed while data is still on screen.
  final bool hasRefreshError;

  /// Canonical refresh operation (re-runs the list authority).
  final Future<void> Function() onRefresh;

  const NotificationListContent({
    super.key,
    required this.userId,
    required this.notifications,
    required this.onNotificationTap,
    required this.onRefresh,
    this.isRefreshing = false,
    this.hasRefreshError = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Group notifications by date
    final groupedNotifications = _groupNotificationsByDate(notifications);
    final showStatus = isRefreshing || hasRefreshError;
    final statusOffset = showStatus ? 1 : 0;

    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
        itemCount: groupedNotifications.length + statusOffset,
        itemBuilder: (context, index) {
          // Non-destructive refresh status: rows stay, progress/error on top.
          if (showStatus && index == 0) {
            return Column(
              children: [
                if (isRefreshing) const LinearProgressIndicator(minHeight: 2),
                if (hasRefreshError) _buildRefreshErrorBanner(context),
              ],
            );
          }
          final group = groupedNotifications[index - statusOffset];
          return _buildDateGroup(context, ref, group);
        },
      ),
    );
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy and a retry that re-runs the canonical refresh.
  Widget _buildRefreshErrorBanner(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(onPressed: onRefresh, child: Text(l10n.retryAction)),
          ],
        ),
      ),
    );
  }

  /// Group notifications by date
  List<_NotificationDateGroup> _groupNotificationsByDate(
    List<NotificationEntity> notifications,
  ) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final yesterday = today.subtract(const Duration(days: 1));

    final groups = <_NotificationDateGroup>[];

    for (final notification in notifications) {
      final notificationDate = DateTime(
        notification.createdAt.year,
        notification.createdAt.month,
        notification.createdAt.day,
      );

      String dateLabel;
      if (notificationDate == today) {
        dateLabel = 'Hari ini';
      } else if (notificationDate == yesterday) {
        dateLabel = 'Kemarin';
      } else {
        dateLabel =
            '${notificationDate.day}/${notificationDate.month}/${notificationDate.year}';
      }

      // Find existing group or create new one
      final existingGroup = groups.indexWhere((g) => g.dateLabel == dateLabel);
      if (existingGroup >= 0) {
        groups[existingGroup].notifications.add(notification);
      } else {
        groups.add(
          _NotificationDateGroup(
            dateLabel: dateLabel,
            notifications: [notification],
          ),
        );
      }
    }

    return groups;
  }

  /// Build a date group with header and notifications
  Widget _buildDateGroup(
    BuildContext context,
    WidgetRef ref,
    _NotificationDateGroup group,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Date header
        Padding(
          padding: const EdgeInsets.fromLTRB(
            AppMetrics.p16,
            AppMetrics.p16,
            AppMetrics.p16,
            AppMetrics.p8,
          ),
          child: Text(
            group.dateLabel,
            style: context.typeRoles.labelMicro.copyWith(
              fontWeight: FontWeight.w600,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
              letterSpacing: 0.5,
            ),
          ),
        ),
        // Notifications in this group
        ...group.notifications.asMap().entries.map((entry) {
          final index = entry.key;
          final notification = entry.value;
          final isLast = index == group.notifications.length - 1;

          return NotificationDismissibleItem(
            notification: notification,
            userId: userId,
            isLast: isLast,
            onTap: () => onNotificationTap(notification),
          );
        }),
      ],
    );
  }
}

/// Date group for notifications
class _NotificationDateGroup {
  final String dateLabel;
  final List<NotificationEntity> notifications;

  _NotificationDateGroup({
    required this.dateLabel,
    required this.notifications,
  });
}
