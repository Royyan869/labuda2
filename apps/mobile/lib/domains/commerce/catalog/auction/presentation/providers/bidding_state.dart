/// Bidding State
/// Sealed classes for type-safe state management
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/entities/bidding_item.dart';

/// Base bidding state
sealed class BiddingState {
  const BiddingState();
}

/// Initial state
class BiddingInitial extends BiddingState {
  const BiddingInitial();
}

/// Loading state
class BiddingLoading extends BiddingState {
  const BiddingLoading();
}

/// Data loaded state
class BiddingData extends BiddingState {
  final BiddingResult result;
  const BiddingData(this.result);
}

/// Error state
class BiddingError extends BiddingState {
  final String error;
  const BiddingError(this.error);
}
