/// Notification Dismissible Item
///
/// Wraps NotificationItemWidget with swipe-to-delete functionality.
/// Extracted from notification_list_screen for better modularity.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'notification_item_widget.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:hishumi/shared/shared.dart';

// Flutter
import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

class NotificationDismissibleItem extends ConsumerWidget {
  final NotificationEntity notification;
  final String userId;
  final VoidCallback onTap;
  final bool isLast;

  const NotificationDismissibleItem({
    super.key,
    required this.notification,
    required this.userId,
    required this.onTap,
    this.isLast = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Dismissible(
      key: Key(notification.id),
      direction: DismissDirection.endToStart,
      background: _buildDismissBackground(context),
      onDismissed: (direction) => _handleDismiss(context, ref),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          NotificationItemWidget(notification: notification, onTap: onTap),
          if (!isLast)
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: AppMetrics.p16),
              child: Divider(height: 1),
            ),
        ],
      ),
    );
  }

  Widget _buildDismissBackground(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      alignment: Alignment.centerRight,
      padding: const EdgeInsets.only(right: AppMetrics.p24),
      color: scheme.error,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          Text(
            'Delete',
            style: TextStyle(
              color: scheme.onPrimary,
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(width: 8),
          Icon(Icons.delete_outline, color: scheme.onPrimary, size: AppIconSize.header),
        ],
      ),
    );
  }

  Future<void> _handleDismiss(BuildContext context, WidgetRef ref) async {
    try {
      // The closure converges the unread-count badge and the list from the
      // backend's canonical post-mutation count on success; on failure it
      // throws and nothing changes.
      final deleteNotification = ref.read(deleteNotificationProvider);
      await deleteNotification(notification.id, userId);
    } catch (e) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Gagal menghapus. Coba lagi.');
      }
    }
  }
}
