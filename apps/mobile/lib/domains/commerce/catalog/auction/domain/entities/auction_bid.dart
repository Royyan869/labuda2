/// Auction Bid domain entities
/// Pure Dart entities - no Firebase, Flutter, or HTTP dependencies
library;

/// Auction bid entity
/// Represents a single bid on an auction
///
/// D14 — Bidder identity surfaces through the backend `publiccard.UserCard`
/// nested under `bidder`. Carries coarsened lifecycle so the UI can
/// redact degraded bidders without a second fetch.
class AuctionBid {
  final String id;
  final String auctionId;
  final String bidderId;
  final String bidderUsername;
  final String? bidderAvatarUrl;
  // D14 — coarsened public lifecycle of the bidder: "active" |
  // "unavailable" | "removed". Null when the backend has not hydrated.
  final String? bidderLifecycle;
  final double amount;
  final DateTime createdAt;
  final bool isWinning;
  final bool isOutbid;

  const AuctionBid({
    required this.id,
    required this.auctionId,
    required this.bidderId,
    required this.bidderUsername,
    this.bidderAvatarUrl,
    this.bidderLifecycle,
    required this.amount,
    required this.createdAt,
    this.isWinning = false,
    this.isOutbid = false,
  });

  AuctionBid copyWith({
    String? id,
    String? auctionId,
    String? bidderId,
    String? bidderUsername,
    String? bidderAvatarUrl,
    String? bidderLifecycle,
    double? amount,
    DateTime? createdAt,
    bool? isWinning,
    bool? isOutbid,
  }) {
    return AuctionBid(
      id: id ?? this.id,
      auctionId: auctionId ?? this.auctionId,
      bidderId: bidderId ?? this.bidderId,
      bidderUsername: bidderUsername ?? this.bidderUsername,
      bidderAvatarUrl: bidderAvatarUrl ?? this.bidderAvatarUrl,
      bidderLifecycle: bidderLifecycle ?? this.bidderLifecycle,
      amount: amount ?? this.amount,
      createdAt: createdAt ?? this.createdAt,
      isWinning: isWinning ?? this.isWinning,
      isOutbid: isOutbid ?? this.isOutbid,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AuctionBid && other.id == id;
  }

  @override
  int get hashCode => id.hashCode;
}

/// Current bid summary for an auction
class CurrentBidSummary {
  final String auctionId;
  final double currentHighestBid;
  final String? highestBidderId;
  final double minimumBid;
  final int totalBids;
  final int timeRemainingSeconds;
  final DateTime endTime;
  final bool isExtended;
  final AuctionBidStatus status;

  const CurrentBidSummary({
    required this.auctionId,
    required this.currentHighestBid,
    this.highestBidderId,
    required this.minimumBid,
    required this.totalBids,
    required this.timeRemainingSeconds,
    required this.endTime,
    required this.isExtended,
    required this.status,
  });

  CurrentBidSummary copyWith({
    String? auctionId,
    double? currentHighestBid,
    String? highestBidderId,
    double? minimumBid,
    int? totalBids,
    int? timeRemainingSeconds,
    DateTime? endTime,
    bool? isExtended,
    AuctionBidStatus? status,
  }) {
    return CurrentBidSummary(
      auctionId: auctionId ?? this.auctionId,
      currentHighestBid: currentHighestBid ?? this.currentHighestBid,
      highestBidderId: highestBidderId ?? this.highestBidderId,
      minimumBid: minimumBid ?? this.minimumBid,
      totalBids: totalBids ?? this.totalBids,
      timeRemainingSeconds: timeRemainingSeconds ?? this.timeRemainingSeconds,
      endTime: endTime ?? this.endTime,
      isExtended: isExtended ?? this.isExtended,
      status: status ?? this.status,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is CurrentBidSummary && other.auctionId == auctionId;
  }

  @override
  int get hashCode => auctionId.hashCode;
}

/// Auction bid status
enum AuctionBidStatus {
  /// Auction is active and accepting bids
  active,

  /// Auction has ended
  ended,

  /// Auction was cancelled
  cancelled,

  /// Auction is scheduled to start
  scheduled,
}
