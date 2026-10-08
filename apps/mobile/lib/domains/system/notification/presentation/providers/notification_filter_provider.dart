/// Notification Filter Providers
///
/// Owns the selected-filter authority and the derived per-filter counts.
/// The notification collection itself is owned by [notificationListProvider]
/// (the ONE canonical read authority) — filtering is presentation-level and is
/// applied by the screen to that authority's value. There is no second
/// collection provider.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';

/// Selected filter state (simple immutable container).
class FilterState {
  const FilterState({this.filter = NotificationFilter.all});

  final NotificationFilter filter;

  FilterState copyWith({NotificationFilter? filter}) {
    return FilterState(filter: filter ?? this.filter);
  }
}

/// Selected filter notifier — the single filter-selection authority.
class SelectedFilterNotifier extends Notifier<FilterState> {
  @override
  FilterState build() => const FilterState();

  void setFilter(NotificationFilter filter) {
    state = state.copyWith(filter: filter);
  }
}

/// Selected filter notifier provider.
final selectedFilterNotifierProvider =
    NotifierProvider<SelectedFilterNotifier, FilterState>(() {
      return SelectedFilterNotifier();
    });

/// Derived per-filter counts for the filter chips.
///
/// Legitimate derived information computed from the canonical list authority's
/// value. When there is no loaded value (loading or error) it returns no
/// counts — it never fabricates `0` for a failed request, and the chips only
/// render a count when it is greater than zero.
final notificationCountsProvider =
    Provider.family<Map<NotificationFilter, int>, String>((ref, userId) {
      final notifications = ref.watch(notificationListProvider(userId)).value;
      if (notifications == null) return const {};

      final counts = <NotificationFilter, int>{};
      for (final filter in NotificationFilter.values) {
        counts[filter] = filter == NotificationFilter.all
            ? notifications.length
            : notifications.where((n) => filter.matches(n.type)).length;
      }
      return counts;
    });
