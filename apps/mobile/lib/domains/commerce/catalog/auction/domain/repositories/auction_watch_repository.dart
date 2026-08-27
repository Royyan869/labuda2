/// Auction Watch Repository Interface
/// Pure Dart interface - no implementation details
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_watcher.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

/// Auction Watch Repository Interface
///
/// Defines all auction watch-related operations without implementation details.
/// Implementations can use API, Firestore, or any other data source.
abstract class AuctionWatchRepository {
  /// Watch an auction (add to watchlist)
  Future<RepositoryResult<AuctionWatcher>> watchAuction({
    required String auctionId,
    required String userId,
    bool notifyOnBid = true,
    bool notifyOnEndingSoon = true,
    bool notifyOnEnded = true,
  });

  /// Unwatch an auction (remove from watchlist)
  Future<RepositoryResult<void>> unwatchAuction({
    required String auctionId,
    required String userId,
  });

  /// Check if user is watching an auction
  Future<RepositoryResult<bool>> isWatching({
    required String auctionId,
    required String userId,
  });

  /// Get watch stats for an auction
  Future<RepositoryResult<AuctionWatchStats>> getWatchStats({
    required String auctionId,
    required String currentUserId,
  });

  /// Get watch count for an auction
  Future<RepositoryResult<int>> getWatchCount(String auctionId);

  /// Get all auctions watched by a user
  Future<RepositoryResult<List<Auction>>> getWatchedAuctions({
    required String userId,
    int limit = 20,
    String? lastAuctionId,
  });

  /// Get all watchers for an auction (for notification purposes)
  Future<RepositoryResult<List<AuctionWatcher>>> getAuctionWatchers({
    required String auctionId,
    int limit = 100,
  });

  /// Stream watch stats for real-time updates
  Stream<AuctionWatchStats> watchWatchStats({
    required String auctionId,
    required String currentUserId,
  });

  /// Toggle watch status
  Future<RepositoryResult<bool>> toggleWatch({
    required String auctionId,
    required String userId,
  });
}

/// Watch auction request params
class WatchAuctionParams {
  final String auctionId;
  final String userId;
  final bool notifyOnBid;
  final bool notifyOnEndingSoon;
  final bool notifyOnEnded;

  const WatchAuctionParams({
    required this.auctionId,
    required this.userId,
    this.notifyOnBid = true,
    this.notifyOnEndingSoon = true,
    this.notifyOnEnded = true,
  });

  Map<String, dynamic> toMap() => {
    'auctionId': auctionId,
    'userId': userId,
    'notifyOnBid': notifyOnBid,
    'notifyOnEndingSoon': notifyOnEndingSoon,
    'notifyOnEnded': notifyOnEnded,
  };
}
