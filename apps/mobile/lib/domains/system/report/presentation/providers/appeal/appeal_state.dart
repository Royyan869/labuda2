/// Appeal State
///
/// State management for appeal functionality.
library;

import 'package:labuda/domains/system/report/domain/entities/entities.dart';

/// Appeal Actions State
class AppealActionsState {
  final bool isLoading;
  final String? error;
  final Appeal? lastAppeal;

  const AppealActionsState({
    this.isLoading = false,
    this.error,
    this.lastAppeal,
  });

  AppealActionsState copyWith({
    bool? isLoading,
    String? error,
    Appeal? lastAppeal,
  }) {
    return AppealActionsState(
      isLoading: isLoading ?? this.isLoading,
      error: error,
      lastAppeal: lastAppeal ?? this.lastAppeal,
    );
  }
}

/// Appeal List State
class AppealListState {
  final List<Appeal> appeals;
  final bool isLoading;
  final String? error;

  const AppealListState({
    this.appeals = const [],
    this.isLoading = false,
    this.error,
  });

  AppealListState copyWith({
    List<Appeal>? appeals,
    bool? isLoading,
    String? error,
  }) {
    return AppealListState(
      appeals: appeals ?? this.appeals,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  int get pendingCount =>
      appeals.where((a) => a.status == AppealStatus.pending).length;

  int get approvedCount =>
      appeals.where((a) => a.status == AppealStatus.approved).length;

  int get rejectedCount =>
      appeals.where((a) => a.status == AppealStatus.rejected).length;
}
