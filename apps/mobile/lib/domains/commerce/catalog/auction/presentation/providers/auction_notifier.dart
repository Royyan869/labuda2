/// Auction Notifier
/// Riverpod Notifier that replaces UseCase classes
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider, auctionWatchRepositoryProvider;
import 'auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';

/// Auction Notifier
///
/// This notifier replaces the following UseCase classes:
/// - GetAuctionsUseCase → loadActiveAuctions, loadUserAuctions
/// - GetAuctionDetailUseCase → loadAuctionDetails
/// - PlaceBidUseCase → placeBid
/// - CreateAuctionUseCase → createAuction
/// - UpdateAuctionUseCase → updateAuction
/// - CancelAuctionUseCase → cancelAuction
/// - WatchAuctionUseCase → watchAuction, unwatchAuction
/// - GetWatchStatsUseCase → loadWatchStats
///
/// Uses Riverpod Notifier for state management
class AuctionNotifier extends Notifier<AuctionNotifierState> {
  late AuctionRepository _auctionRepository;
  late AuctionWatchRepository _watchRepository;
  late ILoggerService _logger;

  // Synchronous double-submit guards for financial operations
  bool _isPlacingBid = false;
  bool _isClaiming = false;

  @override
  AuctionNotifierState build() {
    // Dependencies will be injected via provider override
    // This is a placeholder - actual injection happens in provider
    _auctionRepository = ref.watch(auctionRepositoryProvider);
    _watchRepository = ref.watch(auctionWatchRepositoryProvider);
    _logger = ref.watch(loggerServiceProvider);

    return const AuctionNotifierState();
  }

  // ========== Auction List Operations ==========

