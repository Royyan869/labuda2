/// Auction Watcher domain entities
/// Pure Dart entities - no Firebase, Flutter, or HTTP dependencies
library;

/// Auction watcher entity
/// Represents a user watching an auction for updates
class AuctionWatcher {
  final String id;
  final String auctionId;
  final String userId;
  final DateTime createdAt;

  /// Notification preferences for this watcher
  final bool notifyOnBid;
  final bool notifyOnEndingSoon;
  final bool notifyOnEnded;

  const AuctionWatcher({
    required this.id,
    required this.auctionId,
    required this.userId,
    required this.createdAt,
    this.notifyOnBid = true,
    this.notifyOnEndingSoon = true,
    this.notifyOnEnded = true,
  });

  AuctionWatcher copyWith({
    String? id,
    String? auctionId,
    String? userId,
    DateTime? createdAt,
    bool? notifyOnBid,
    bool? notifyOnEndingSoon,
    bool? notifyOnEnded,
  }) {
    return AuctionWatcher(
      id: id ?? this.id,
      auctionId: auctionId ?? this.auctionId,
      userId: userId ?? this.userId,
      createdAt: createdAt ?? this.createdAt,
      notifyOnBid: notifyOnBid ?? this.notifyOnBid,
      notifyOnEndingSoon: notifyOnEndingSoon ?? this.notifyOnEndingSoon,
      notifyOnEnded: notifyOnEnded ?? this.notifyOnEnded,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AuctionWatcher &&
        other.id == id &&
        other.auctionId == auctionId &&
        other.userId == userId;
  }

  @override
  int get hashCode => id.hashCode ^ auctionId.hashCode ^ userId.hashCode;

  @override
  String toString() =>
      'AuctionWatcher(id: $id, auctionId: $auctionId, userId: $userId)';
}

/// Watch statistics for a specific auction
class AuctionWatchStats {
  final String auctionId;
  final int? totalWatchers;
  final bool isWatchedByCurrentUser;

  const AuctionWatchStats({
    required this.auctionId,
    required this.totalWatchers,
    required this.isWatchedByCurrentUser,
  });

  AuctionWatchStats copyWith({
    String? auctionId,
    int? totalWatchers,
    bool? isWatchedByCurrentUser,
  }) {
    return AuctionWatchStats(
      auctionId: auctionId ?? this.auctionId,
      totalWatchers: totalWatchers ?? this.totalWatchers,
      isWatchedByCurrentUser:
          isWatchedByCurrentUser ?? this.isWatchedByCurrentUser,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AuctionWatchStats &&
        other.auctionId == auctionId &&
        other.totalWatchers == totalWatchers &&
        other.isWatchedByCurrentUser == isWatchedByCurrentUser;
  }

  @override
  int get hashCode =>
      auctionId.hashCode ^
      totalWatchers.hashCode ^
      isWatchedByCurrentUser.hashCode;

  @override
  String toString() =>
      'AuctionWatchStats(auctionId: $auctionId, totalWatchers: $totalWatchers, isWatchedByCurrentUser: $isWatchedByCurrentUser)';
}
