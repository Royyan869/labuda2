/// Notification List Content
///
/// Main list content with pull-to-refresh and loading states.
/// Extracted from notification_list_screen for better modularity.
///
/// Size: < 150 lines (per GUIDELINES)
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_dismissible_item.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_empty_state_widget.dart';

import 'package:flutter/material.dart';

class NotificationListContent extends ConsumerWidget {
  final String userId;
  final List<NotificationEntity> notifications;
  final NotificationFilter selectedFilter;
  final Function(NotificationEntity) onNotificationTap;

  const NotificationListContent({
    super.key,
    required this.userId,
    required this.notifications,
    required this.selectedFilter,
    required this.onNotificationTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (notifications.isEmpty) {
      return NotificationEmptyStateWidget(filter: selectedFilter);
    }

    // Group notifications by date
    final groupedNotifications = _groupNotificationsByDate(notifications);

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(notificationListProvider(userId));
        await Future.delayed(const Duration(milliseconds: 500));
      },
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(vertical: 12),
        itemCount: groupedNotifications.length,
        itemBuilder: (context, index) {
          final group = groupedNotifications[index];
          return _buildDateGroup(context, ref, group);
        },
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
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Text(
            group.dateLabel,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: Colors.grey[600],
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
