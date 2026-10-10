/// Notification confirmation facade.
///
/// Owns the notification-specific SIDE-EFFECT behind each confirmation — the
/// provider call, the list invalidation and the result snackbar. The dialog
/// COMPOSITION is delegated to the canonical [AppDialog] authority; this file
/// must never build a raw `AlertDialog`/`showDialog` of its own (Slice #2 of
/// the dialog-authority convergence).
///
/// Responsibilities:
/// - Delete-all / delete-read confirmations (destructive intent) + action
/// - Mark all as read (no dialog)
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:hishumi/shared/shared.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationDialogHelper {
  NotificationDialogHelper._();

  /// Show the delete-all confirmation, then delete only if confirmed.
  static void showDeleteAllConfirmation(
    BuildContext context,
    WidgetRef ref,
    String userId,
  ) {
    AppDialog.confirm(
      context: context,
      title: 'Delete All Notifications',
      message:
          'All notifications will be permanently deleted. This action cannot be undone.',
      confirmLabel: 'Delete All',
      cancelLabel: 'Batal',
      intent: AppDialogIntent.destructive,
    ).then((confirmed) async {
      if (!confirmed) return;
      try {
        final deleteAll = ref.read(deleteAllNotificationsProvider);
        // The closure converges the unread-count badge and the list from the
        // backend's canonical post-deletion state on success.
        await deleteAll(userId);
        if (context.mounted) {
          AppSnackBar.showSuccess(context, 'Semua notifikasi dihapus');
        }
      } catch (e) {
        if (context.mounted) {
          AppSnackBar.showError(context, 'Gagal menghapus. Coba lagi.');
        }
      }
    });
  }

  /// Show the delete-read confirmation, then delete only if confirmed.
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

    AppDialog.confirm(
      context: context,
      title: 'Delete Read Notifications',
      message:
          'Delete $readCount read notifications? This action cannot be undone.',
      confirmLabel: 'Delete',
      cancelLabel: 'Batal',
      intent: AppDialogIntent.destructive,
    ).then((confirmed) async {
      if (!confirmed) return;
      try {
        final deleteRead = ref.read(deleteReadNotificationsProvider);
        // The closure converges the unread-count badge and the list from the
        // backend's canonical post-deletion state on success.
        await deleteRead(userId);
        if (context.mounted) {
          AppSnackBar.showSuccess(context, '$readCount notifikasi dihapus');
        }
      } catch (e) {
        if (context.mounted) {
          AppSnackBar.showError(context, 'Gagal menghapus. Coba lagi.');
        }
      }
    });
  }

  /// Mark all notifications as read (no dialog).
  static Future<void> markAllAsRead(
    BuildContext context,
    WidgetRef ref,
    String userId,
  ) async {
    try {
      final markAll = ref.read(markAllNotificationsAsReadProvider);
      await markAll(userId);
      if (context.mounted) {
        AppSnackBar.showSuccess(context, 'Semua notifikasi ditandai sudah dibaca');
      }
    } catch (e) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Gagal memperbarui. Coba lagi.');
      }
    }
  }
}
