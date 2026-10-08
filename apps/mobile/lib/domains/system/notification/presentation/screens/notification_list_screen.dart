/// Notification List Screen
///
/// Displays the account's notifications with pull-to-refresh and filter tabs.
///
/// STATE COMPOSITION (owner-locked): the screen consumes the ONE canonical
/// list authority [notificationListProvider] and renders its AsyncValue with
/// the canonical foundation — LoadingIndicator (no data), PageErrorState
/// (initial failure), EmptyState (successful zero-result), and the list with
/// non-destructive refresh + inline refresh error. Filtering is
/// presentation-level over that same authority; there is no second collection
/// provider.
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';
import 'package:labuda/domains/system/notification/presentation/providers/navigation_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_filter_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_list_content.dart';
import 'package:labuda/shared/shared.dart';

// Flutter
import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

class NotificationListScreen extends ConsumerWidget {
  final String userId;

  const NotificationListScreen({super.key, required this.userId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // THE canonical notification list authority. Filtering is applied below to
    // its value — never through a second provider.
    final notificationsAsync = ref.watch(notificationListProvider(userId));
    final filterState = ref.watch(selectedFilterNotifierProvider);
    final counts = ref.watch(notificationCountsProvider(userId));
    final markAsRead = ref.read(markNotificationAsReadProvider);

    final all = notificationsAsync.value ?? const <NotificationEntity>[];
    final filtered = filterState.filter == NotificationFilter.all
        ? all
        : all.where((n) => filterState.filter.matches(n.type)).toList();

    final hasData = notificationsAsync.value != null;
    final isInitialLoading = notificationsAsync.isLoading && !hasData;
    final hasInitialError = notificationsAsync.hasError && !hasData;
    final isRefreshing =
        notificationsAsync.isLoading && hasData && !notificationsAsync.hasError;
    final hasRefreshError = notificationsAsync.hasError && hasData;

    return PopScope(
      canPop: true,
      child: Scaffold(
        appBar: _buildAppBar(context, ref, filterState.filter),
        // SAFE-AREA-36: the body content owns the LIVE system bottom inset
        // — /notifications is a FLAT top-level GoRoute (ProfileModule), so
        // no shell bar owns it, and the list's EXPLICIT vertical p12
        // padding bypasses the framework's automatic list-padding
        // consumption, so nothing else could claim the bottom. Top stays
        // with Scaffold.appBar (the body slot already drops the top
        // padding — no phantom top).
        body: SafeArea(
          child: Column(
            children: [
              _buildFilterTabs(context, ref, filterState.filter, counts),
              Expanded(
                child: _buildBody(
                  context,
                  ref,
                  filtered,
                  filterState.filter,
                  markAsRead: markAsRead,
                  isInitialLoading: isInitialLoading,
                  hasInitialError: hasInitialError,
                  isRefreshing: isRefreshing,
                  hasRefreshError: hasRefreshError,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Canonical reload: initial-load retry, pull-to-refresh, and the inline
  /// refresh-error retry all re-run the ONE canonical list operation.
  /// Existing notifications stay visible (the provider preserves its previous
  /// value on refresh); failure is never rethrown or rendered raw.
  Future<void> _reload(WidgetRef ref) async {
    try {
      await ref.refresh(notificationListProvider(userId).future).then((_) {});
    } catch (_) {
      // No-op: the failure remains observable via async.hasError with the
      // last-known-good collection preserved in async.value.
    }
  }

  Widget _buildBody(
    BuildContext context,
    WidgetRef ref,
    List<NotificationEntity> filtered,
    NotificationFilter selectedFilter, {
    required Future<void> Function(String) markAsRead,
    required bool isInitialLoading,
    required bool hasInitialError,
    required bool isRefreshing,
    required bool hasRefreshError,
  }) {
    if (isInitialLoading) {
      // First request with no data → LoadingIndicator. Never EmptyState.
      return const Center(child: LoadingIndicator());
    }

    if (hasInitialError) {
      // CANONICAL page-level load error (PageErrorState): controlled
      // localized copy only — the raw error never reaches the screen.
      return PageErrorState(onRetry: () => _reload(ref));
    }

    if (filtered.isEmpty) {
      // Successful zero-result (or filter matched none) → the ONE canonical
      // EmptyState. Loading/error never reach here.
      final (title, subtitle) = _emptyCopy(selectedFilter);
      return EmptyState(
        icon: selectedFilter.icon,
        title: title,
        subtitle: subtitle,
      );
    }

    return NotificationListContent(
      userId: userId,
      notifications: filtered,
      onNotificationTap: (notification) =>
          _handleNotificationTap(context, ref, notification, markAsRead),
      isRefreshing: isRefreshing,
      hasRefreshError: hasRefreshError,
      onRefresh: () => _reload(ref),
    );
  }

  /// Contextual empty copy per active filter (preserved business copy).
  (String, String) _emptyCopy(NotificationFilter filter) {
    switch (filter) {
      case NotificationFilter.all:
        return (
          'Belum ada notifikasi',
          'Anda akan menerima notifikasi untuk aktivitas penting di sini',
        );
      case NotificationFilter.order:
        return (
          'Belum ada notifikasi pesanan',
          'Notifikasi untuk pesanan Anda akan muncul di sini',
        );
      case NotificationFilter.dispute:
        return (
          'Belum ada notifikasi sengketa',
          'Notifikasi untuk sengketa dan banding akan muncul di sini',
        );
      case NotificationFilter.payout:
        return (
          'Belum ada notifikasi pembayaran',
          'Notifikasi untuk pembayaran dan penarikan akan muncul di sini',
        );
      case NotificationFilter.support:
        return (
          'Belum ada notifikasi bantuan',
          'Notifikasi untuk tiket bantuan dan keamanan akan muncul di sini',
        );
    }
  }

  /// Build app bar with filter indicator
  AppBar _buildAppBar(
    BuildContext context,
    WidgetRef ref,
    NotificationFilter filter,
  ) {
    return AppBar(
      leading: IconButton(
        icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
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
                  Icon(Icons.done_all, size: AppIconSize.action),
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
    // CONTENT-DRIVEN height, deliberately. This rail used to promise
    // `height: 50` around a horizontal ListView — and a horizontal list CLIPS
    // its cross axis in silence, without a single overflow stripe, so a chip
    // that grew with the type ladder would have been cut off unnoticed. The
    // filters are a CLOSED enum (nothing lazy to pay for), so a Row inside a
    // horizontal scroll view follows the chips instead of boxing them.
    Widget chip(NotificationFilter filter) {
      final isSelected = filter == selectedFilter;
      final count = counts[filter] ?? 0;
      return Padding(
        padding: const EdgeInsets.only(right: AppMetrics.p12),
        child: FilterChip(
          avatar: Icon(filter.icon, size: AppIconSize.action),
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
                        : Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ],
          ),
          selected: isSelected,
          onSelected: (_) {
            ref.read(selectedFilterNotifierProvider.notifier).setFilter(filter);
          },
          backgroundColor: Theme.of(
            context,
          ).colorScheme.surfaceContainerHighest,
          selectedColor: Theme.of(context).colorScheme.primaryContainer,
          checkmarkColor: Theme.of(context).colorScheme.primary,
        ),
      );
    }

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p8,
      ),
      child: Row(
        children: [
          for (final filter in NotificationFilter.values) chip(filter),
        ],
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
}
