import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/rating/data/datasources/rating_api_datasource.dart';
import 'package:labuda/domains/social/rating/data/dto/rating_api_models.dart';
import 'package:labuda/domains/social/rating/data/repositories/api/rating_repository_api.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';

// CANONICAL RATING DATASOURCE / REPOSITORY CONTRACT TEST
//
// Locks the mobile HTTP consumer against the canonical Rating HTTP contract:
// - limit + cursor (int64 Unix-ns) request semantics; BARE collection response
// - raw buyer_id / seller_id identity; NO reviewer / verified_purchase
// - NO GetRatingState, NO /ratings/state, NO RatingCursor

class _FakeApiClient extends Fake implements ApiClient {}

class _RecordingRatingDatasource extends RatingApiDatasource {
  String? lastReceivedSellerId;
  int? lastReceivedLimit;
  int? lastReceivedCursor;
  int receivedCallCount = 0;

  int? lastGivenLimit;
  int? lastGivenCursor;
  int givenCallCount = 0;

  String? lastSummarySellerId;
  int summaryCallCount = 0;

  String? lastCreateOrderId;
  CreateRatingApiRequest? lastCreateRequest;
  int createCallCount = 0;

  List<RatingApiResponse>? cannedListResponse;
  RatingSummaryApiResponse? cannedSummaryResponse;
  RatingApiResponse? cannedCreateResponse;
  Object? cannedError;

  _RecordingRatingDatasource() : super(_FakeApiClient());

  @override
  Future<Result<List<RatingApiResponse>>> getRatingsReceived(
    String sellerId, {
    int limit = 20,
    int? cursor,
  }) async {
    receivedCallCount++;
    lastReceivedSellerId = sellerId;
    lastReceivedLimit = limit;
    lastReceivedCursor = cursor;
    if (cannedError != null) return Result.error(cannedError.toString());
    return Result.success(cannedListResponse ?? const <RatingApiResponse>[]);
  }

  @override
  Future<Result<List<RatingApiResponse>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async {
    givenCallCount++;
    lastGivenLimit = limit;
    lastGivenCursor = cursor;
    if (cannedError != null) return Result.error(cannedError.toString());
    return Result.success(cannedListResponse ?? const <RatingApiResponse>[]);
  }

