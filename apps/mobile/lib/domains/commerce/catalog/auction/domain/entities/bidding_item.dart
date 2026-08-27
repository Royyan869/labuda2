/// Bidding Domain Entities
library;

import 'package:equatable/equatable.dart';

/// Bidding status for a single auction
enum BiddingStatus {
  leading,
  outbid,
  waitingClaim,
  won,
  lost;

  static BiddingStatus fromString(String status) {
    switch (status) {
      case 'leading':
        return BiddingStatus.leading;
      case 'outbid':
        return BiddingStatus.outbid;
      case 'waiting_claim':
        return BiddingStatus.waitingClaim;
      case 'won':
        return BiddingStatus.won;
      case 'lost':
        return BiddingStatus.lost;
      default:
        return BiddingStatus.lost;
    }
  }

  String toDisplayString() {
    switch (this) {
      case BiddingStatus.leading:
        return 'Leading';
      case BiddingStatus.outbid:
        return 'Outbid';
      case BiddingStatus.waitingClaim:
        return 'Waiting Claim';
      case BiddingStatus.won:
        return 'Won';
      case BiddingStatus.lost:
        return 'Lost';
    }
  }

  bool get isActive =>
      this == BiddingStatus.leading ||
      this == BiddingStatus.outbid ||
      this == BiddingStatus.waitingClaim;
}

/// Bidding item entity
///
/// Represents a user's bidding view for a single auction.
class BiddingItem extends Equatable {
  final String auctionId;
  final String title;
  final double yourLastBid;
  final double currentBid;
  final BiddingStatus status;
  final DateTime endAt;
  final DateTime updatedAt;

  const BiddingItem({
    required this.auctionId,
    required this.title,
    required this.yourLastBid,
    required this.currentBid,
    required this.status,
    required this.endAt,
    required this.updatedAt,
  });

  BiddingItem copyWith({
    String? auctionId,
    String? title,
    double? yourLastBid,
    double? currentBid,
    BiddingStatus? status,
    DateTime? endAt,
    DateTime? updatedAt,
  }) {
    return BiddingItem(
      auctionId: auctionId ?? this.auctionId,
      title: title ?? this.title,
      yourLastBid: yourLastBid ?? this.yourLastBid,
      currentBid: currentBid ?? this.currentBid,
      status: status ?? this.status,
      endAt: endAt ?? this.endAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  List<Object?> get props => [auctionId, title, status];
}

/// Bidding result entity
///
/// Contains the list of bidding items plus aggregated counts.
class BiddingResult extends Equatable {
  final List<BiddingItem> items;
  final int activeCount;
  final int wonCount;
  final int lostCount;

  const BiddingResult({
    required this.items,
    required this.activeCount,
    required this.wonCount,
    required this.lostCount,
  });

  BiddingResult copyWith({
    List<BiddingItem>? items,
    int? activeCount,
    int? wonCount,
    int? lostCount,
  }) {
    return BiddingResult(
      items: items ?? this.items,
      activeCount: activeCount ?? this.activeCount,
      wonCount: wonCount ?? this.wonCount,
      lostCount: lostCount ?? this.lostCount,
    );
  }

  @override
  List<Object?> get props => [items, activeCount, wonCount, lostCount];
}
