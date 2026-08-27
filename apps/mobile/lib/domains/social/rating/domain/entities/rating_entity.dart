import 'package:meta/meta.dart';

/// CANONICAL RATING ENTITY
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no update/delete after submission)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings (no auction, service, delivery contexts)
/// - Single score 1-5 (no granular criteria scores)
/// - Optional comment text (no media, no helpful voting)
///
/// This entity represents the canonical backend contract from order_ratings table.
/// All fake/unsupported features have been removed to align with backend truth.
@immutable
class Rating {
  /// Unique rating ID
  final String id;

  /// Order ID that this rating is for
  final String orderId;

  /// User ID of the buyer who created the rating
  final String buyerId;

  /// User ID of the seller being rated
  final String sellerId;

  /// Rating value (1-5)
  final int ratingValue;

  /// Optional review comment
  final String? comment;

  /// When the rating was created (immutable)
  final DateTime createdAt;

  const Rating({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.ratingValue,
    this.comment,
    required this.createdAt,
  });

  /// Create a Rating from JSON (API response)
  factory Rating.fromJson(Map<String, dynamic> json) {
    return Rating(
      id: json['id'] as String,
      orderId: json['order_id'] as String,
      buyerId: json['buyer_id'] as String,
      sellerId: json['seller_id'] as String,
      ratingValue: json['rating_value'] as int,
      comment: json['comment'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  /// Convert to JSON for API requests
  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'order_id': orderId,
      'buyer_id': buyerId,
      'seller_id': sellerId,
      'rating_value': ratingValue,
      'comment': comment,
      'created_at': createdAt.toIso8601String(),
    };
  }

  Rating copyWith({
    String? id,
    String? orderId,
    String? buyerId,
    String? sellerId,
    int? ratingValue,
    String? comment,
    DateTime? createdAt,
  }) {
    return Rating(
      id: id ?? this.id,
      orderId: orderId ?? this.orderId,
      buyerId: buyerId ?? this.buyerId,
      sellerId: sellerId ?? this.sellerId,
      ratingValue: ratingValue ?? this.ratingValue,
      comment: comment ?? this.comment,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is Rating &&
        other.id == id &&
        other.orderId == orderId &&
        other.buyerId == buyerId &&
        other.sellerId == sellerId &&
        other.ratingValue == ratingValue &&
        other.comment == comment &&
        other.createdAt == createdAt;
  }

  @override
  int get hashCode {
    return id.hashCode ^
        orderId.hashCode ^
        buyerId.hashCode ^
        sellerId.hashCode ^
        ratingValue.hashCode ^
        comment.hashCode ^
        createdAt.hashCode;
  }

  @override
  String toString() {
    return 'Rating(id: $id, orderId: $orderId, buyerId: $buyerId, sellerId: $sellerId, ratingValue: $ratingValue, comment: $comment, createdAt: $createdAt)';
  }
}

/// Summary of seller's ratings from backend aggregation
///
/// RATING INVALIDATION: Only includes valid ratings (invalidated_at IS NULL).
/// Invalidated ratings from refunded orders are excluded from the summary.
@immutable
class RatingSummary {
  /// Total number of valid ratings received
  final int totalRatings;

  /// Average rating value (1-5)
  final double averageRating;

  /// Number of 1-star ratings
  final int oneStarCount;

  /// Number of 2-star ratings
  final int twoStarCount;

  /// Number of 3-star ratings
  final int threeStarCount;

  /// Number of 4-star ratings
  final int fourStarCount;

  /// Number of 5-star ratings
  final int fiveStarCount;

  const RatingSummary({
    required this.totalRatings,
    required this.averageRating,
    required this.oneStarCount,
    required this.twoStarCount,
    required this.threeStarCount,
    required this.fourStarCount,
    required this.fiveStarCount,
  });

  /// Create a RatingSummary from JSON (API response)
  factory RatingSummary.fromJson(Map<String, dynamic> json) {
    return RatingSummary(
      totalRatings: json['total_ratings'] as int,
      averageRating: (json['average_rating'] as num).toDouble(),
      oneStarCount: json['one_star_count'] as int,
      twoStarCount: json['two_star_count'] as int,
      threeStarCount: json['three_star_count'] as int,
      fourStarCount: json['four_star_count'] as int,
      fiveStarCount: json['five_star_count'] as int,
    );
  }

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

  RatingSummary copyWith({
    int? totalRatings,
    double? averageRating,
    int? oneStarCount,
    int? twoStarCount,
    int? threeStarCount,
    int? fourStarCount,
    int? fiveStarCount,
  }) {
    return RatingSummary(
      totalRatings: totalRatings ?? this.totalRatings,
      averageRating: averageRating ?? this.averageRating,
      oneStarCount: oneStarCount ?? this.oneStarCount,
      twoStarCount: twoStarCount ?? this.twoStarCount,
      threeStarCount: threeStarCount ?? this.threeStarCount,
      fourStarCount: fourStarCount ?? this.fourStarCount,
      fiveStarCount: fiveStarCount ?? this.fiveStarCount,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is RatingSummary &&
        other.totalRatings == totalRatings &&
        other.averageRating == averageRating &&
        other.oneStarCount == oneStarCount &&
        other.twoStarCount == twoStarCount &&
        other.threeStarCount == threeStarCount &&
        other.fourStarCount == fourStarCount &&
        other.fiveStarCount == fiveStarCount;
  }

  @override
  int get hashCode {
    return totalRatings.hashCode ^
        averageRating.hashCode ^
        oneStarCount.hashCode ^
        twoStarCount.hashCode ^
        threeStarCount.hashCode ^
        fourStarCount.hashCode ^
        fiveStarCount.hashCode;
  }

  @override
  String toString() {
    return 'RatingSummary(totalRatings: $totalRatings, averageRating: $averageRating, distribution: {$oneStarCount, $twoStarCount, $threeStarCount, $fourStarCount, $fiveStarCount})';
  }
}
