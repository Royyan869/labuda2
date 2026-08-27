import 'dart:collection';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/repositories/auction_repository_impl.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _NoopLogger implements ILoggerService {
  Future<Result<void>> _ok() async => Result.success(null);

  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      _ok();

  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      _ok();

  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      _ok();

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => _ok();

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) => _ok();

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) => _ok();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) => _ok();

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) => _ok();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) => _ok();

  @override
  Future<Result<void>> clearLogs() => _ok();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const []);

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _NoopAuctionRemoteDatasource extends AuctionRemoteDatasource {
  _NoopAuctionRemoteDatasource()
    : super(
        ApiClient.testing(baseUrl: 'https://example.com/api/v1'),
        logger: _NoopLogger(),
      );
}

class _ScriptedAuctionRepository extends AuctionRepositoryImpl {
  _ScriptedAuctionRepository({
    required this.activeResults,
    required this.detailResults,
    required this.bidResults,
  }) : super(datasource: _NoopAuctionRemoteDatasource(), logger: _NoopLogger());

  final Queue<Future<RepositoryResult<List<Auction>>> Function()> activeResults;
  final Queue<Future<RepositoryResult<Auction>> Function()> detailResults;
  final Queue<Future<RepositoryResult<List<AuctionBid>>> Function()> bidResults;

  int activeCalls = 0;
  int detailCalls = 0;
  int bidCalls = 0;

  @override
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) {
    activeCalls += 1;
    return activeResults.removeFirst()();
  }

  @override
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId) {
    detailCalls += 1;
    return detailResults.removeFirst()();
  }

  @override
  Future<RepositoryResult<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) {
    bidCalls += 1;
    return bidResults.removeFirst()();
  }
}

Auction _auction({
  required String id,
  required int currentBid,
  required AuctionStatus status,
  required String mediaUrl,
}) {
  return Auction(
    id: id,
    sellerId: 'seller-1',
    sellerUsername: 'yayan',
    sellerFarmName: 'Farm Koi Nusantara',
    title: 'Sanke Auction',
    description: 'Live auction',
    media: [
      MediaEntity(
        id: auctionMediaLogicalKey(
          auctionId: id,
          mediaReference: mediaUrl,
          position: 0,
        ),
        originalUrl: mediaUrl,
        type: MediaType.image,
        createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
      ),
    ],
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 0,
      ageInMonths: 0,
      gender: 'unknown',
      certificates: [],
    ),
    openingBid: 1000000,
    currentBid: currentBid,
    bidIncrement: 50000,
    startTime: DateTime.parse('2026-01-01T00:00:00.000Z'),
    endTime: DateTime.parse('2026-01-02T00:00:00.000Z'),
    status: status,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    updatedAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
  );
}

AuctionBid _bid({
  required String id,
  required int amount,
  required String bidderId,
}) {
  return AuctionBid(
    id: id,
    auctionId: 'auction-1',
    bidderId: bidderId,
    bidderUsername: bidderId,
    amount: amount,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    isWinning: true,
    isOutbid: false,
  );
}

