/// Auction State
/// Sealed classes for type-safe state management
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';

/// Base auction state
sealed class AuctionState {
  const AuctionState();
}

/// Initial state
class AuctionInitial extends AuctionState {
  const AuctionInitial();
}

/// Loading state
class AuctionLoading extends AuctionState {
  const AuctionLoading();
}

/// Data loaded state
class AuctionData extends AuctionState {
  final Auction auction;
  const AuctionData(this.auction);
}

/// Auction list loaded state
class AuctionListData extends AuctionState {
  final List<Auction> auctions;
  const AuctionListData(this.auctions);
}

/// Bids loaded state
class AuctionBidsData extends AuctionState {
  final List<AuctionBid> bids;
  const AuctionBidsData(this.bids);
}

/// Watch stats loaded state
class AuctionWatchStatsData extends AuctionState {
  final AuctionWatchStats stats;
  const AuctionWatchStatsData(this.stats);
}

/// Action success state
class AuctionSuccess extends AuctionState {
  final String? message;
  const AuctionSuccess([this.message]);
}

/// Error state
class AuctionError extends AuctionState {
  final String error;
  const AuctionError(this.error);
}

/// Auction Notifier State (combined)
///
/// This is the main state class used by AuctionNotifier
/// It contains all possible states for auction operations
class AuctionNotifierState {
  final List<Auction> auctions;
  final Auction? selectedAuction;
  final List<AuctionBid> bids;
  final AuctionWatchStats? watchStats;
  final bool isLoading;
  final bool isPlacingBid;
  final bool isCreating;
  final bool isUpdating;
  final String? error;

  /// Machine-readable code for [error], when the failure came from a known
  /// API contract (e.g. `EMAIL_VERIFICATION_REQUIRED`). Null when the error
  /// is transport-level or untagged.
  final String? errorCode;

  /// Structured details from the API error response (e.g.
  /// `{"permanent_ban": true, "active_strikes": 4}`). Null when no details
  /// were provided or the error is transport-level.
  final Map<String, dynamic>? errorDetails;
  final String? successMessage;

  const AuctionNotifierState({
    this.auctions = const [],
    this.selectedAuction,
    this.bids = const [],
    this.watchStats,
    this.isLoading = false,
    this.isPlacingBid = false,
    this.isCreating = false,
    this.isUpdating = false,
    this.error,
    this.errorCode,
    this.errorDetails,
    this.successMessage,
  });

  AuctionNotifierState copyWith({
    List<Auction>? auctions,
    Auction? selectedAuction,
    List<AuctionBid>? bids,
    AuctionWatchStats? watchStats,
    bool? isLoading,
    bool? isPlacingBid,
    bool? isCreating,
    bool? isUpdating,
    String? error,
    String? errorCode,
    Map<String, dynamic>? errorDetails,
    String? successMessage,
    bool clearError = false,
    bool clearSuccess = false,
  }) {
    return AuctionNotifierState(
      auctions: auctions ?? this.auctions,
      selectedAuction: selectedAuction ?? this.selectedAuction,
      bids: bids ?? this.bids,
      watchStats: watchStats ?? this.watchStats,
      isLoading: isLoading ?? this.isLoading,
      isPlacingBid: isPlacingBid ?? this.isPlacingBid,
      isCreating: isCreating ?? this.isCreating,
      isUpdating: isUpdating ?? this.isUpdating,
      error: clearError ? null : (error ?? this.error),
      errorCode: clearError ? null : (errorCode ?? this.errorCode),
      errorDetails: clearError ? null : (errorDetails ?? this.errorDetails),
      successMessage: clearSuccess
          ? null
          : (successMessage ?? this.successMessage),
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AuctionNotifierState &&
        other.auctions == auctions &&
        other.selectedAuction == selectedAuction &&
        other.bids == bids &&
        other.watchStats == watchStats &&
        other.isLoading == isLoading &&
        other.isPlacingBid == isPlacingBid &&
        other.isCreating == isCreating &&
        other.isUpdating == isUpdating &&
        other.error == error &&
        other.errorCode == errorCode &&
        other.errorDetails == errorDetails &&
        other.successMessage == successMessage;
  }

  @override
  int get hashCode =>
      auctions.hashCode ^
      selectedAuction.hashCode ^
      bids.hashCode ^
      watchStats.hashCode ^
      isLoading.hashCode ^
      isPlacingBid.hashCode ^
      isCreating.hashCode ^
      isUpdating.hashCode ^
      error.hashCode ^
      errorCode.hashCode ^
      errorDetails.hashCode ^
      successMessage.hashCode;
}
