import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'explore_state.dart';

part 'explore_notifier.g.dart';

/// Explore notifier for tab navigation
///
/// This notifier handles tab switching and deep link navigation.
/// The actual content (listing, auction) is provided
/// by their respective feature modules.
@riverpod
class ExploreNotifier extends _$ExploreNotifier {
  @override
  ExploreState build() {
    return const ExploreState.initial();
  }

  /// Change to a specific tab
  void changeTab(int index) {
    state = ExploreState.tabChanged(tabIndex: index);
  }

  /// Reset to initial state
  void reset() {
    state = const ExploreState.initial();
  }
}
