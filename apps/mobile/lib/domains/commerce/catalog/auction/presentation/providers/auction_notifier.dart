/// Auction Notifier
/// Riverpod Notifier that replaces UseCase classes
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider;
import 'auction_state.dart';
import 'seller_auctions_pager.dart';
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
/// - RelistAuction → relistAuction
///
/// Uses Riverpod Notifier for state management
class AuctionNotifier extends Notifier<AuctionNotifierState> {
  late AuctionRepository _auctionRepository;
  late ILoggerService _logger;

  // Synchronous double-submit guards for financial operations
  bool _isPlacingBid = false;

  @override
  AuctionNotifierState build() {
    // Dependencies will be injected via provider override
    // This is a placeholder - actual injection happens in provider
    _auctionRepository = ref.watch(auctionRepositoryProvider);
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
      (error) => state = state.copyWith(isLoading: false, error: error),
      (auctions) => state = state.copyWith(
        auctions: auctions,
        isLoading: false,
        error: null,
      ),
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
      (error) => state = state.copyWith(isLoading: false, error: error),
      (auctions) => state = state.copyWith(
        auctions: auctions,
        isLoading: false,
        error: null,
      ),
    );
  }

  // ========== Auction Detail Operations ==========

  /// Load auction details
  Future<void> loadAuctionDetails(String auctionId) async {
    state = state.copyWith(isLoading: true, clearError: true);

    final result = await _auctionRepository.getAuctionById(auctionId);

    result.fold(
      (error) => state = state.copyWith(isLoading: false, error: error),
      (auction) {
        state = state.copyWith(
          selectedAuction: auction,
          isLoading: false,
          error: null,
        );

        // Note: View tracking is handled by backend automatically via GET endpoint
      },
    );
  }

  /// Load auction bids
  Future<void> loadAuctionBids(String auctionId, {int limit = 50}) async {
    final result = await _auctionRepository.getAuctionBids(
      auctionId: auctionId,
      limit: limit,
    );

    result.fold(
      (error) => state = state.copyWith(error: error),
      (bids) => state = state.copyWith(bids: bids),
    );
  }

  // ========== Bidding Operations ==========

  /// Place bid on auction
  ///
  /// BACKEND AUTHORITY: The following validations use DERIVED PRESENTATION STATE
  /// for UX optimization (fail fast). The BACKEND is the FINAL AUTHORITY for all
  /// business decisions. These client-side checks are purely for better UX.
  ///
  /// Business Rules:
  /// - Auction must be active
  /// - Auction must not have ended
  /// - Bid amount must be >= minimum bid
  /// - User cannot bid on own auction
  Future<bool> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async {
    // Synchronous guard - prevent double-tap
    if (_isPlacingBid) return false;
    _isPlacingBid = true;

    try {
      state = state.copyWith(isPlacingBid: true, clearError: true);

      // Get auction first for validation
      final auctionResult = await _auctionRepository.getAuctionById(auctionId);

      return auctionResult.fold(
        (error) {
          state = state.copyWith(isPlacingBid: false, error: error);
          return false;
        },
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
          // Derived from factual fields: currentBid + bidIncrement
          // Validate bid amount
          final minimumBid = auction.currentBid + auction.bidIncrement;
          if (amount < minimumBid) {
            state = state.copyWith(
              isPlacingBid: false,
              error: 'Bid minimum: Rp ${formatGroupedAmount(minimumBid.round())}',
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
          await loadAuctionDetails(auctionId);
          await loadAuctionBids(auctionId);
          ref.invalidate(auctionStreamProvider(auctionId));
          ref.invalidate(auctionBidsStreamProvider(auctionId));
          return true;
        },
      );
    } finally {
      // Always reset guard in finally
      _isPlacingBid = false;
    }
  }

  // ========== Auction CRUD Operations ==========

  /// Create new auction. A Product is created inline by the backend from
  /// the item fields below — there is no productId/forSaleId parameter.
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
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    required PreparationTime preparationTime,
    required List<String> shippingSetupIds,
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
      preparationTime: preparationTime,
      shippingSetupIds: shippingSetupIds,
    );

    if (result.isError) {
      state = state.copyWith(
        isCreating: false,
        error: result.error ?? 'Gagal membuat lelang',
        errorCode: result.errorCode,
        errorDetails: result.errorDetails,
      );
      return false;
    }

    state = state.copyWith(
      isCreating: false,
      error: null,
      successMessage: 'Lelang berhasil dibuat',
      selectedAuction: result.data,
    );
    // Invalidate discovery lists (one engine — Future) so marketplace/showcase reflect new auction immediately.
    try {
      ref.invalidate(marketplaceAuctionsProvider);
      ref.invalidate(sellerAuctionsProvider(sellerId));
      // My Auctions surface — the canonical owner-inventory pager — must
      // reflect the new auction while it is alive.
      ref.invalidate(sellerAuctionsPagerProvider);
    } catch (_) {}
    return true;
  }

  /// Update auction
  Future<bool> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    state = state.copyWith(isUpdating: true, clearError: true);

    final result = await _auctionRepository.updateAuction(auctionId, updates);

    return result.fold(
      (error) {
        state = state.copyWith(isUpdating: false, error: error);
        return false;
      },
      (auction) {
        state = state.copyWith(
          isUpdating: false,
          error: null,
          successMessage: 'Lelang berhasil diupdate',
          selectedAuction: auction,
        );
        return true;
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
      (error) {
        state = state.copyWith(isLoading: false, error: error);
        return false;
      },
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
    );
  }

  /// Relist (republish) an auction that ended with no bids, or one that
  /// lapsed before activation.
  ///
  /// The backend decides whether the auction is relistable; this only reports
  /// the outcome so the caller can refresh the inventory or surface failure.
  Future<bool> relistAuction({
    required String auctionId,
    required String title,
    required String description,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);

    final result = await _auctionRepository.relistAuction(
      auctionId: auctionId,
      title: title,
      description: description,
      openingBid: openingBid,
      bidIncrement: bidIncrement,
      buyNowPrice: buyNowPrice,
      startMode: startMode,
      scheduledStartAt: scheduledStartAt,
      durationHours: durationHours,
    );

    return result.fold(
      (error) {
        state = state.copyWith(isLoading: false, error: error);
        return false;
      },
      (_) {
        state = state.copyWith(
          isLoading: false,
          error: null,
          successMessage: 'Lelang dijadwalkan ulang',
        );
        loadAuctionDetails(auctionId);
        return true;
      },
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

/// Auction Notifier Provider
final auctionNotifierProvider =
    NotifierProvider<AuctionNotifier, AuctionNotifierState>(
      AuctionNotifier.new,
    );

// ========== Unified Discovery Providers (ONE ENGINE — FutureProvider, like ForSale) ==========
// Business truth: marketplace/showcase list is discovery (scheduled+active), not live.
// Live is detail/bids only. Using FutureProvider + pull-to-refresh + invalidate
// mirrors forSalesProvider and removes Timer/StreamController complexity.

/// Marketplace auctions — canonical discovery (scheduled+active, like ForSale active).
/// Replaces polling StreamProvider; immediate fetch, no Timer.
final marketplaceAuctionsProvider =
    FutureProvider.autoDispose<List<Auction>>((ref) async {
  final repository = ref.watch(auctionRepositoryProvider);
  final result = await repository.getActiveAuctions(limit: 50);
  return result.fold((e) => throw Exception(e), (auctions) {
    final now = DateTime.now();
    final cutoff = now.subtract(const Duration(minutes: 5));
    final filtered = auctions.where((a) => a.endTime.isAfter(cutoff)).toList();
    filtered.sort((a, b) => a.endTime.compareTo(b.endTime));
    return filtered;
  });
});

/// Seller showcase auctions — mirrors sellerForSalesProvider (Future, not Stream).
/// Limit 50 = backend max (auction max 50, for-sale 100). 100 was 400.
final sellerAuctionsProvider =
    FutureProvider.autoDispose.family<List<Auction>, String>((ref, sellerId) async {
  final repository = ref.watch(auctionRepositoryProvider);
  final result = await repository.getUserAuctions(sellerId: sellerId, limit: 50);
  return result.fold((e) => throw Exception(e), (a) => a);
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

// ========== Future Providers (for single fetch) ==========

/// Future provider for auction detail
final auctionDetailProvider = FutureProvider.family<Auction?, String>((
  ref,
  auctionId,
) async {
  ref.keepAlive(); // Keep alive to avoid refetching
  final repository = ref.watch(auctionRepositoryProvider);
  final result = await repository.getAuctionById(auctionId);

  return result.fold((error) => throw Exception(error), (auction) => auction);
});

/// Future provider for auction bids
final auctionBidsProvider = FutureProvider.family<List<AuctionBid>, String>((
  ref,
  auctionId,
) async {
  final repository = ref.watch(auctionRepositoryProvider);
  final result = await repository.getAuctionBids(auctionId: auctionId);

  return result.fold((error) => throw Exception(error), (bids) => bids);
});
