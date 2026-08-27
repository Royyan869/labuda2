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
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/shared/shared.dart';

// Flutter
import 'package:flutter/material.dart';

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
      background: _buildDismissBackground(),
      onDismissed: (direction) => _handleDismiss(context, ref),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          NotificationItemWidget(notification: notification, onTap: onTap),
          if (!isLast)
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 16),
              child: Divider(height: 1),
            ),
        ],
      ),
    );
  }

  Widget _buildDismissBackground() {
    return Container(
      alignment: Alignment.centerRight,
      padding: const EdgeInsets.only(right: 20),
      color: AppColors.primaryRed,
      child: const Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          Text(
            'Delete',
            style: TextStyle(color: Colors.white, fontWeight: FontWeight.w500),
          ),
          SizedBox(width: 8),
          Icon(Icons.delete_outline, color: Colors.white, size: 24),
        ],
      ),
    );
  }

  Future<void> _handleDismiss(BuildContext context, WidgetRef ref) async {
    try {
      final deleteNotification = ref.read(deleteNotificationProvider);
      await deleteNotification(notification.id);
      // Refresh list after successful deletion
      ref.invalidate(notificationListProvider(userId));
    } catch (e) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Gagal menghapus. Coba lagi.');
      }
    }
  }
}
