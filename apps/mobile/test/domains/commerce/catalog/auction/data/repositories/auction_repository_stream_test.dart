import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/repositories/auction_repository_impl.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
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
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) => _ok();

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

class _ProbeAuctionRepository extends AuctionRepositoryImpl {
  _ProbeAuctionRepository()
    : super(
        datasource: _NoopAuctionRemoteDatasource(),
        logger: _NoopLogger(),
      );

  int activeCalls = 0;
  int userCalls = 0;
  int? lastUserLimit;
  Completer<RepositoryResult<List<Auction>>>? activeCompleter;
  Completer<RepositoryResult<List<Auction>>>? userCompleter;

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
    return activeCompleter!.future;
  }

  @override
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) {
    userCalls += 1;
    lastUserLimit = limit;
    return userCompleter!.future;
  }
}

Auction _auction() {
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: 'qiqijho',
    sellerFarmName: 'Qiqi Store',
    title: 'test',
    description: 'desc',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 50,
      ageInMonths: 12,
      gender: 'male',
    ),
    openingBid: 100000,
    currentBid: 100000,
    bidIncrement: 50000,
    startTime: DateTime.utc(2026, 7, 26),
    endTime: DateTime.utc(2026, 7, 31),
    status: AuctionStatus.active,
    createdAt: DateTime.utc(2026, 7, 26),
    updatedAt: DateTime.utc(2026, 7, 26),
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
  );
}

void main() {
  group('AuctionRepository stream discovery', () {
    test('watchActiveAuctions fetches immediately on listen', () async {
      final repo = _ProbeAuctionRepository()
        ..activeCompleter = Completer<RepositoryResult<List<Auction>>>();
      final values = <List<Auction>>[];
      final errors = <Object>[];
      final sub = repo.watchActiveAuctions(limit: 2).listen(
            values.add,
            onError: errors.add,
          );
      addTearDown(sub.cancel);

      await Future<void>.delayed(Duration.zero);

      expect(repo.activeCalls, 1);
      expect(values, isEmpty);
      expect(errors, isEmpty);

      repo.activeCompleter!.complete(
        RepositoryResult.success(<Auction>[_auction()]),
      );

      await Future<void>.delayed(Duration.zero);

      expect(values, hasLength(1));
      expect(values.single, hasLength(1));
      expect(values.single.single.id, 'auction-1');
      expect(errors, isEmpty);
    });

    test('watchUserAuctions fetches immediately on listen', () async {
      final repo = _ProbeAuctionRepository()
        ..userCompleter = Completer<RepositoryResult<List<Auction>>>();
      final values = <List<Auction>>[];
      final errors = <Object>[];
      final sub = repo.watchUserAuctions(
        sellerId: 'seller-1',
        limit: 2,
      ).listen(
        values.add,
        onError: errors.add,
      );
      addTearDown(sub.cancel);

      await Future<void>.delayed(Duration.zero);

      expect(repo.userCalls, 1);
      expect(values, isEmpty);
      expect(errors, isEmpty);

      repo.userCompleter!.complete(
        RepositoryResult.success(<Auction>[_auction()]),
      );

      await Future<void>.delayed(Duration.zero);

      expect(values, hasLength(1));
      expect(values.single, hasLength(1));
      expect(values.single.single.sellerUsername, 'qiqijho');
      expect(errors, isEmpty);
    });

    test('watchUserAuctions defaults seller feeds to 50', () async {
      final repo = _ProbeAuctionRepository()
        ..userCompleter = Completer<RepositoryResult<List<Auction>>>();
      final sub = repo.watchUserAuctions(sellerId: 'seller-1').listen(
            (_) {},
          );
      addTearDown(sub.cancel);

      await Future<void>.delayed(Duration.zero);

      expect(repo.userCalls, 1);
      expect(repo.lastUserLimit, 50);

      repo.userCompleter!.complete(RepositoryResult.success(const <Auction>[]));
      await Future<void>.delayed(Duration.zero);
    });

    test('stream failures surface as errors instead of empty data', () async {
      final repo = _ProbeAuctionRepository()
        ..userCompleter = Completer<RepositoryResult<List<Auction>>>();
      final values = <List<Auction>>[];
      final errors = <Object>[];
      final sub = repo.watchUserAuctions(
        sellerId: 'seller-1',
      ).listen(
        values.add,
        onError: errors.add,
      );
      addTearDown(sub.cancel);

      await Future<void>.delayed(Duration.zero);

      expect(repo.userCalls, 1);

      repo.userCompleter!.complete(RepositoryResult.error('boom'));

      await Future<void>.delayed(Duration.zero);

      expect(values, isEmpty);
      expect(errors, hasLength(1));
      expect(errors.single, isA<StateError>());
    });
  });
}
