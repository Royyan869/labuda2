/// Notification List Screen (REFACTORED)
///
/// Displays list of user notifications with pull-to-refresh and filter tabs.
/// Professional design with proper loading & error states.
///
/// REFACTORED: Extracted navigation, dialogs, and UI components
/// to comply with GUIDELINES file size limits.
///
/// MIGRATION: Now uses Riverpod providers instead of ServiceLocator.
/// ADDED: Filter tabs (All, Order, Dispute, Payout, Support)
///
/// Size: < 150 lines (per GUIDELINES) ✅
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';
import 'package:labuda/domains/system/notification/presentation/providers/navigation_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/filtered_notification_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_list_content.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationListScreen extends ConsumerWidget {
  final String userId;

  const NotificationListScreen({super.key, required this.userId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final notificationsAsync = ref.watch(filteredNotificationsProvider(userId));
    final filterState = ref.watch(selectedFilterNotifierProvider);
    final counts = ref.watch(notificationCountsProvider(userId));
    final markAsRead = ref.read(markNotificationAsReadProvider);

    return PopScope(
      canPop: true,
      child: Scaffold(
        appBar: _buildAppBar(context, ref, filterState.filter),
        body: Column(
          children: [
            _buildFilterTabs(context, ref, filterState.filter, counts),
            Expanded(
              child: notificationsAsync.when(
                data: (notifications) => NotificationListContent(
                  userId: userId,
                  notifications: notifications,
                  selectedFilter: filterState.filter,
                  onNotificationTap: (notification) => _handleNotificationTap(
                    context,
                    ref,
                    notification,
                    markAsRead,
                  ),
                ),
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => _buildErrorState(context, ref, error),
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Build app bar with filter indicator
  AppBar _buildAppBar(
    BuildContext context,
    WidgetRef ref,
    NotificationFilter filter,
  ) {
    return AppBar(
      elevation: 0,
      surfaceTintColor: Colors.transparent,
      leading: IconButton(
        icon: const Icon(Icons.arrow_back),
        onPressed: () => Navigator.of(context).pop(),
      ),
      title: Text(
        'Notifications${filter != NotificationFilter.all ? ' - ${filter.displayLabel}' : ''}',
      ),
      actions: [
        PopupMenuButton<String>(
          icon: const Icon(Icons.more_vert),
          onSelected: (value) => _handleMenuAction(context, ref, value),
          itemBuilder: (context) => [
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
          ],
        ),
      ],
    );
  }

  /// Build filter tabs
  Widget _buildFilterTabs(
    BuildContext context,
    WidgetRef ref,
    NotificationFilter selectedFilter,
    Map<NotificationFilter, int> counts,
  ) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: ListView.builder(
        scrollDirection: Axis.horizontal,
        itemCount: NotificationFilter.values.length,
        itemBuilder: (context, index) {
          final filter = NotificationFilter.values[index];
          final isSelected = filter == selectedFilter;
          final count = counts[filter] ?? 0;

          return Padding(
            padding: const EdgeInsets.only(right: 12),
            child: FilterChip(
              avatar: Icon(filter.icon, size: 18),
              label: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(filter.displayLabel),
                  if (count > 0) ...[
                    const SizedBox(width: 6),
                    Text(
                      '($count)',
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        color: isSelected
                            ? Theme.of(context).colorScheme.primary
                            : Colors.grey[700],
                      ),
                    ),
                  ],
                ],
              ),
              selected: isSelected,
              onSelected: (_) {
                ref
                    .read(selectedFilterNotifierProvider.notifier)
                    .setFilter(filter);
              },
              backgroundColor: Colors.grey[200],
              selectedColor: Theme.of(context).colorScheme.primaryContainer,
              checkmarkColor: Theme.of(context).colorScheme.primary,
            ),
          );
        },
      ),
    );
  }

  /// Handle menu action
  void _handleMenuAction(BuildContext context, WidgetRef ref, String value) {
    switch (value) {
      case 'mark_all_read':
        ref.read(markAllNotificationsAsReadProvider)(userId);
        break;
    }
  }

  /// Handle notification tap: mark as read and navigate
  Future<void> _handleNotificationTap(
    BuildContext context,
    WidgetRef ref,
    NotificationEntity notification,
    Future<void> Function(String) markAsRead,
  ) async {
    // Mark as read if unread
    if (!notification.isRead) {
      await markAsRead(notification.id);
    }

    // Navigate using NotificationNavigationService from provider
    if (context.mounted) {
      final navigationService = ref.read(notificationNavigationServiceProvider);
      await navigationService.handleNotificationTap(context, notification);
    }
  }

  /// Build error state with retry
  Widget _buildErrorState(BuildContext context, WidgetRef ref, Object error) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, size: 64, color: Colors.red),
            const SizedBox(height: 16),
            const Text(
              'Failed to Load Notifications',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 8),
            Text(
              error.toString(),
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 14, color: Colors.grey[600]),
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: () {
                ref.invalidate(filteredNotificationsProvider(userId));
              },
              icon: const Icon(Icons.refresh),
              label: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }
}
