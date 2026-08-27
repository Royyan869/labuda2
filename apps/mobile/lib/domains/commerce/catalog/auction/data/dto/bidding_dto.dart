/// Bidding Data Transfer Objects
/// Matches the GET /api/v1/bidding API response from Go backend
library;

import 'package:equatable/equatable.dart';

/// Bidding item response from GET /api/v1/bidding
///
/// Represents a user's bidding view for a single auction.
/// The backend returns data in snake_case, we map to camelCase.
class BiddingItemDto extends Equatable {
  final String auctionId;
  final String title;
  final double yourLastBid;
  final double currentBid;
  final String status; // leading | outbid | won | lost | waiting_claim
  final DateTime endAt;
  final DateTime updatedAt;

  const BiddingItemDto({
    required this.auctionId,
    required this.title,
    required this.yourLastBid,
    required this.currentBid,
    required this.status,
    required this.endAt,
    required this.updatedAt,
  });

  factory BiddingItemDto.fromJson(Map<String, dynamic> json) {
    return BiddingItemDto(
      auctionId: json['auction_id'] as String,
      title: json['title'] as String,
      yourLastBid: (json['your_last_bid'] as num).toDouble(),
      currentBid: (json['current_bid'] as num).toDouble(),
      status: json['status'] as String,
      endAt: DateTime.parse(json['end_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
    'auction_id': auctionId,
    'title': title,
    'your_last_bid': yourLastBid,
    'current_bid': currentBid,
    'status': status,
    'end_at': endAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [auctionId, title, status];
}

/// Bidding result response from GET /api/v1/bidding
///
/// Contains the list of bidding items plus aggregated counts.
class BiddingResultDto extends Equatable {
  final List<BiddingItemDto> items;
  final int activeCount;
  final int wonCount;
  final int lostCount;

  const BiddingResultDto({
    required this.items,
    required this.activeCount,
    required this.wonCount,
    required this.lostCount,
  });

  factory BiddingResultDto.fromJson(Map<String, dynamic> json) {
    return BiddingResultDto(
      items:
          (json['items'] as List<dynamic>?)
              ?.map(
                (item) => BiddingItemDto.fromJson(item as Map<String, dynamic>),
              )
              .toList() ??
          [],
      activeCount: json['active_count'] as int? ?? 0,
      wonCount: json['won_count'] as int? ?? 0,
      lostCount: json['lost_count'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    'items': items.map((item) => item.toJson()).toList(),
    'active_count': activeCount,
    'won_count': wonCount,
    'lost_count': lostCount,
  };

  @override
  List<Object?> get props => [items, activeCount, wonCount, lostCount];
}
