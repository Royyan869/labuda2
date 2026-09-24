import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'marketplace_state.dart';

part 'marketplace_notifier.g.dart';

@riverpod
class MarketplaceNotifier extends _$MarketplaceNotifier {
  @override
  MarketplaceState build() {
    return const MarketplaceState.initial();
  }

  void changeTab(int index) {
    state = MarketplaceState.tabChanged(tabIndex: index);
  }

  void reset() {
    state = const MarketplaceState.initial();
  }
}
