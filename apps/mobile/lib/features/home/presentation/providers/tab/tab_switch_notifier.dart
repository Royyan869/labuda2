import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'tab_switch_state.dart';

part 'tab_switch_notifier.g.dart';

/// Notifier untuk manage pending tab switch
/// Digunakan ketika create screen selesai dan perlu switch tab
@riverpod
class TabSwitchNotifier extends _$TabSwitchNotifier {
  @override
  TabSwitchState build() {
    return const TabSwitchState();
  }

  /// Set pending tab switch untuk explore dengan sub-tab tertentu
  void switchToExplore({required int subTab}) {
    state = state.copyWith(
      targetTab: 'explore',
      subTabIndex: subTab,
      isPending: true,
    );
  }

  /// Set pending tab switch untuk profile
  void switchToProfile() {
    state = state.copyWith(targetTab: 'profile', isPending: true);
  }

  /// Clear pending switch setelah diproses
  void clear() {
    state = const TabSwitchState();
  }
}
