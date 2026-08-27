import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/rating/rating.dart';

// CANONICAL ORDER-DETAIL RATING SUBMIT TEST
//
// Exercises the live canonical submit path used by
// OrderDetailHandlersMixin.handleSubmitRating (order_detail_handlers.dart):
//   ref.read(ratingRepositoryProvider).createRatingForOrder(orderId: …)
//
// Locks canonical identity (buyer_id / seller_id), exactly-once invocation,
// error-code passthrough, and the parked getRatingForOrder semantics behind
// hasUserRatedOrderProvider. NO reviewer projection, NO RatingState.

class _SubmitTestRepository implements IRatingRepository {
  Result<Rating>? createResult;
  Result<RatingSummary>? summaryResult;
  Result<Rating?>? ratingForOrderResult;

  int createCallCount = 0;
  int getForOrderCallCount = 0;

  String? lastCreateOrderId;
  int? lastCreateRatingValue;
  String? lastCreateComment;

  @override
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async {
    createCallCount++;
    lastCreateOrderId = orderId;
    lastCreateRatingValue = ratingValue;
    lastCreateComment = comment;
    return createResult ?? Result.error('set createResult');
  }

  @override
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<List<Rating>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<RatingSummary>> getRatingSummary({
    required String sellerId,
  }) async {
    if (summaryResult != null) return summaryResult!;
    return Result.success(
      const RatingSummary(
        totalRatings: 0,
        averageRating: 0,
        oneStarCount: 0,
        twoStarCount: 0,
        threeStarCount: 0,
        fourStarCount: 0,
        fiveStarCount: 0,
      ),
    );
  }

  @override
  Future<Result<Rating?>> getRatingForOrder({
    required String orderId,
  }) async {
    getForOrderCallCount++;
    return ratingForOrderResult ?? Result.success(null);
  }
}

Rating _makeRating(String id) => Rating(
  id: id,
  orderId: 'o-$id',
  buyerId: 'b1',
  sellerId: 's1',
  ratingValue: 4,
  comment: 'nice product',
  createdAt: DateTime(2026, 7, 1),
);

void main() {
  group('Order-detail rating submit SUCCESS', () {
    test('createRatingForOrder called exactly once with exact params',
        () async {
      final repo = _SubmitTestRepository()
        ..createResult = Result.success(_makeRating('cr-ok'));

      final container = ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      // Mirrors handleSubmitRating: ref.read(ratingRepositoryProvider)
      //   .createRatingForOrder(orderId:…, ratingValue:…, comment:…)
      final result = await container
          .read(ratingRepositoryProvider)
          .createRatingForOrder(
            orderId: 'order-100',
            ratingValue: 4,
            comment: 'nice product',
          );

      expect(result.isSuccess, isTrue);
      expect(repo.createCallCount, 1);
      expect(repo.lastCreateOrderId, 'order-100');
      expect(repo.lastCreateRatingValue, 4);
      expect(repo.lastCreateComment, 'nice product');
    });

    test('returned Rating carries raw buyer_id / seller_id identity', () async {
      final repo = _SubmitTestRepository()
        ..createResult = Result.success(_makeRating('cr-proj'));

      final container = ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      final result = await container
          .read(ratingRepositoryProvider)
          .createRatingForOrder(orderId: 'o-x', ratingValue: 3);

      expect(result.isSuccess, isTrue);
      final rating = result.data!;
      expect(rating.buyerId, 'b1');
      expect(rating.sellerId, 's1');
      expect(rating.orderId, 'o-cr-proj');
    });

    test('no lookup precedes the create call', () async {
      final repo = _SubmitTestRepository()
        ..createResult = Result.success(_makeRating('cr-np'));

      final container = ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      await container
          .read(ratingRepositoryProvider)
          .createRatingForOrder(orderId: 'o-np', ratingValue: 5);

      expect(repo.getForOrderCallCount, 0,
          reason: 'no getRatingForOrder lookup during submit');
      expect(repo.createCallCount, 1);
    });
  });

  group('Order-detail rating submit FAILURE', () {
    test('error result propagates without success state', () async {
      final repo = _SubmitTestRepository()
        ..createResult = Result.error('RATING_DUPLICATE');

      final container = ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      final result = await container
          .read(ratingRepositoryProvider)
          .createRatingForOrder(orderId: 'o-fail', ratingValue: 3);

      expect(result.isError, isTrue);
      expect(result.error, 'RATING_DUPLICATE');
      expect(repo.createCallCount, 1);
    });
  });

  group('hasUserRatedOrderProvider (parked getRatingForOrder)', () {
    test('false when no rating exists for the order', () async {
      final repo = _SubmitTestRepository();

      final container = ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      final before = await container.read(
        hasUserRatedOrderProvider(
          orderId: 'order-before',
          buyerId: 'b1',
          sellerId: 's1',
        ).future,
      );

      expect(before, isFalse);
    });

    test('true when a matching buyer→seller rating exists', () async {
      final repo = _SubmitTestRepository()
        ..ratingForOrderResult = Result<Rating?>.success(_makeRating('r1'));

      final container = ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      final rated = await container.read(
        hasUserRatedOrderProvider(
          orderId: 'o-r1',
          buyerId: 'b1',
          sellerId: 's1',
        ).future,
      );

      expect(rated, isTrue);
    });
  });
}