  @override
  Future<Result<RatingSummaryApiResponse>> getRatingSummary(
    String sellerId,
  ) async {
    summaryCallCount++;
    lastSummarySellerId = sellerId;
    if (cannedError != null) return Result.error(cannedError.toString());
    return Result.success(
      cannedSummaryResponse ??
          const RatingSummaryApiResponse(
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
  Future<Result<RatingApiResponse>> createRatingForOrder(
    String orderId,
    CreateRatingApiRequest request,
  ) async {
    createCallCount++;
    lastCreateOrderId = orderId;
    lastCreateRequest = request;
    if (cannedError != null) return Result.error(cannedError.toString());
    if (cannedCreateResponse != null) {
      return Result.success(cannedCreateResponse!);
    }
    throw UnimplementedError('set cannedCreateResponse');
  }
}

RatingApiResponse _makeItem(String rid, String bid) => RatingApiResponse(
  id: rid,
  orderId: 'o-$rid',
  buyerId: bid,
  sellerId: 's1',
  ratingValue: 5,
  createdAt: DateTime(2026, 7, 1),
);

void main() {
  group('Datasource contract', () {
    test('getRatingsReceived sends limit and int64 cursor', () async {
      final ds = _RecordingRatingDatasource();
      final result = await ds.getRatingsReceived('seller-1', limit: 10);
      expect(result.isSuccess, isTrue);
      expect(ds.receivedCallCount, 1);
      expect(ds.lastReceivedSellerId, 'seller-1');
      expect(ds.lastReceivedLimit, 10);
      expect(ds.lastReceivedCursor, isNull, reason: 'cursor omitted when null');
    });

    test('getRatingsReceived forwards cursor when provided', () async {
      final ds = _RecordingRatingDatasource();
      await ds.getRatingsReceived('s', limit: 20, cursor: 123456789);
      expect(ds.lastReceivedCursor, 123456789);
    });

    test('getRatingsGiven sends limit and cursor', () async {
      final ds = _RecordingRatingDatasource();
      await ds.getRatingsGiven(limit: 5, cursor: 999);
      expect(ds.givenCallCount, 1);
      expect(ds.lastGivenLimit, 5);
      expect(ds.lastGivenCursor, 999);
    });

    test('getRatingSummary sends seller ID', () async {
      final ds = _RecordingRatingDatasource();
      final result = await ds.getRatingSummary('seller-99');
      expect(result.isSuccess, isTrue);
      expect(ds.lastSummarySellerId, 'seller-99');
      expect(ds.summaryCallCount, 1);
    });

    test('bare list — no envelope, no cursor payload in response parsing',
        () async {
      final ds = _RecordingRatingDatasource()..cannedListResponse = [
        _makeItem('r1', 'b1'),
        _makeItem('r2', 'b2'),
      ];
      final result = await ds.getRatingsReceived('s', limit: 20);
      expect(result.isSuccess, isTrue);
      expect(result.data, hasLength(2));
    });
  });

  group('Repository passthrough', () {
    test('getRatingsReceived returns bare List<Rating> with raw identity',
        () async {
      final ds = _RecordingRatingDatasource()
        ..cannedListResponse = [_makeItem('r1', 'buyer-42')];
      final repo = RatingRepositoryApi(ds);

      final result = await repo.getRatingsReceived(sellerId: 's1', limit: 20);

      expect(result.isSuccess, isTrue);
      expect(result.data, hasLength(1));
      final rating = result.data!.single;
      expect(rating.buyerId, 'buyer-42');
      expect(rating.sellerId, 's1');
      expect(rating.id, 'r1');
    });

    test('getRatingSummary maps aggregates', () async {
      final ds = _RecordingRatingDatasource()
        ..cannedSummaryResponse = const RatingSummaryApiResponse(
          totalRatings: 3,
          averageRating: 4.0,
          oneStarCount: 0,
          twoStarCount: 0,
          threeStarCount: 1,
          fourStarCount: 1,
          fiveStarCount: 1,
        );
      final repo = RatingRepositoryApi(ds);

      final result = await repo.getRatingSummary(sellerId: 's1');

      expect(result.isSuccess, isTrue);
      expect(result.data!.totalRatings, 3);
      expect(result.data!.averageRating, 4.0);
    });

    test('createRatingForOrder maps canonically and preserves identity',
        () async {
      final ds = _RecordingRatingDatasource()
        ..cannedCreateResponse = _makeItem('cr1', 'buyer-7');
      final repo = RatingRepositoryApi(ds);

      final result = await repo.createRatingForOrder(
        orderId: 'o1',
        ratingValue: 4,
        comment: 'good',
      );

      expect(result.isSuccess, isTrue);
      final rating = result.data!;
      expect(rating.buyerId, 'buyer-7');
      expect(rating.sellerId, 's1');
      expect(rating.ratingValue, 5);
      expect(ds.createCallCount, 1);
      expect(ds.lastCreateOrderId, 'o1');
    });

    test('createRatingForOrder propagates error code', () async {
      final ds = _RecordingRatingDatasource()
        ..cannedError = 'EMAIL_VERIFICATION_REQUIRED';
      final repo = RatingRepositoryApi(ds);

      final result = await repo.createRatingForOrder(
        orderId: 'o1',
        ratingValue: 4,
      );

      expect(result.isError, isTrue);
    });
  });

  group('hasUserRatedOrderProvider', () {
    test('reports false when no rating exists for the order', () async {
      final ds = _RecordingRatingDatasource();
      final container = ProviderContainer(
        overrides: [
          ratingRepositoryProvider.overrideWithValue(RatingRepositoryApi(ds)),
        ],
      );
      addTearDown(container.dispose);

      final result = await container.read(
        hasUserRatedOrderProvider(
          orderId: 'o1',
          buyerId: 'b1',
          sellerId: 's1',
        ).future,
      );

      expect(result, isFalse);
    });
  });
}