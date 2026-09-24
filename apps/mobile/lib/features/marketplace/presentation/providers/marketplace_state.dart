import 'package:freezed_annotation/freezed_annotation.dart';

part 'marketplace_state.freezed.dart';

@freezed
class MarketplaceState with _$MarketplaceState {
  const factory MarketplaceState.initial() = MarketplaceInitialState;

  const factory MarketplaceState.tabChanged({required int tabIndex}) =
      MarketplaceTabChangedState;

  const factory MarketplaceState.error({required String message}) =
      MarketplaceErrorState;
}
