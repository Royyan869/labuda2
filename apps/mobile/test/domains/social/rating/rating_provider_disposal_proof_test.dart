import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/rating/rating.dart';

// CANONICAL RATING PROVIDER DISPOSAL TEST
//
// Exercises the live canonical `ratingProvider` (Riverpod Notifier):
// - invalidate()/dispose() while a load is in-flight must not leak the late
//   completion into a fresh instance (generation isolation).
// - State surface is the canonical RatingListState (ratings/isLoading/error/
//   summary) — no cursor/opaque pagination surface exists.

class _DeferredRatingRepository implements IRatingRepository {
  Completer<Result<List<Rating>>>? receivedCompleter;
  Completer<Result<RatingSummary>>? summaryCompleter;
  Completer<Result<Rating>>? createCompleter;

  int receivedCallCount = 0;
  String? lastReceivedSellerId;

  @override
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  }) async {
    receivedCallCount++;
    lastReceivedSellerId = sellerId;
    if (receivedCompleter != null) return receivedCompleter!.future;
    return Result.success(const <Rating>[]);
  }

  @override
  Future<Result<List<Rating>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<RatingSummary>> getRatingSummary({
    required String sellerId,
  }) async {
    if (summaryCompleter != null) return summaryCompleter!.future;
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
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async {
    if (createCompleter != null) return createCompleter!.future;
    return Result.error('Not used');
  }

  @override
  Future<Result<Rating?>> getRatingForOrder({required String orderId}) async =>
      Result.success(null);
}

Rating _makeRating(String id) => Rating(
  id: id,
  orderId: 'o-$id',
  buyerId: 'b-$id',
  sellerId: 's1',
  ratingValue: 5,
  createdAt: DateTime(2026, 7, 1),
);

void main() {
  group('RatingProvider disposal / isolation', () {
    ProviderContainer createTestContainer(_DeferredRatingRepository repo) {
      return ProviderContainer(
        overrides: [ratingRepositoryProvider.overrideWithValue(repo)],
      );
    }

    test('invalidate() while in-flight discards the late result', () async {
      final repo = _DeferredRatingRepository();
      final receivedCompleter = Completer<Result<List<Rating>>>();
      repo.receivedCompleter = receivedCompleter;

      final container = createTestContainer(repo);
      final asyncErrors = <Object>[];

      await runZonedGuarded(() async {
        final loadFuture = container
            .read(ratingProvider.notifier)
            .loadUserRatings(userId: 'seller-1', isReceived: true);

        expect(container.read(ratingProvider).isLoading, isTrue);
        expect(repo.receivedCallCount, 1);
        expect(repo.lastReceivedSellerId, 'seller-1');

        container.invalidate(ratingProvider);

        receivedCompleter.complete(Result.success([_makeRating('r-old')]));

        await loadFuture;
        await Future<void>.delayed(Duration.zero);

        final newState = container.read(ratingProvider);
        expect(newState.ratings, isEmpty,
            reason: 'old instance result must not appear in new instance');
        expect(newState.isLoading, isFalse);
        expect(newState.error, isNull);
        expect(newState.summary, isNull);

        final newNotifier = container.read(ratingProvider.notifier);
        repo.receivedCompleter = null;
        await newNotifier.loadUserRatings(
          userId: 'seller-2',
          isReceived: true,
        );
        final afterNewLoad = container.read(ratingProvider);
        expect(afterNewLoad.ratings.where((r) => r.id == 'r-old'), isEmpty,
            reason: 'old generation items must not leak');
        expect(repo.lastReceivedSellerId, 'seller-2');
      }, (error, stack) {
        asyncErrors.add(error);
      });

      await Future<void>.delayed(const Duration(milliseconds: 100));
      expect(asyncErrors, isEmpty);
    });

    test('container.dispose() while in-flight does not throw uncaught',
        () async {
      final repo = _DeferredRatingRepository();
      final receivedCompleter = Completer<Result<List<Rating>>>();
      repo.receivedCompleter = receivedCompleter;

      final container = createTestContainer(repo);
      final asyncErrors = <Object>[];

      await runZonedGuarded(() async {
        final loadFuture = container
            .read(ratingProvider.notifier)
            .loadUserRatings(userId: 'seller-1', isReceived: true);

        expect(container.read(ratingProvider).isLoading, isTrue);

        container.dispose();

        if (!receivedCompleter.isCompleted) {
          receivedCompleter.complete(Result.success([_makeRating('r-gone')]));
        }

        try {
          await loadFuture;
        } catch (_) {
          // Expected after container disposal.
        }

        await Future<void>.delayed(Duration.zero);
      }, (error, stack) {
        asyncErrors.add(error);
      });

      await Future<void>.delayed(const Duration(milliseconds: 100));
      expect(asyncErrors, isEmpty);
    });

    test('disposed provider state is not resurrected', () async {
      final repo = _DeferredRatingRepository();

      final container1 = createTestContainer(repo);
      final receivedCompleter1 = Completer<Result<List<Rating>>>();
      repo.receivedCompleter = receivedCompleter1;
      final loadFuture1 = container1
          .read(ratingProvider.notifier)
          .loadUserRatings(userId: 'seller-1', isReceived: true);
      receivedCompleter1.complete(Result.success([_makeRating('r-old')]));
      await loadFuture1;

      expect(container1.read(ratingProvider).ratings, isNotEmpty);

      container1.dispose();

      final container2 = createTestContainer(repo);
      addTearDown(container2.dispose);

      final newState = container2.read(ratingProvider);
      expect(newState.ratings, isEmpty, reason: 'disposed state resurrected');
      expect(newState.isLoading, isFalse);
      expect(newState.error, isNull);
      expect(newState.summary, isNull);
    });

    test('new instance issues a fresh load with its own params', () async {
      final repo = _DeferredRatingRepository();

      final container = createTestContainer(repo);
      addTearDown(container.dispose);

      final completer1 = Completer<Result<List<Rating>>>();
      repo.receivedCompleter = completer1;
      final loadFuture = container
          .read(ratingProvider.notifier)
          .loadUserRatings(userId: 'seller-1', isReceived: true);
      completer1.complete(Result.success([_makeRating('r1')]));
      await loadFuture;

      expect(container.read(ratingProvider).ratings, isNotEmpty);

      container.invalidate(ratingProvider);

      final freshState = container.read(ratingProvider);
      expect(freshState.ratings, isEmpty);

      final oldCallCount = repo.receivedCallCount;
      repo.receivedCompleter = null;
      await container
          .read(ratingProvider.notifier)
          .loadUserRatings(userId: 'seller-2', isReceived: true);

      expect(repo.receivedCallCount, oldCallCount + 1);
      expect(repo.lastReceivedSellerId, 'seller-2');
      expect(container.read(ratingProvider).isLoading, isFalse);
    });
  });
}