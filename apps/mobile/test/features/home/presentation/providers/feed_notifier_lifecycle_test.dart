import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/domain/entities/feed_page.dart';
import 'package:labuda/features/home/domain/repositories/home_repository.dart';
import 'package:labuda/features/home/presentation/providers/feed/feed_notifier.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _CountingHomeRepository implements HomeRepository {
  int getFeedPageCalls = 0;

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    getFeedPageCalls++;
    return const FeedPage(items: <FeedItem>[], hasMore: false);
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) {
    return const Stream<List<FeedItem>>.empty();
  }

  @override
  Future<void> refreshFeedItems() async {}

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

void main() {
  test(
    'FeedNotifier survives repeated invalidation on the same provider lifecycle',
    () async {
      final feedRepository = _CountingHomeRepository();
      final container = ProviderContainer(
        overrides: [
          homeRepositoryProvider.overrideWithValue(feedRepository),
          loggerServiceProvider.overrideWithValue(LoggerService.instance),
          authControllerProvider.overrideWith(_FakeAuthController.new),
        ],
      );
      addTearDown(container.dispose);

      final subscription = container.listen(
        feedProvider,
        (previous, next) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      container.invalidate(feedProvider);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      container.invalidate(feedProvider);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      final state = container.read(feedProvider);
      expect(feedRepository.getFeedPageCalls, 3);
      expect(state.errorMessage, isNull);
      expect(state.isLoading, isFalse);
      expect(state.items, isEmpty);
      expect(state.hasReachedMax, isTrue);
    },
  );
}