  /// Load active auctions with filters
  Future<void> loadActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);

    final result = await _auctionRepository.getActiveAuctions(
      variety: variety,
      minSize: minSize,
      maxSize: maxSize,
      maxBid: maxBid,
      limit: limit,
    );

    result.fold(
      (auctions) => state = state.copyWith(
        auctions: auctions,
        isLoading: false,
        error: null,
      ),
      (error) => state = state.copyWith(isLoading: false, error: error),
    );
  }

  /// Load user's auctions (seller dashboard)
  Future<void> loadUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);

    final result = await _auctionRepository.getUserAuctions(
      sellerId: sellerId,
      status: status,
      limit: limit,
    );

    result.fold(
      (auctions) => state = state.copyWith(
        auctions: auctions,
        isLoading: false,
        error: null,
      ),
      (error) => state = state.copyWith(isLoading: false, error: error),
    );
  }

  // ========== Auction Detail Operations ==========

  /// Load auction details
  Future<void> loadAuctionDetails(String auctionId) async {
    state = state.copyWith(isLoading: true, clearError: true);

    final result = await _auctionRepository.getAuctionById(auctionId);

    result.fold((auction) {
      state = state.copyWith(
        selectedAuction: auction,
        isLoading: false,
        error: null,
      );

      // Note: View tracking is handled by backend automatically via GET endpoint
    }, (error) => state = state.copyWith(isLoading: false, error: error));
  }

  /// Load auction bids
  Future<void> loadAuctionBids(String auctionId, {int limit = 50}) async {
    final result = await _auctionRepository.getAuctionBids(
      auctionId: auctionId,
      limit: limit,
    );

    result.fold(
      (bids) => state = state.copyWith(bids: bids),
      (error) => state = state.copyWith(error: error),
    );
  }

  // ========== Bidding Operations ==========

  /// Place bid on auction
  ///
  /// BACKEND AUTHORITY: The following validations use DERIVED PRESENTATION STATE
  /// for UX optimization (fail fast). The BACKEND is the FINAL AUTHORITY for all
  /// business decisions. These client-side checks are purely for better UX.
  ///
  /// FUTURE: Use auction.decision.allowed_actions.contains('bid') and
  /// auction.decision.display.minimumNextBid from backend decision contract.
  ///
  /// Business Rules:
  /// - Auction must be active
  /// - Auction must not have ended
  /// - Bid amount must be >= minimum bid
  /// - User cannot bid on own auction
  Future<bool> placeBid({
    required String auctionId,
    required String bidderId,
    required double amount,
  }) async {
    // Synchronous guard - prevent double-tap
    if (_isPlacingBid) return false;
    _isPlacingBid = true;

    try {
      state = state.copyWith(isPlacingBid: true, clearError: true);

      // Get auction first for validation
      final auctionResult = await _auctionRepository.getAuctionById(auctionId);

      return auctionResult.fold(
        (auction) async {
          // BOUNDARY NORMALIZATION (PHASE 1D):
          // UX pre-validation checks - backend is final authority
          // These provide better UX by failing fast before API call
          // The backend will enforce all business rules definitively

          // Validate auction is active and can accept bids
          if (!auction.isActive) {
            final errorMsg = auction.status == AuctionStatus.ended
                ? 'Lelang sudah berakhir'
                : auction.status == AuctionStatus.scheduled
                ? 'Lelang belum dimulai'
                : 'Lelang tidak aktif';
            state = state.copyWith(isPlacingBid: false, error: errorMsg);
            return false;
          }

          // LOCAL COMPUTATION: UX optimization - backend is final authority
          // FUTURE: Use auction.decision.display.minimumNextBid from backend
          // Validate bid amount
          final minimumBid = auction.currentBid + auction.bidIncrement;
          if (amount < minimumBid) {
            state = state.copyWith(
              isPlacingBid: false,
              error: 'Bid minimum: Rp ${minimumBid.toStringAsFixed(0)}',
            );
            return false;
          }

          // Validate user is not seller
          if (auction.sellerId == bidderId) {
            state = state.copyWith(
              isPlacingBid: false,
              error: 'Tidak dapat bid pada lelang sendiri',
            );
            return false;
          }

          // Log high-value bids (MVP: trust-based without escrow)
          if (amount >= 500000) {
            _logger.warning(
              'High-value bid without escrow (MVP mode)',
              extra: {
                'auctionId': auctionId,
                'bidderId': bidderId,
                'bidAmount': amount,
              },
            );
          }

          // Place bid
          final result = await _auctionRepository.placeBid(
            auctionId: auctionId,
            bidderId: bidderId,
            amount: amount,
          );

          if (result.isError) {
            state = state.copyWith(
              isPlacingBid: false,
              error: result.error,
              errorCode: result.errorCode,
              errorDetails: result.errorDetails,
            );
            return false;
          }
          state = state.copyWith(
            isPlacingBid: false,
            error: null,
            successMessage: 'Bid berhasil ditempatkan',
          );
          // Reload auction details
          loadAuctionDetails(auctionId);
          loadAuctionBids(auctionId);
          return true;
        },
        (error) {
          state = state.copyWith(isPlacingBid: false, error: error);
          return false;
        },
      );
    } finally {
      // Always reset guard in finally
      _isPlacingBid = false;
    }
  }

  /// Claim auction - creates order for auction winner
  ///
  /// SINGLE SOURCE OF TRUTH: This is the ONLY way to create orders from won auctions.
  /// The backend validates:
  /// - Caller is the winner
  /// - Auction is in waiting_settlement status
  /// - Claim deadline has not passed
  /// - Creates order atomically with order_id set on auction
  ///
  /// Returns order_id on success, null on failure
  Future<String?> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingSetupId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    // Synchronous guard - prevent double-tap
    if (_isClaiming) return null;
    _isClaiming = true;

    try {
      state = state.copyWith(isLoading: true, clearError: true);

      // Execute claim
      final result = await _auctionRepository.claimAuction(
        auctionId: auctionId,
        addressId: addressId,
        shippingSetupId: shippingSetupId,
        discountCode: discountCode,
        useCoins: useCoins,
      );

      return result.fold(
        (orderId) {
          state = state.copyWith(
            isLoading: false,
            error: null,
            successMessage: 'Klaim berhasil! Pesanan telah dibuat',
          );

          // Reload auction details to get updated order_id
          loadAuctionDetails(auctionId);

          return orderId;
        },
        (error) {
          state = state.copyWith(isLoading: false, error: error);
          return null;
        },
      );
    } finally {
      // Always reset guard in finally
      _isClaiming = false;
    }
  }

  // ========== Auction CRUD Operations ==========

  /// Create new auction. A Product is created inline by the backend from
  /// the item fields below — there is no productId/listingId parameter.
  Future<bool> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required double openingBid,
    required double bidIncrement,
    double? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,
    required List<String> shippingSetupIds,
    String? preparationNote,
  }) async {
    state = state.copyWith(isCreating: true, clearError: true);

    final result = await _auctionRepository.createAuction(
      sellerId: sellerId,
      sellerUsername: sellerUsername,
      sellerFarmName: sellerFarmName,
      sellerAvatar: sellerAvatar,
      title: title,
      description: description,
      mediaUrls: mediaUrls,
      mediaTypes: mediaTypes,
      koiDetails: koiDetails,
      openingBid: openingBid,
      bidIncrement: bidIncrement,
      buyNowPrice: buyNowPrice,
      startMode: startMode,
      scheduledStartAt: scheduledStartAt,
      durationHours: durationHours,
      farmAddressId: farmAddressId,
      location: location,
      shippingSetupIds: shippingSetupIds,
      preparationNote: preparationNote,
    );

    return result.fold(
      (auction) {
        state = state.copyWith(
          isCreating: false,
          error: null,
          successMessage: 'Lelang berhasil dibuat',
          selectedAuction: auction,
        );
        return true;
      },
      (error) {
        state = state.copyWith(isCreating: false, error: error);
        return false;
      },
    );
  }

  /// Update auction
  Future<bool> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    state = state.copyWith(isUpdating: true, clearError: true);

    final result = await _auctionRepository.updateAuction(auctionId, updates);

    return result.fold(
      (auction) {
        state = state.copyWith(
          isUpdating: false,
          error: null,
          successMessage: 'Lelang berhasil diupdate',
          selectedAuction: auction,
        );
        return true;
      },
      (error) {
        state = state.copyWith(isUpdating: false, error: error);
        return false;
      },
    );
  }

  /// Cancel auction
  Future<bool> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);

    final result = await _auctionRepository.cancelAuction(
      auctionId: auctionId,
      sellerId: sellerId,
      reason: reason,
    );

    return result.fold(
      (_) {
        state = state.copyWith(
          isLoading: false,
          error: null,
          successMessage: 'Lelang berhasil dibatalkan',
        );
        // Reload auction to get updated status
        loadAuctionDetails(auctionId);
        return true;
      },
      (error) {
        state = state.copyWith(isLoading: false, error: error);
        return false;
      },
    );
  }

  // ========== Watch Operations ==========

  /// Watch an auction
  Future<bool> watchAuction({
    required String auctionId,
    required String userId,
    bool notifyOnBid = true,
    bool notifyOnEndingSoon = true,
    bool notifyOnEnded = true,
  }) async {
    final result = await _watchRepository.watchAuction(
      auctionId: auctionId,
      userId: userId,
      notifyOnBid: notifyOnBid,
      notifyOnEndingSoon: notifyOnEndingSoon,
      notifyOnEnded: notifyOnEnded,
    );

    return result.fold(
      (_) {
        state = state.copyWith(
          successMessage: 'Lelang ditambahkan ke watchlist',
        );
        return true;
      },
      (error) {
        state = state.copyWith(error: error);
        return false;
      },
    );
  }

  /// Unwatch an auction
  Future<bool> unwatchAuction({
    required String auctionId,
    required String userId,
  }) async {
    final result = await _watchRepository.unwatchAuction(
      auctionId: auctionId,
      userId: userId,
    );

    return result.fold(
      (_) {
        state = state.copyWith(successMessage: 'Lelang dihapus dari watchlist');
        return true;
      },
      (error) {
        state = state.copyWith(error: error);
        return false;
      },
    );
  }

  /// Toggle watch status
  Future<bool> toggleWatch({
    required String auctionId,
    required String userId,
  }) async {
    final result = await _watchRepository.toggleWatch(
      auctionId: auctionId,
      userId: userId,
    );

    return result.fold(
      (isWatching) {
        state = state.copyWith(
          successMessage: isWatching
              ? 'Lelang ditambahkan ke watchlist'
              : 'Lelang dihapus dari watchlist',
        );
        return isWatching;
      },
      (error) {
        state = state.copyWith(error: error);
        return false;
      },
    );
  }

  /// Load watch stats
  Future<void> loadWatchStats({
    required String auctionId,
    required String currentUserId,
  }) async {
    final result = await _watchRepository.getWatchStats(
      auctionId: auctionId,
      currentUserId: currentUserId,
    );

    result.fold(
      (stats) => state = state.copyWith(watchStats: stats),
      (error) => state = state.copyWith(error: error),
    );
  }

  // ========== Utility Methods ==========

  /// Clear error
  void clearError() {
    state = state.copyWith(clearError: true);
  }

  /// Clear success message
  void clearSuccess() {
    state = state.copyWith(clearSuccess: true);
  }

  /// Reset state
  void reset() {
    state = const AuctionNotifierState();
  }
}

