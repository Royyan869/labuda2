/// Bidding Notifier
/// Riverpod Notifier for bidding state management
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show biddingRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/bidding_repository.dart';
import 'bidding_state.dart';

/// Bidding Notifier
///
/// Manages state for user's bidding activity.
class BiddingNotifier extends Notifier<BiddingState> {
  late BiddingRepository _biddingRepository;
  late ILoggerService _logger;

  @override
  BiddingState build() {
    _biddingRepository = ref.watch(biddingRepositoryProvider);
    _logger = ref.watch(loggerServiceProvider);
    return const BiddingInitial();
  }

  /// Load user's bidding activity
  Future<void> loadMyBidding() async {
    state = const BiddingLoading();

    final result = await _biddingRepository.getMyBidding();

    result.fold((data) => state = BiddingData(data), (error) {
      _logger.error('Failed to load bidding: $error');
      state = BiddingError(error);
    });
  }

  /// Refresh bidding data
  Future<void> refresh() async {
    await loadMyBidding();
  }

  /// Clear error
  void clearError() {
    if (state is BiddingError) {
      state = const BiddingInitial();
    }
  }
}

/// Bidding Notifier Provider
final biddingNotifierProvider = NotifierProvider<BiddingNotifier, BiddingState>(
  BiddingNotifier.new,
);
