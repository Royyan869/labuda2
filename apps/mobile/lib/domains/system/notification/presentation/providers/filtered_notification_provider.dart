/// Filtered Notification Provider
///
/// Provides filtered notifications based on selected filter tab.
/// Auto-updates from API (polling) and applies filter logic.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';

/// Selected filter provider (simple container)
class FilterState {
  const FilterState({this.filter = NotificationFilter.all});

  final NotificationFilter filter;

  FilterState copyWith({NotificationFilter? filter}) {
    return FilterState(filter: filter ?? this.filter);
  }
}

/// Selected filter state provider
final selectedFilterStateProvider = Provider<FilterState>((ref) {
  return const FilterState();
});

/// Selected filter notifier
class SelectedFilterNotifier extends Notifier<FilterState> {
  @override
  FilterState build() => const FilterState();

  void setFilter(NotificationFilter filter) {
    state = state.copyWith(filter: filter);
  }
}

/// Selected filter notifier provider
final selectedFilterNotifierProvider =
    NotifierProvider<SelectedFilterNotifier, FilterState>(() {
      return SelectedFilterNotifier();
    });

/// Filtered notifications provider
/// Combines base notifications with selected filter
final filteredNotificationsProvider =
    StreamProvider.family<List<NotificationEntity>, String>((ref, userId) {
      final filterState = ref.watch(selectedFilterNotifierProvider);
      final notificationsAsync = ref.watch(notificationListProvider(userId));

      return notificationsAsync.when(
        data: (notifications) {
          // Create a stream that emits filtered notifications
          return Stream.value(
            filterState.filter == NotificationFilter.all
                ? notifications
                : notifications
                      .where((n) => filterState.filter.matches(n.type))
                      .toList(),
          );
        },
        loading: () => Stream.value([]),
        error: (_, _) => Stream.value([]),
      );
    });

/// Notification counts by filter
final notificationCountsProvider =
    Provider.family<Map<NotificationFilter, int>, String>((ref, userId) {
      final notificationsAsync = ref.watch(notificationListProvider(userId));

      return notificationsAsync.when(
        data: (notifications) {
          final counts = <NotificationFilter, int>{};

          for (final filter in NotificationFilter.values) {
            if (filter == NotificationFilter.all) {
              counts[filter] = notifications.length;
            } else {
              counts[filter] = notifications
                  .where((n) => filter.matches(n.type))
                  .length;
            }
          }

          return counts;
        },
        loading: () => {},
        error: (_, _) => {},
      );
    });
