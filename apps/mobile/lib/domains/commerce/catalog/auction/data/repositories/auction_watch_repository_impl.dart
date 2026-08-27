/// Auction Watch Repository Implementation
/// Retargeted to /saved-items (C1D: canonical parking authority)
///
/// C1D audit verdict: /auctions/:id/watch endpoints never existed in backend.
/// Canonical authority = saved_items table via /saved-items API.
/// Mapping:
///   watchAuction       → POST   /saved-items {target_type:'auction', target_id}
///   unwatchAuction     → DELETE /saved-items/{id}?type=auction
///   isWatching         → GET    /saved-items/check?type=auction&id=...
///   getWatchedAuctions → [] (V1: Auction entity needs fields not in snapshot)
///   getWatchStats      → isWatching (saved-items) + totalWatchers:0
///   getWatchCount      → 0 (no backend aggregate)
///   getAuctionWatchers → [] (no backend watcher list)
///   toggleWatch        → delegates to isWatching + watch/unwatch
///
/// V1 constraints:
///   - Notification preferences dropped (no backend storage).
///   - Watcher counts always 0 (no backend aggregate).
library;

import 'dart:async';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';

/// Auction Watch Repository — saved-items backed implementation.
class AuctionWatchRepositoryImpl implements AuctionWatchRepository {
  final SavedItemRepository _savedItemRepo;
  final ILoggerService _logger;

  AuctionWatchRepositoryImpl({
    required SavedItemRepository savedItemRepo,
    required ILoggerService logger,
  }) : _savedItemRepo = savedItemRepo,
       _logger = logger;

  // ── watch ──────────────────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<AuctionWatcher>> watchAuction({
    required String auctionId,
    required String userId,
    bool notifyOnBid = true, // V1: ignored, no backend preference support
    bool notifyOnEndingSoon = true, // V1: ignored
    bool notifyOnEnded = true, // V1: ignored
  }) async {
    try {
      await _savedItemRepo.addSavedItem(
        targetType: 'auction',
        targetId: auctionId,
      );

      final watcher = AuctionWatcher(
        id: '${auctionId}_$userId',
        auctionId: auctionId,
        userId: userId,
        createdAt: DateTime.now(),
        // V1: notification prefs not stored — default values for type compat
      );

      return RepositoryResult.success(watcher);
    } catch (e) {
      _logger.error('Failed to watch auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  // ── unwatch ────────────────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<void>> unwatchAuction({
    required String auctionId,
    required String userId,
  }) async {
    try {
      await _savedItemRepo.removeSavedItem(
        targetType: 'auction',
        targetId: auctionId,
      );
      return RepositoryResult.success(null);
    } catch (e) {
      _logger.error('Failed to unwatch auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  // ── isWatching ─────────────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<bool>> isWatching({
    required String auctionId,
    required String userId,
  }) async {
    try {
      final result = await _savedItemRepo.isSaved(
        targetType: 'auction',
        targetId: auctionId,
      );
      return RepositoryResult.success(result);
    } catch (e) {
      _logger.error('Failed to check watch status: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  // ── getWatchStats ──────────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<AuctionWatchStats>> getWatchStats({
    required String auctionId,
    required String currentUserId,
  }) async {
    try {
      final watching = await _savedItemRepo.isSaved(
        targetType: 'auction',
        targetId: auctionId,
      );

      final stats = AuctionWatchStats(
        auctionId: auctionId,
        totalWatchers: null,
        isWatchedByCurrentUser: watching,
      );

      return RepositoryResult.success(stats);
    } catch (e) {
      _logger.error('Failed to get watch stats: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  // ── getWatchCount ──────────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<int>> getWatchCount(String auctionId) async {
    return RepositoryResult.error(
      'Watch count unavailable until backend exposes a real aggregate.',
    );
  }

  // ── getWatchedAuctions ─────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<List<Auction>>> getWatchedAuctions({
    required String userId,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    return RepositoryResult.error(
      'Watched auction feed unavailable until a real backend source exists.',
    );
  }

  // ── getAuctionWatchers ─────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<List<AuctionWatcher>>> getAuctionWatchers({
    required String auctionId,
    int limit = 100,
  }) async {
    return RepositoryResult.error(
      'Auction watcher list unavailable until backend exposes a real source.',
    );
  }

  // ── watchWatchStats (stream) ───────────────────────────────────────────────

  @override
  Stream<AuctionWatchStats> watchWatchStats({
    required String auctionId,
    required String currentUserId,
  }) {
    return Stream.periodic(
      const Duration(seconds: 30),
      (_) => auctionId,
    ).asyncMap((_) async {
      final result = await getWatchStats(
        auctionId: auctionId,
        currentUserId: currentUserId,
      );
      return result.fold(
        (stats) => stats,
        (_) => AuctionWatchStats(
          auctionId: auctionId,
          totalWatchers: null,
          isWatchedByCurrentUser: false,
        ),
      );
    });
  }

  // ── toggleWatch ────────────────────────────────────────────────────────────

  @override
  Future<RepositoryResult<bool>> toggleWatch({
    required String auctionId,
    required String userId,
  }) async {
    try {
      final checkResult = await isWatching(
        auctionId: auctionId,
        userId: userId,
      );

      if (checkResult.isFailure) {
        return RepositoryResult.error(checkResult.error ?? 'Unknown error');
      }

      final currentlyWatching = checkResult.data ?? false;

      if (currentlyWatching) {
        await unwatchAuction(auctionId: auctionId, userId: userId);
        return RepositoryResult.success(false);
      } else {
        await watchAuction(auctionId: auctionId, userId: userId);
        return RepositoryResult.success(true);
      }
    } catch (e) {
      _logger.error('Failed to toggle watch: $e');
      return RepositoryResult.error(e.toString());
    }
  }
}
