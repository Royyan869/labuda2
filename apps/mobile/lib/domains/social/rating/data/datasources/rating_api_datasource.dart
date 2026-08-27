import 'package:labuda/core/api/base_api_repository.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/rating/data/dto/rating_api_models.dart';

/// CANONICAL API Datasource for Rating operations
///
/// Provides HTTP operations aligned with backend contract.
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no update/delete after submission)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings
/// - Single score 1-5 (int, not double)
/// - Optional comment text (no media, no helpful voting)
class RatingApiDatasource extends BaseApiRepository {
  static const String _ordersPath = '/orders';
  static const String _usersPath = '/users';

  RatingApiDatasource(super.apiClient, {super.logger});

  // ===========================================
  // CANONICAL CREATE METHODS
  // ===========================================

  /// Create a rating for an order (canonical endpoint)
  ///
  /// Endpoint: POST /api/v1/orders/{orderId}/ratings
  Future<Result<RatingApiResponse>> createRatingForOrder(
    String orderId,
    CreateRatingApiRequest request,
  ) async {
    logger?.info('Creating rating for order: $orderId');

    return executeRequest(
      () => apiClient.post(
        '$_ordersPath/$orderId/ratings',
        data: request.toJson(),
      ),
      parser: (data) =>
          RatingApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ===========================================
  // CANONICAL QUERY METHODS
  // ===========================================

  /// Get ratings received by a seller
  ///
  /// Endpoint: GET /api/v1/users/{sellerId}/ratings
  ///
  /// Canonical Rating HTTP contract: keyset pagination with a Unix-nanosecond
  /// [cursor] and [limit] page size. The backend returns a BARE rating
  /// collection (snake_case OrderRating fields) — there is NO page/meta
  /// envelope. Use [executeListRequest] to parse the bare list.
  Future<Result<List<RatingApiResponse>>> getRatingsReceived(
    String sellerId, {
    int limit = 20,
    int? cursor,
  }) async {
    logger?.info(
      'Getting ratings received for seller: $sellerId (limit: $limit, cursor: $cursor)',
    );

    final queryParams = <String, dynamic>{'limit': limit, 'cursor': cursor};

    return executeListRequest(
      () => apiClient.get(
        '$_usersPath/$sellerId/ratings',
        queryParameters: queryParams,
      ),
      itemParser: RatingApiResponse.fromJson,
    );
  }

  /// Get ratings given by the current user (buyer)
  ///
  /// Endpoint: GET /api/v1/users/me/ratings/given
  ///
  /// Canonical Rating HTTP contract: keyset pagination with a Unix-nanosecond
  /// [cursor] and [limit] page size. The backend returns a BARE rating
  /// collection (snake_case OrderRating fields).
  Future<Result<List<RatingApiResponse>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async {
    logger?.info('Getting ratings given by current user (limit: $limit, cursor: $cursor)');

    final queryParams = <String, dynamic>{'limit': limit, 'cursor': cursor};

    return executeListRequest(
      () => apiClient.get(
        '$_usersPath/me/ratings/given',
        queryParameters: queryParams,
      ),
      itemParser: RatingApiResponse.fromJson,
    );
  }

  /// Get rating summary for a seller
  ///
  /// Endpoint: GET /api/v1/users/{sellerId}/ratings/summary
  Future<Result<RatingSummaryApiResponse>> getRatingSummary(
    String sellerId,
  ) async {
    logger?.info('Getting rating summary for seller: $sellerId');

    return executeRequest(
      () => apiClient.get('$_usersPath/$sellerId/ratings/summary'),
      parser: (data) =>
          RatingSummaryApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ===========================================
  // DEPRECATED: Legacy methods for transition
  // ===========================================

  /// @deprecated Ratings are immutable
  Future<Result<RatingApiResponse>> updateRating(
    String ratingId,
    Map<String, dynamic> request,
  ) async {
    logger?.warning('Update rating called (not supported): $ratingId');
    return Result.error('Ratings are immutable and cannot be updated');
  }

  /// @deprecated Ratings are immutable
  Future<Result<void>> deleteRating(String ratingId) async {
    logger?.warning('Delete rating called (not supported): $ratingId');
    return Result.error('Ratings are immutable and cannot be deleted');
  }

  /// @deprecated Helpful voting not supported
  Future<Result<bool>> toggleRatingHelpful(String ratingId) async {
    logger?.warning('Toggle helpful called (not supported): $ratingId');
    return Result.error('Helpful voting is not supported');
  }
}