// ==============================================================================
// R4.1B DI MIGRATION: Import canonical providers from data layer
// ==============================================================================
// The following providers are now imported from data layer and core layer:
// - auctionRepositoryProvider (from auction/data/auction_providers.dart)
// - auctionWatchRepositoryProvider (from auction/data/auction_providers.dart)
// - loggerServiceProvider (from core/providers/core_providers.dart)
//
// Previous implementation used sl<>() which violates canonical DI path.
// These providers are now canonical - imported, not redefined here.
// ==============================================================================

/// Auction Notifier Provider
final auctionNotifierProvider =
    NotifierProvider<AuctionNotifier, AuctionNotifierState>(
      AuctionNotifier.new,
    );

// ========== Stream Providers (for UI) ==========

/// Stream provider for active auctions (explore tab)
final exploreAuctionsStreamProvider = StreamProvider<List<Auction>>((ref) {
  final repository = ref.watch(auctionRepositoryProvider);

  return repository.watchActiveAuctions(limit: 50).map((auctions) {
    final now = DateTime.now();

    // Filter out auctions that have ended
    final filtered = auctions
        .where((auction) => auction.endTime.isAfter(now))
        .toList();

    // Sort by endTime (ending soon first)
    filtered.sort((a, b) => a.endTime.compareTo(b.endTime));

    return filtered;
  });
});

