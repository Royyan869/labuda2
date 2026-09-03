import 'dart:async';
import 'dart:collection';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/follow/presentation/providers/follow_status_provider.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _MutableAuthController extends AuthController {
  _MutableAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setStateValue(AuthState next) {
    _state = next;
    state = next;
  }
}

class _ControlledFollowRepository implements IFollowRepository {
  final Map<String, Queue<Completer<Result<bool>>>> _statusQueues = {};
  final Map<String, Queue<Completer<Result<bool>>>> _followQueues = {};
  final Map<String, Queue<Completer<Result<bool>>>> _unfollowQueues = {};

  int checkCalls = 0;
  int followCalls = 0;
  int unfollowCalls = 0;

  Completer<Result<bool>> enqueueStatus(String followingId) {
    final queue = _statusQueues.putIfAbsent(followingId, Queue.new);
    final completer = Completer<Result<bool>>();
    queue.addLast(completer);
    return completer;
  }

  Completer<Result<bool>> enqueueFollow(String followingId) {
    final queue = _followQueues.putIfAbsent(followingId, Queue.new);
    final completer = Completer<Result<bool>>();
    queue.addLast(completer);
    return completer;
  }

  Completer<Result<bool>> enqueueUnfollow(String followingId) {
    final queue = _unfollowQueues.putIfAbsent(followingId, Queue.new);
    final completer = Completer<Result<bool>>();
    queue.addLast(completer);
    return completer;
  }

  void completeNextStatus(String followingId, Result<bool> result) {
    _statusQueues[followingId]!.removeFirst().complete(result);
  }

  int pendingStatusCount(String followingId) {
    return _statusQueues[followingId]?.length ?? 0;
  }

  void completeStatusAt(String followingId, int index, Result<bool> result) {
    final queue = _statusQueues[followingId]!;
    final completer = queue.elementAt(index);
    final nextQueue = Queue<Completer<Result<bool>>>.from(queue.toList());
    nextQueue.remove(completer);
    _statusQueues[followingId] = nextQueue;
    completer.complete(result);
  }

  void completeNextFollow(String followingId, Result<bool> result) {
    _followQueues[followingId]!.removeFirst().complete(result);
  }

  void completeNextUnfollow(String followingId, Result<bool> result) {
    _unfollowQueues[followingId]!.removeFirst().complete(result);
  }

  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async {
    checkCalls += 1;
    return enqueueStatus(followingId).future;
  }

  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  }) async {
    followCalls += 1;
    return enqueueFollow(followingId).future;
  }

  @override
  Future<Result<List<FollowableUser>>> getFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async => Result.success(const <FollowableUser>[]);

  @override
  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  }) async => Result.success(
    FollowStats(
      userId: userId,
      followersCount: 0,
      followingCount: 0,
      lastUpdated: DateTime.utc(2026, 7, 31),
    ),
  );

  @override
  Future<Result<List<FollowableUser>>> getFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async => Result.success(const <FollowableUser>[]);

  @override
  Future<Result<bool>> muteUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<List<FollowableUser>>> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  }) async => Result.success(const <FollowableUser>[]);

  @override
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async {
    unfollowCalls += 1;
    return enqueueUnfollow(followingId).future;
  }

  @override
  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> unmuteUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Stream<List<FollowActivity>> watchFollowActivities(String userId) =>
      Stream.value(const <FollowActivity>[]);

  @override
  Stream<FollowStats> watchFollowStats(String userId) => Stream.value(
    FollowStats(
      userId: userId,
      followersCount: 0,
      followingCount: 0,
      lastUpdated: DateTime.utc(2026, 7, 31),
    ),
  );

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) =>
      Stream.value(const <FollowableUser>[]);

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) =>
      Stream.value(const <FollowableUser>[]);
}

