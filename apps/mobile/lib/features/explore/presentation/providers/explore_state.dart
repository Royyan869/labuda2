import 'package:freezed_annotation/freezed_annotation.dart';

part 'explore_state.freezed.dart';

/// Explore state for tab navigation
///
/// Since explore is a presentation-only module (aggregates other features),
/// this state only manages tab index and loading states.
@freezed
class ExploreState with _$ExploreState {
  const factory ExploreState.initial() = ExploreInitialState;

  const factory ExploreState.tabChanged({required int tabIndex}) =
      ExploreTabChangedState;

  const factory ExploreState.error({required String message}) =
      ExploreErrorState;
}