/// Stream provider for user auctions (seller dashboard)
final userAuctionsStreamProvider = StreamProvider.family<List<Auction>, String>(
  (ref, sellerId) {
    final repository = ref.watch(auctionRepositoryProvider);

    return repository.watchUserAuctions(sellerId: sellerId, limit: 100);
  },
);

/// Stream provider for user auctions with status filter
/// Requires sellerId to be provided via a separate provider
final myAuctionsStreamProvider =
    StreamProvider.family<
      List<Auction>,
      ({String sellerId, AuctionStatus? status})
    >((ref, params) {
      final repository = ref.watch(auctionRepositoryProvider);

      return repository.watchUserAuctions(
        sellerId: params.sellerId,
        status: params.status,
        limit: 100,
      );
    });

/// Stream provider for auction detail (real-time updates)
final auctionStreamProvider = StreamProvider.family<Auction?, String>((
  ref,
  auctionId,
) {
  final repository = ref.watch(auctionRepositoryProvider);
  return repository.watchAuction(auctionId);
});

/// Stream provider for auction bids (real-time updates)
final auctionBidsStreamProvider =
    StreamProvider.family<List<AuctionBid>, String>((ref, auctionId) {
      final repository = ref.watch(auctionRepositoryProvider);
      return repository.watchAuctionBids(auctionId, limit: 50);
    });

/// Stream provider for watch stats (real-time updates)
/// Requires currentUserId to be provided
final watchStatsStreamProvider =
    StreamProvider.family<
      AuctionWatchStats,
      ({String auctionId, String currentUserId})
    >((ref, params) {
      final watchRepository = ref.watch(auctionWatchRepositoryProvider);

      return watchRepository.watchWatchStats(
        auctionId: params.auctionId,
        currentUserId: params.currentUserId,
      );
    });

// ========== Future Providers (for single fetch) ==========

/// Future provider for auction detail
final auctionDetailProvider = FutureProvider.family<Auction?, String>((
  ref,
  auctionId,
) async {
  ref.keepAlive(); // Keep alive to avoid refetching
  final repository = ref.watch(auctionRepositoryProvider);
  final result = await repository.getAuctionById(auctionId);

  return result.fold((auction) => auction, (error) => throw Exception(error));
});

/// Future provider for auction bids
final auctionBidsProvider = FutureProvider.family<List<AuctionBid>, String>((
  ref,
  auctionId,
) async {
  final repository = ref.watch(auctionRepositoryProvider);
  final result = await repository.getAuctionBids(auctionId: auctionId);

  return result.fold((bids) => bids, (error) => throw Exception(error));
});

/// Future provider for watched auctions
final watchedAuctionsProvider = FutureProvider.family<List<Auction>, String>((
  ref,
  userId,
) async {
  final watchRepository = ref.watch(auctionWatchRepositoryProvider);
  final result = await watchRepository.getWatchedAuctions(userId: userId);

  return result.fold((auctions) => auctions, (error) => throw Exception(error));
});