void main() {
  group('AuctionRepository polling retention', () {
    test(
      'watchActiveAuctions suppresses duplicate snapshots and retains last data on transient error',
      () {
        fakeAsync((async) {
          final repo = _ScriptedAuctionRepository(
            activeResults: Queue.of([
              () => Future.value(
                RepositoryResult.success(<Auction>[
                  _auction(
                    id: 'auction-1',
                    currentBid: 1000000,
                    status: AuctionStatus.active,
                    mediaUrl:
                        'https://cdn.example.com/auctions/a-1.jpg?X-Amz-Signature=one',
                  ),
                ]),
              ),
              () => Future.value(
                RepositoryResult.success(<Auction>[
                  _auction(
                    id: 'auction-1',
                    currentBid: 1000000,
                    status: AuctionStatus.active,
                    mediaUrl:
                        'https://cdn.example.com/auctions/a-1.jpg?X-Amz-Signature=two',
                  ),
                ]),
              ),
              () =>
                  Future.value(RepositoryResult.error('transient list error')),
            ]),
            detailResults: Queue.of([]),
            bidResults: Queue.of([]),
          );

          final events = <List<Auction>>[];
          final errors = <Object>[];
          final sub = repo
              .watchActiveAuctions(limit: 1)
              .listen(events.add, onError: errors.add);

          async.flushMicrotasks();

          expect(repo.activeCalls, 1);
          expect(events, hasLength(1));
          expect(errors, isEmpty);

          async.elapse(const Duration(seconds: 30));
          async.flushMicrotasks();

          expect(repo.activeCalls, 2);
          expect(events, hasLength(1));
          expect(errors, isEmpty);

          async.elapse(const Duration(seconds: 30));
          async.flushMicrotasks();

          expect(repo.activeCalls, 3);
          expect(events, hasLength(1));
          expect(errors, isEmpty);

          sub.cancel();
          repo.dispose();
        });
      },
    );

    test(
      'watchAuction publishes meaningful status/media refreshes while suppressing identical retries',
      () {
        fakeAsync((async) {
          final repo = _ScriptedAuctionRepository(
            activeResults: Queue.of([]),
            detailResults: Queue.of([
              () => Future.value(
                RepositoryResult.success(
                  _auction(
                    id: 'auction-1',
                    currentBid: 1000000,
                    status: AuctionStatus.active,
                    mediaUrl:
                        'https://cdn.example.com/auctions/a-1.jpg?X-Amz-Signature=one',
                  ),
                ),
              ),
              () => Future.value(
                RepositoryResult.success(
                  _auction(
                    id: 'auction-1',
                    currentBid: 1500000,
                    status: AuctionStatus.active,
                    mediaUrl:
                        'https://cdn.example.com/auctions/a-1.jpg?X-Amz-Signature=two',
                  ),
                ),
              ),
              () => Future.value(
                RepositoryResult.error('transient detail error'),
              ),
            ]),
            bidResults: Queue.of([]),
          );

          final events = <Auction?>[];
          final errors = <Object>[];
          final sub = repo
              .watchAuction('auction-1')
              .listen(events.add, onError: errors.add);

          async.flushMicrotasks();

          expect(repo.detailCalls, 1);
          expect(events, hasLength(1));
          expect(events.single?.currentBid, 1000000);
          expect(errors, isEmpty);

          for (var i = 0; i < 30 && repo.detailCalls < 2; i++) {
            async.elapse(const Duration(seconds: 1));
            async.flushMicrotasks();
            async.flushMicrotasks();
          }

          expect(repo.detailCalls, 2);
          expect(events, hasLength(2));
          expect(events.last?.currentBid, 1500000);
          expect(errors, isEmpty);

          for (var i = 0; i < 30 && repo.detailCalls < 3; i++) {
            async.elapse(const Duration(seconds: 1));
            async.flushMicrotasks();
            async.flushMicrotasks();
          }

          expect(repo.detailCalls, 3);
          expect(events, hasLength(2));
          expect(errors, isEmpty);

          sub.cancel();
          repo.dispose();
        });
      },
    );

    test(
      'watchAuctionBids emits refreshed bid history without duplicate churn',
      () {
        fakeAsync((async) {
          final repo = _ScriptedAuctionRepository(
            activeResults: Queue.of([]),
            detailResults: Queue.of([]),
            bidResults: Queue.of([
              () => Future.value(
                RepositoryResult.success(<AuctionBid>[
                  _bid(id: 'bid-1', amount: 1000000, bidderId: 'bidder-1'),
                ]),
              ),
              () => Future.value(
                RepositoryResult.success(<AuctionBid>[
                  _bid(id: 'bid-1', amount: 1000000, bidderId: 'bidder-1'),
                ]),
              ),
              () => Future.value(
                RepositoryResult.success(<AuctionBid>[
                  _bid(id: 'bid-1', amount: 1500000, bidderId: 'bidder-2'),
                ]),
              ),
            ]),
          );

          final events = <List<AuctionBid>>[];
          final errors = <Object>[];
          final sub = repo
              .watchAuctionBids('auction-1', limit: 1)
              .listen(events.add, onError: errors.add);

          async.flushMicrotasks();

          expect(repo.bidCalls, 1);
          expect(events, hasLength(1));
          expect(events.single.single.amount, 1000000);
          expect(errors, isEmpty);

          for (var i = 0; i < 6 && repo.bidCalls < 2; i++) {
            async.elapse(const Duration(seconds: 10));
            async.flushMicrotasks();
            async.flushMicrotasks();
          }

          expect(repo.bidCalls, 2);
          expect(events, hasLength(1));
          expect(errors, isEmpty);

          for (var i = 0; i < 6 && repo.bidCalls < 3; i++) {
            async.elapse(const Duration(seconds: 10));
            async.flushMicrotasks();
            async.flushMicrotasks();
          }

          expect(repo.bidCalls, 3);
          expect(events, hasLength(2));
          expect(events.last.single.amount, 1500000);
          expect(errors, isEmpty);

          sub.cancel();
          repo.dispose();
        });
      },
    );
  });
}
