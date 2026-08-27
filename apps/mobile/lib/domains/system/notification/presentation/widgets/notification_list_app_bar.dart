/// Notification List App Bar
///
/// App bar with menu options for notification management.
/// Extracted from notification_list_screen for better modularity.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/presentation/helpers/notification_dialog_helper.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationListAppBar extends ConsumerWidget
    implements PreferredSizeWidget {
  final String userId;
  final List<NotificationEntity> notifications;

  const NotificationListAppBar({
    super.key,
    required this.userId,
    required this.notifications,
  });

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final hasUnread = notifications.any((n) => !n.isRead);
    final hasRead = notifications.any((n) => n.isRead);
    final isEmpty = notifications.isEmpty;

    return AppBar(
      elevation: 0,
      surfaceTintColor: Colors.transparent,
      scrolledUnderElevation: 0,
      leading: IconButton(
        icon: const Icon(Icons.arrow_back),
        onPressed: () => Navigator.of(context).pop(),
      ),
      title: const Text(
        'Notifications',
        style: TextStyle(
          fontSize: 20,
          fontWeight: FontWeight.w600,
          letterSpacing: -0.5,
        ),
      ),
      actions: [
        if (!isEmpty)
          PopupMenuButton<String>(
            icon: const Icon(Icons.more_vert),
            onSelected: (value) => _handleMenuAction(context, ref, value),
            itemBuilder: (context) => [
              if (hasUnread)
                const PopupMenuItem(
                  value: 'mark_all_read',
                  child: Row(
                    children: [
                      Icon(Icons.done_all, size: 20),
                      SizedBox(width: 12),
                      Text('Mark All as Read'),
                    ],
                  ),
                ),
              if (hasRead)
                const PopupMenuItem(
                  value: 'delete_read',
                  child: Row(
                    children: [
                      Icon(Icons.delete_sweep, size: 20),
                      SizedBox(width: 12),
                      Text('Delete Read'),
                    ],
                  ),
                ),
              const PopupMenuItem(
                value: 'delete_all',
                child: Row(
                  children: [
                    Icon(Icons.delete_forever, size: 20, color: Colors.red),
                    SizedBox(width: 12),
                    Text('Delete All', style: TextStyle(color: Colors.red)),
                  ],
                ),
              ),
            ],
          ),
      ],
    );
  }

  void _handleMenuAction(BuildContext context, WidgetRef ref, String value) {
    switch (value) {
      case 'mark_all_read':
        NotificationDialogHelper.markAllAsRead(context, ref, userId);
        break;
      case 'delete_read':
        NotificationDialogHelper.showDeleteReadConfirmation(
          context,
          ref,
          userId,
          notifications,
        );
        break;
      case 'delete_all':
        NotificationDialogHelper.showDeleteAllConfirmation(
          context,
          ref,
          userId,
        );
        break;
    }
  }
}
