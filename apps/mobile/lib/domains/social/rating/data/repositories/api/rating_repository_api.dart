import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/social/rating/domain/repositories/i_rating_repository.dart';
import 'package:labuda/domains/social/rating/data/datasources/rating_api_datasource.dart';
import 'package:labuda/domains/social/rating/data/dto/rating_api_models.dart';
import 'package:labuda/domains/social/rating/data/mappers/rating_api_mapper.dart';

/// CANONICAL Rating Repository Implementation
///
/// Implements IRatingRepository aligned with backend contract.
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no update/delete after submission)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings
/// - Single score 1-5 (int, not double)
/// - Optional comment text (no media, no helpful voting)
class RatingRepositoryApi implements IRatingRepository {
  final RatingApiDatasource _datasource;
  final ILoggerService? _logger;

  RatingRepositoryApi(this._datasource, {ILoggerService? logger})
    : _logger = logger;

  // ===========================================
  // CANONICAL METHODS
  // ===========================================

  @override
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async {
    _logger?.info('Creating rating for order: $orderId');

    final request = CreateRatingApiRequest(
      ratingValue: ratingValue,
      comment: comment,
    );

    final result = await _datasource.createRatingForOrder(orderId, request);

    if (result.isError) {
      // Preserve the API code (e.g. EMAIL_VERIFICATION_REQUIRED) so the
      // call site can react via Result.errorCode.
      return Result.error(
        result.error ?? 'Unknown error',
        code: result.errorCode,
      );
    }
    return Result.success(RatingApiMapper.toRating(result.data!));
  }

  @override
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  }) async {
    _logger?.info('Getting ratings received for seller: $sellerId');

    final result = await _datasource.getRatingsReceived(
      sellerId,
      limit: limit,
      cursor: cursor,
    );

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(RatingApiMapper.toRatingList(response)),
    );
  }

  @override
  Future<Result<List<Rating>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async {
    _logger?.info('Getting ratings given by current user');

    final result = await _datasource.getRatingsGiven(
      limit: limit,
      cursor: cursor,
    );

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(RatingApiMapper.toRatingList(response)),
    );
  }

  @override
  Future<Result<RatingSummary>> getRatingSummary({
    required String sellerId,
  }) async {
    _logger?.info('Getting rating summary for seller: $sellerId');

    final result = await _datasource.getRatingSummary(sellerId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(RatingApiMapper.toRatingSummary(response)),
    );
  }

  @override
  Future<Result<Rating?>> getRatingForOrder({required String orderId}) async {
    // PARKED V1: Backend has no GET /orders/:id/ratings route; this remains
    // null until rating detail endpoint is designed.
    return Result.success(null);
  }
}
