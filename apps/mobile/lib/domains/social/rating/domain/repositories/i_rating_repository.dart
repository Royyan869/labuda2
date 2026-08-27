import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';

/// CANONICAL RATING REPOSITORY INTERFACE
///
/// Defines the contract for rating data operations aligned with backend.
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no update/delete after submission)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings (no auction, service, delivery contexts)
/// - Single score 1-5 (int, not double)
/// - Optional comment text (no media, no helpful voting)
abstract interface class IRatingRepository {
  /// Create a new rating for a completed order
  ///
  /// API: POST /api/v1/orders/{orderId}/ratings
  ///
  /// Returns [Result] with:
  /// - [Rating] if successful
  /// - [Error] if:
  ///   - Order not found
  ///   - Order not completed
  ///   - User not the buyer
  ///   - Order already rated
  ///   - Rating frequency limit exceeded
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  });

  /// Get ratings received by a seller
  ///
  /// API: GET /api/v1/users/{sellerId}/ratings
  ///
  /// Parameters:
  /// - [sellerId]: User ID of the seller
  /// - [limit]: Max results (default 20, max 50)
  /// - [cursor]: Unix timestamp in nanoseconds for pagination
  ///
  /// Returns paginated list of ratings received by the seller.
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  });

  /// Get ratings given by a buyer
  ///
  /// API: GET /api/v1/users/me/ratings/given
  ///
  /// Parameters:
  /// - [limit]: Max results (default 20, max 50)
  /// - [cursor]: Unix timestamp in nanoseconds for pagination
  ///
  /// Returns paginated list of ratings given by the current user (buyer).
  Future<Result<List<Rating>>> getRatingsGiven({int limit = 20, int? cursor});

  /// Get rating summary for a seller
  ///
  /// API: GET /api/v1/users/{sellerId}/ratings/summary
  ///
  /// Returns aggregated rating summary:
  /// - Total ratings count
  /// - Average rating (1-5)
  /// - Distribution by star rating (1-5)
  ///
  /// RATING INVALIDATION: Only includes valid ratings (invalidated_at IS NULL).
  Future<Result<RatingSummary>> getRatingSummary({required String sellerId});

  /// Get rating for a specific order
  ///
  /// Returns the rating for the given order ID, if exists.
  /// Returns null if order has not been rated yet.
  ///
  /// PARKED V1: Backend has no GET /orders/:id/ratings route; this remains
  /// null until rating detail endpoint is designed.
  Future<Result<Rating?>> getRatingForOrder({required String orderId});
}
