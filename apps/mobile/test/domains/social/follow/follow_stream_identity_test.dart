import 'dart:async';
import 'dart:collection';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/follow/data/datasources/follow_api_datasource.dart';
import 'package:labuda/domains/social/follow/data/dto/follow_api_models.dart';
import 'package:labuda/domains/social/follow/data/repositories/api/follow_repository_api.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _NoopApiClient extends ApiClient {
  _NoopApiClient() : super.testing(baseUrl: 'https://example.com');
}

class _ScriptedFollowDatasource extends FollowApiDatasource {
  _ScriptedFollowDatasource({
    required Queue<Future<Result<FollowListResponseDto>> Function()> followers,
    required Queue<Future<Result<FollowListResponseDto>> Function()> following,
  })  : _followers = followers,
        _following = following,
        super(_NoopApiClient());

  final Queue<Future<Result<FollowListResponseDto>> Function()> _followers;
  final Queue<Future<Result<FollowListResponseDto>> Function()> _following;

  @override
  Future<Result<FollowListResponseDto>> getFollowers(
    String userId, {
    int limit = 20,
    String? cursor,
  }) {
    return _followers.removeFirst()();
  }

  @override
  Future<Result<FollowListResponseDto>> getFollowing(
    String userId, {
    int limit = 20,
    String? cursor,
  }) {
    return _following.removeFirst()();
  }
}

FollowListResponseDto _listResponse(List<FollowableUser> users) {
  return FollowListResponseDto(
    items: users
        .map(
          (user) => FollowListUserCardDto(
            id: user.id,
            username: user.username,
            avatarUrl: user.avatar,
            followersCount: user.followersCount,
            followingCount: user.followingCount,
            lifecycle: switch (user.lifecycle) {
              ContentLifecycle.active => 'active',
              ContentLifecycle.unavailable => 'unavailable',
              ContentLifecycle.removed => 'removed',
            },
          ),
        )
        .toList(),
    limit: users.length,
  );
}

FollowableUser _activeUser(
  String id,
  String username, {
  String? avatar,
  int followersCount = 0,
  int followingCount = 0,
}) {
  return FollowableUser(
    id: id,
    username: username,
    avatar: avatar,
    userType: UserType.buyer,
    followersCount: followersCount,
    followingCount: followingCount,
    lifecycle: ContentLifecycle.active,
  );
}

FollowableUser _unavailableUser(String id) {
  return FollowableUser(
    id: id,
    username: '',
    avatar: null,
    userType: UserType.buyer,
    lifecycle: ContentLifecycle.unavailable,
  );
}

FollowableUser _removedUser(String id) {
  return FollowableUser(
    id: id,
    username: '',
    avatar: null,
    userType: UserType.buyer,
    lifecycle: ContentLifecycle.removed,
  );
}

void main() {
  test('active to unavailable transitions through watchFollowers', () {
    fakeAsync((async) {
      final datasource = _ScriptedFollowDatasource(
        followers: Queue.of([
          () => Future.value(
                Result.success(
                  _listResponse([
                    _activeUser('user-1', 'alice', avatar: 'https://cdn.example.com/a.jpg'),
                  ]),
                ),
              ),
          () => Future.value(
                Result.success(
                  _listResponse([
                    _unavailableUser('user-1'),
                  ]),
                ),
              ),
        ]),
        following: Queue.of([]),
      );
      final repo = FollowRepositoryApi(datasource);
      final events = <List<FollowableUser>>[];

      final sub = repo.watchFollowers('user-1').listen(events.add);
      async.flushMicrotasks();

      expect(events, hasLength(1));
      expect(events.single.single.username, 'alice');
      expect(events.single.single.avatar, 'https://cdn.example.com/a.jpg');

      async.elapse(const Duration(seconds: 30));
      async.flushMicrotasks();

      expect(events, hasLength(2));
      expect(events.last.single.username, '');
      expect(events.last.single.avatar, isNull);
      expect(events.last.single.lifecycle, ContentLifecycle.unavailable);

      sub.cancel();
      repo.dispose();
    });
  });

  test('active to removed transitions through watchFollowing', () {
    fakeAsync((async) {
      final datasource = _ScriptedFollowDatasource(
        followers: Queue.of([]),
        following: Queue.of([
          () => Future.value(
                Result.success(
                  _listResponse([
                    _activeUser('user-2', 'bob', avatar: 'https://cdn.example.com/b.jpg'),
                  ]),
                ),
              ),
          () => Future.value(
                Result.success(
                  _listResponse([
                    _removedUser('user-2'),
                  ]),
                ),
              ),
        ]),
      );
      final repo = FollowRepositoryApi(datasource);
      final events = <List<FollowableUser>>[];

      final sub = repo.watchFollowing('user-2').listen(events.add);
      async.flushMicrotasks();

      expect(events, hasLength(1));
      expect(events.single.single.username, 'bob');

      async.elapse(const Duration(seconds: 30));
      async.flushMicrotasks();

      expect(events, hasLength(2));
      expect(events.last.single.username, '');
      expect(events.last.single.avatar, isNull);
      expect(events.last.single.lifecycle, ContentLifecycle.removed);

      sub.cancel();
      repo.dispose();
    });
  });

  test('in-flight poll does not resurrect after disposal', () {
    fakeAsync((async) {
      final pending = Completer<Result<FollowListResponseDto>>();
      final datasource = _ScriptedFollowDatasource(
        followers: Queue.of([
          () => Future.value(
                Result.success(
                  _listResponse([
                    _activeUser('user-3', 'carol'),
                  ]),
                ),
              ),
          () => pending.future,
        ]),
        following: Queue.of([]),
      );
      final repo = FollowRepositoryApi(datasource);
      final events = <List<FollowableUser>>[];
      final errors = <Object>[];

      final sub = repo.watchFollowers('user-3').listen(
        events.add,
        onError: errors.add,
      );
      async.flushMicrotasks();
      expect(events, hasLength(1));

      async.elapse(const Duration(seconds: 30));
      async.flushMicrotasks();
      expect(events, hasLength(1));

      repo.dispose();
      pending.complete(
        Result.success(
          _listResponse([
            _removedUser('user-3'),
          ]),
        ),
      );
      async.flushMicrotasks();

      expect(events, hasLength(1));
      expect(errors, isEmpty);

      sub.cancel();
    });
  });
}
