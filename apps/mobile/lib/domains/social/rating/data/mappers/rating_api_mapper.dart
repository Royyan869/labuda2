import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/social/rating/data/dto/rating_api_models.dart';

/// CANONICAL Rating Mapper
///
/// Maps between backend API models and domain entities.
/// Simplified to align with canonical backend contract from order_ratings table.
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no update/delete after submission)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings
/// - Single score 1-5 (int, not double)
class RatingApiMapper {
  // ===========================================
  // API TO DOMAIN
  // ===========================================

  /// Convert RatingApiResponse to Rating entity
  ///
  /// Maps backend order_rating fields to domain entity:
  /// - id -> id
  /// - order_id -> orderId
  /// - buyer_id -> buyerId
  /// - seller_id -> sellerId
  /// - rating_value -> ratingValue
  /// - comment -> comment
  /// - created_at -> createdAt
  static Rating toRating(RatingApiResponse response) {
    return Rating(
      id: response.id,
      orderId: response.orderId,
      buyerId: response.buyerId,
      sellerId: response.sellerId,
      ratingValue: response.ratingValue,
      comment: response.comment,
      createdAt: response.createdAt,
    );
  }

  /// Convert list of RatingApiResponse to list of Rating entities
  static List<Rating> toRatingList(List<RatingApiResponse> responses) {
    return responses.map(toRating).toList();
  }

  /// Convert RatingSummaryApiResponse to RatingSummary entity
  static RatingSummary toRatingSummary(RatingSummaryApiResponse response) {
    return RatingSummary(
      totalRatings: response.totalRatings,
      averageRating: response.averageRating,
      oneStarCount: response.oneStarCount,
      twoStarCount: response.twoStarCount,
      threeStarCount: response.threeStarCount,
      fourStarCount: response.fourStarCount,
      fiveStarCount: response.fiveStarCount,
    );
  }

  // ===========================================
  // DOMAIN TO API
  // ===========================================

  /// Build CreateRatingApiRequest from domain data
  ///
  /// For use with POST /api/v1/orders/{id}/ratings
  static CreateRatingApiRequest buildCreateRequest({
    required int ratingValue,
    String? comment,
  }) {
    return CreateRatingApiRequest(ratingValue: ratingValue, comment: comment);
  }
}