class _FakeContentRepository implements ContentRepository {
  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async => ContentRepositoryResult.success(
    const ContentAuthorPage(
      items: <Content>[],
      hasMore: false,
      nextCursor: null,
    ),
  );

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeRatingRepository implements IRatingRepository {
  @override
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async => Result.error('Not used');

  @override
  Future<Result<List<Rating>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<RatingSummary>> getRatingSummary({
    required String sellerId,
  }) async => Result.success(
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

  @override
  Future<Result<Rating?>> getRatingForOrder({
    required String orderId,
  }) async => Result.success(null);
}

AuthUser _authUser({required String id, required String username}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 7, 31),
    updatedAt: DateTime.utc(2026, 7, 31),
    email: '$username@example.com',
    username: username,
    avatarUrl: 'https://example.com/$username.png',
    bio: 'Bio for $username',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

AuthState _authenticatedState({required String id, required String username}) {
  return AuthState.authenticated(
    _authUser(id: id, username: username),
    emailVerified: true,
  );
}

ProfileEntity _profileEntity({required String userId}) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    location: 'Bandung, West Java',
    coverPhotoUrl: 'https://example.com/$userId-cover.jpg',
    joinedAt: DateTime.utc(2026, 7, 1),
    lastActiveAt: DateTime.utc(2026, 7, 31),
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: const UserVerificationInfo(
      isPhoneVerified: false,
      isEmailVerified: true,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
  );
}

Widget _wrapWithProviderScope({
  required Widget child,
  required _MutableAuthController authController,
  required _ControlledFollowRepository repository,
  required String profileUserId,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      followRepositoryProvider.overrideWithValue(repository),
      contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
      ratingRepositoryProvider.overrideWithValue(_FakeRatingRepository()),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
      blockedUserIdsProvider.overrideWith((ref) => Stream.value(<String>{})),
      profileViewDataProvider(profileUserId).overrideWithValue(
        AsyncData<ProfileViewData?>(
          ProfileViewData(
            user: _authUser(id: profileUserId, username: 'target'),
            profile: _profileEntity(userId: profileUserId),
          ),
        ),
      ),
    ],
    child: child,
  );
}

