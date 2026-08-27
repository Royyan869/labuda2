// CANONICAL RATING API MODELS
//
// These models match the Go backend contract from order_ratings table:
// backend/internal/domain/rating/
//
// Business Truth (LOCKED):
// - Rating is IMMUTABLE (no update/delete after submission)
// - Rating direction is BUYER → SELLER ONLY
// - Only order-based ratings
// - Single score 1-5 (int, not double)
// - Optional comment text (no media, no helpful voting)
//
// All fake/unsupported features have been removed to align with backend truth.

import 'package:equatable/equatable.dart';
import 'package:labuda/core/api/models/common_api_models.dart';

// =============================================================================
// CANONICAL Rating Request/Response DTOs
// =============================================================================

/// Request to create a rating for a completed order
///
/// API: POST /api/v1/orders/{id}/ratings
class CreateRatingApiRequest {
  final int ratingValue;
  final String? comment;

  const CreateRatingApiRequest({required this.ratingValue, this.comment});

  Map<String, dynamic> toJson() => {
    'rating_value': ratingValue,
    if (comment != null) 'comment': comment,
  };

  @override
  String toString() =>
      'CreateRatingApiRequest(ratingValue: $ratingValue, comment: $comment)';
}

/// Rating response from backend
///
/// Matches OrderRating entity from backend:
/// - id: UUID
/// - order_id: UUID
/// - buyer_id: UUID (user who created the rating)
/// - seller_id: UUID (user being rated)
/// - rating_value: int (1-5)
/// - comment: optional string
/// - created_at: timestamp (immutable)
class RatingApiResponse extends Equatable {
  final String id;
  final String orderId;
  final String buyerId;
  final String sellerId;
  final int ratingValue;
  final String? comment;
  final DateTime createdAt;
  final UserBriefApiResponse? buyer;
  final UserBriefApiResponse? seller;

  const RatingApiResponse({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.ratingValue,
    this.comment,
    required this.createdAt,
    this.buyer,
    this.seller,
  });

  /// Parse from backend JSON response
  factory RatingApiResponse.fromJson(Map<String, dynamic> json) {
    return RatingApiResponse(
      id: json['id'] as String,
      orderId: json['order_id'] as String,
      buyerId: json['buyer_id'] as String,
      sellerId: json['seller_id'] as String,
      ratingValue: json['rating_value'] as int,
      comment: json['comment'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      buyer: json['buyer'] != null
          ? UserBriefApiResponse.fromJson(json['buyer'] as Map<String, dynamic>)
          : null,
      seller: json['seller'] != null
          ? UserBriefApiResponse.fromJson(
              json['seller'] as Map<String, dynamic>,
            )
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'order_id': orderId,
    'buyer_id': buyerId,
    'seller_id': sellerId,
    'rating_value': ratingValue,
    if (comment != null) 'comment': comment,
    'created_at': createdAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    id,
    orderId,
    buyerId,
    sellerId,
    ratingValue,
    comment,
    createdAt,
  ];
}

/// Rating summary response from backend aggregation
///
/// API: GET /api/v1/users/{id}/ratings/summary
///
/// RATING INVALIDATION: Only includes valid ratings (invalidated_at IS NULL).
/// Invalidated ratings from refunded orders are excluded from the summary.
class RatingSummaryApiResponse extends Equatable {
  final int totalRatings;
  final double averageRating;
  final int oneStarCount;
  final int twoStarCount;
  final int threeStarCount;
  final int fourStarCount;
  final int fiveStarCount;

  const RatingSummaryApiResponse({
    required this.totalRatings,
    required this.averageRating,
    required this.oneStarCount,
    required this.twoStarCount,
    required this.threeStarCount,
    required this.fourStarCount,
    required this.fiveStarCount,
  });

  /// Parse from backend JSON response
  factory RatingSummaryApiResponse.fromJson(Map<String, dynamic> json) {
    return RatingSummaryApiResponse(
      totalRatings: json['total_ratings'] as int,
      averageRating: (json['average_rating'] as num).toDouble(),
      oneStarCount: json['one_star_count'] as int,
      twoStarCount: json['two_star_count'] as int,
      threeStarCount: json['three_star_count'] as int,
      fourStarCount: json['four_star_count'] as int,
      fiveStarCount: json['five_star_count'] as int,
    );
  }

  Map<String, dynamic> toJson() => {
    'total_ratings': totalRatings,
    'average_rating': averageRating,
    'one_star_count': oneStarCount,
    'two_star_count': twoStarCount,
    'three_star_count': threeStarCount,
    'four_star_count': fourStarCount,
    'five_star_count': fiveStarCount,
  };

  /// Get distribution as a map (1-5 stars -> count)
  Map<int, int> get distribution {
    return {
      1: oneStarCount,
      2: twoStarCount,
      3: threeStarCount,
      4: fourStarCount,
      5: fiveStarCount,
    };
  }

  /// Calculate percentage of ratings that are 4-5 stars
  double get positiveRatingPercentage {
    if (totalRatings == 0) return 0.0;
    return ((fourStarCount + fiveStarCount) / totalRatings) * 100;
  }

  @override
  List<Object?> get props => [
    totalRatings,
    averageRating,
    oneStarCount,
    twoStarCount,
    threeStarCount,
    fourStarCount,
    fiveStarCount,
  ];
}
