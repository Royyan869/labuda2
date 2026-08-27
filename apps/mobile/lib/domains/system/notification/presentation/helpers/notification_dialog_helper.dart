/// Notification Dialog Helper
///
/// Handles all dialog and modal presentations for notifications.
/// Extracted from notification_list_screen to comply with file size limits.
///
/// Responsibilities:
/// - Delete confirmation dialogs
/// - Mark all as read confirmation
/// - Error dialogs
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/shared/shared.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationDialogHelper {
  /// Show delete all notifications confirmation
  static void showDeleteAllConfirmation(
    BuildContext context,
    WidgetRef ref,
    String userId,
  ) {
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Delete All Notifications'),
        content: const Text(
          'All notifications will be permanently deleted. This action cannot be undone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(dialogContext);
              try {
                final deleteAll = ref.read(deleteAllNotificationsProvider);
                await deleteAll(userId);
                ref.invalidate(notificationListProvider(userId));
                if (context.mounted) {
                  AppSnackBar.showSuccess(context, 'All notifications deleted');
                }
              } catch (e) {
                if (context.mounted) {
                  AppSnackBar.showError(context, 'Gagal menghapus. Coba lagi.');
                }
              }
            },
            style: TextButton.styleFrom(foregroundColor: Colors.red),
            child: const Text('Delete All'),
          ),
        ],
      ),
    );
  }

  /// Show delete read notifications confirmation
  static void showDeleteReadConfirmation(
    BuildContext context,
    WidgetRef ref,
    String userId,
    List<NotificationEntity> notifications,
  ) {
    final readCount = notifications.where((n) => n.isRead).length;

    if (readCount == 0) {
      AppSnackBar.showInfo(context, 'No read notifications to delete');
      return;
    }

    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Delete Read Notifications'),
        content: Text(
          'Delete $readCount read notifications? This action cannot be undone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(dialogContext);
              try {
                final deleteRead = ref.read(deleteReadNotificationsProvider);
                await deleteRead(userId);
                ref.invalidate(notificationListProvider(userId));
                if (context.mounted) {
                  AppSnackBar.showSuccess(
                    context,
                    '$readCount notifications deleted',
                  );
                }
              } catch (e) {
                if (context.mounted) {
                  AppSnackBar.showError(context, 'Gagal menghapus. Coba lagi.');
                }
              }
            },
            style: TextButton.styleFrom(foregroundColor: Colors.red),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
  }

  /// Show mark all as read confirmation (optional, can be direct action)
  static Future<void> markAllAsRead(
    BuildContext context,
    WidgetRef ref,
    String userId,
  ) async {
    try {
      final markAll = ref.read(markAllNotificationsAsReadProvider);
      await markAll(userId);
      if (context.mounted) {
        AppSnackBar.showSuccess(context, 'All notifications marked as read');
      }
    } catch (e) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Gagal memperbarui. Coba lagi.');
      }
    }
  }

  /// Show error dialog with retry option
  static void showErrorDialog(
    BuildContext context, {
    required String message,
    VoidCallback? onRetry,
  }) {
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.error_outline, color: Colors.red[700]),
            const SizedBox(width: 8),
            const Text('Error'),
          ],
        ),
        content: Text(message),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Close'),
          ),
          if (onRetry != null)
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                onRetry();
              },
              child: const Text('Retry'),
            ),
        ],
      ),
    );
  }
}
