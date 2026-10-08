/// Seller Performance read model.
///
/// A live projection over canonical Reputation + Rating authorities. The
/// backend is the sole source of truth — mobile never re-aggregates these
/// numbers. Fulfillment metrics (tier, fulfillment rate, completed orders,
/// cancelled timeout) come from the rolling 90-day reputation authority;
/// rating metrics come from the canonical rating domain.
library;

import 'package:equatable/equatable.dart';

/// Seller Performance Entity — canonical trust + fulfillment projection.
class SellerPerformance extends Equatable {
  final String sellerId;

  /// Current seller tier badge (basic, pro, elite).
  final String tier;

  /// Rolling 90-day fulfillment rate (0.0 - 1.0).
  final double fulfillmentRate;

  /// Rolling 90-day completed orders.
  final int completedOrders;

  /// Rolling 90-day cancelled-timeout orders (shipping timeout).
  final int cancelledTimeout;

  /// Average rating (0-5) from the canonical rating domain.
  final double averageRating;

  /// Total valid review/rating count.
  final int reviewCount;

  final int oneStarCount;
  final int twoStarCount;
  final int threeStarCount;
  final int fourStarCount;
  final int fiveStarCount;

  const SellerPerformance({
    required this.sellerId,
    required this.tier,
    required this.fulfillmentRate,
    required this.completedOrders,
    required this.cancelledTimeout,
    required this.averageRating,
    required this.reviewCount,
    required this.oneStarCount,
    required this.twoStarCount,
    required this.threeStarCount,
    required this.fourStarCount,
    required this.fiveStarCount,
  });

  @override
  List<Object?> get props => [
    sellerId,
    tier,
    fulfillmentRate,
    completedOrders,
    cancelledTimeout,
    averageRating,
    reviewCount,
    oneStarCount,
    twoStarCount,
    threeStarCount,
    fourStarCount,
    fiveStarCount,
  ];
}