Future<void> _prepareProfileTestViewport(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1080, 2400));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  test(
    'checkFollowStatus completes after consumer disposal without touching disposed ref',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);

      final subscription = container.listen<FollowStatusState>(
        followStatusProvider,
        (previous, next) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      final future = container
          .read(followStatusProvider.notifier)
          .checkFollowStatus(followerId: 'viewer-1', followingId: 'target-1');

      await Future<void>.delayed(Duration.zero);
      container.dispose();
      repository.completeNextStatus('target-1', Result.success(true));

      await future;
    },
  );

  testWidgets(
    'Profile to Chat navigation disposes in-flight follow lookup cleanly',
    (tester) async {
      await _prepareProfileTestViewport(tester);
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      repository.enqueueStatus('target-1');

      final profile = _wrapWithProviderScope(
        authController: authController,
        repository: repository,
        profileUserId: 'target-1',
        child: const MaterialApp(home: ProfileScreen(userId: 'target-1')),
      );

      await tester.pumpWidget(profile);
      await tester.pump();

      expect(repository.checkCalls, greaterThan(0));

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(body: Center(child: Text('Chat'))),
        ),
      );

      while (repository.pendingStatusCount('target-1') > 0) {
        repository.completeNextStatus('target-1', Result.success(false));
      }
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
    },
  );

  test(
    'target change A to B keeps the latest target state and ignores stale A completion',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final notifier = container.read(followStatusProvider.notifier);
      final first = notifier.checkFollowStatus(
        followerId: 'viewer-1',
        followingId: 'user-a',
      );
      final second = notifier.checkFollowStatus(
        followerId: 'viewer-1',
        followingId: 'user-b',
      );

      repository.completeNextStatus('user-b', Result.success(false));
      await second;
      expect(
        container.read(followStatusProvider).followStatusMap['user-b'],
        isFalse,
      );

      repository.completeNextStatus('user-a', Result.success(true));
      await first;
      final state = container.read(followStatusProvider);
      expect(state.followStatusMap['user-a'], isTrue);
      expect(state.followStatusMap['user-b'], isFalse);
    },
  );

  test(
    'latest overlapping response for the same target wins over an older response',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final notifier = container.read(followStatusProvider.notifier);
      final first = notifier.checkFollowStatus(
        followerId: 'viewer-1',
        followingId: 'target-1',
      );
      final second = notifier.checkFollowStatus(
        followerId: 'viewer-1',
        followingId: 'target-1',
      );

      repository.completeStatusAt('target-1', 1, Result.success(false));
      await second;
      repository.completeStatusAt('target-1', 0, Result.success(true));
      await first;

      final state = container.read(followStatusProvider);
      expect(state.followStatusMap['target-1'], isFalse);
    },
  );

  test(
    'logout while request is in flight clears follow cache and drops stale result',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final future = container
          .read(followStatusProvider.notifier)
          .checkFollowStatus(followerId: 'viewer-1', followingId: 'target-1');

      authController.setStateValue(const AuthState.unauthenticated());
      await Future<void>.delayed(Duration.zero);

      repository.completeNextStatus('target-1', Result.success(true));
      await future;

      final state = container.read(followStatusProvider);
      expect(state.followStatusMap, isEmpty);
      expect(state.error, isNull);
    },
  );

  test(
    'principal switch while request is in flight clears follow cache and drops stale result',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final future = container
          .read(followStatusProvider.notifier)
          .checkFollowStatus(followerId: 'viewer-1', followingId: 'target-1');

      authController.setStateValue(
        _authenticatedState(id: 'viewer-2', username: 'viewer2'),
      );
      await Future<void>.delayed(Duration.zero);

      repository.completeNextStatus('target-1', Result.success(true));
      await future;

      final state = container.read(followStatusProvider);
      expect(state.followStatusMap, isEmpty);
      expect(state.error, isNull);
    },
  );

  test(
    'repository error publishes state when provider stays mounted',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final future = container
          .read(followStatusProvider.notifier)
          .checkFollowStatus(followerId: 'viewer-1', followingId: 'target-1');
      repository.completeNextStatus('target-1', Result.error('network down'));

      await future;

      final state = container.read(followStatusProvider);
      expect(state.error, 'network down');
      expect(state.followStatusMap['target-1'], isNull);
    },
  );

  test(
    'follow and unfollow mutate the correct target and do not leak to another key',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final notifier = container.read(followStatusProvider.notifier);

      final followFuture = notifier.followUser(
        followerId: 'viewer-1',
        followingId: 'user-a',
      );
      repository.completeNextFollow('user-a', Result.success(true));
      await followFuture;

      final afterFollow = container.read(followStatusProvider);
      expect(afterFollow.followStatusMap['user-a'], isTrue);
      expect(afterFollow.followStatusMap['user-b'], isNull);

      final unfollowFuture = notifier.unfollowUser(
        followerId: 'viewer-1',
        followingId: 'user-a',
      );
      repository.completeNextUnfollow('user-a', Result.success(true));
      await unfollowFuture;

      final afterUnfollow = container.read(followStatusProvider);
      expect(afterUnfollow.followStatusMap['user-a'], isFalse);
      expect(afterUnfollow.followStatusMap['user-b'], isNull);
    },
  );

  test(
    'provider can be recreated after disposal and continue working normally',
    () async {
      Future<FollowStatusState> runOnce() async {
        final authController = _MutableAuthController(
          _authenticatedState(id: 'viewer-1', username: 'viewer'),
        );
        final repository = _ControlledFollowRepository();
        final container = ProviderContainer(
          overrides: [
            authControllerProvider.overrideWith(() => authController),
            followRepositoryProvider.overrideWithValue(repository),
          ],
        );
        addTearDown(container.dispose);
        addTearDown(
          container
              .listen<FollowStatusState>(
                followStatusProvider,
                (previous, next) {},
                fireImmediately: true,
              )
              .close,
        );

        final future = container
            .read(followStatusProvider.notifier)
            .checkFollowStatus(followerId: 'viewer-1', followingId: 'target-1');
        repository.completeNextStatus('target-1', Result.success(true));
        await future;
        return container.read(followStatusProvider);
      }

      final first = await runOnce();
      final second = await runOnce();

      expect(first.followStatusMap['target-1'], isTrue);
      expect(second.followStatusMap['target-1'], isTrue);
    },
  );

  test(
    'stale response cannot republish after a fresh request has already completed',
    () async {
      final authController = _MutableAuthController(
        _authenticatedState(id: 'viewer-1', username: 'viewer'),
      );
      final repository = _ControlledFollowRepository();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          followRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(
        container
            .listen<FollowStatusState>(
              followStatusProvider,
              (previous, next) {},
              fireImmediately: true,
            )
            .close,
      );

      final notifier = container.read(followStatusProvider.notifier);
      final stale = notifier.checkFollowStatus(
        followerId: 'viewer-1',
        followingId: 'user-a',
      );
      final fresh = notifier.checkFollowStatus(
        followerId: 'viewer-1',
        followingId: 'user-a',
      );

      repository.completeStatusAt('user-a', 1, Result.success(false));
      await fresh;
      repository.completeStatusAt('user-a', 0, Result.success(true));
      await stale;

      final state = container.read(followStatusProvider);
      expect(state.followStatusMap['user-a'], isFalse);
      expect(state.error, isNull);
    },
  );
}
