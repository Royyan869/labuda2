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
  // Canonical int read representation: backend AuctionBid.Amount is int64
  // and the wire emits an integer JSON literal (bidToResponseWithBidderCard).
  final int amount;
  final DateTime createdAt;
  //
  // PURGED: isWinning/isOutbid — the bid wire (bidToResponseWithBidderCard)
  // never emitted these flags; they were always-false dead state. Canonical
  // buyer bid-position authority is GET /api/v1/bidding (leading | outbid |
  // won | lost | waiting_claim) mapped to BiddingItem.

  const AuctionBid({
    required this.id,
    required this.auctionId,
    required this.bidderId,
    required this.bidderUsername,
    this.bidderAvatarUrl,
    this.bidderLifecycle,
    required this.amount,
    required this.createdAt,
  });

  AuctionBid copyWith({
    String? id,
    String? auctionId,
    String? bidderId,
    String? bidderUsername,
    String? bidderAvatarUrl,
    String? bidderLifecycle,
    int? amount,
    DateTime? createdAt,
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
