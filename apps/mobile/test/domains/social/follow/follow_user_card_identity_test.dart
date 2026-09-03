import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/follow/presentation/widgets/user_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/follow_button.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this.stateValue);

  final AuthState stateValue;

  @override
  AuthState build() => stateValue;
}

class _FakeFollowRepository implements IFollowRepository {
  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async =>
      Result.success(true);

  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async =>
      Result.success(false);

  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  }) async =>
      Result.success(true);

  @override
  Future<Result<List<FollowableUser>>> getFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async =>
      Result.success(const []);

  @override
  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  }) async =>
      Result.success(
        FollowStats(
          userId: userId,
          lastUpdated: DateTime.now(),
          followersCount: 0,
          followingCount: 0,
        ),
      );

  @override
  Future<Result<List<FollowableUser>>> getFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async =>
      Result.success(const []);

  @override
  Future<Result<bool>> muteUser({
    required String userId,
    required String targetUserId,
  }) async =>
      Result.success(true);

  @override
  Future<Result<List<FollowableUser>>> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  }) async =>
      Result.success(const []);

  @override
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async =>
      Result.success(true);

  @override
  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
  }) async =>
      Result.success(true);

  @override
  Future<Result<bool>> unmuteUser({
    required String userId,
    required String targetUserId,
  }) async =>
      Result.success(true);

  @override
  Stream<List<FollowActivity>> watchFollowActivities(String userId) =>
      Stream.value(const []);

  @override
  Stream<FollowStats> watchFollowStats(String userId) => Stream.value(
        FollowStats(
          userId: userId,
          lastUpdated: DateTime.now(),
          followersCount: 0,
          followingCount: 0,
        ),
      );

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) =>
      Stream.value(const []);

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) =>
      Stream.value(const []);
}

AuthState _authenticatedState({
  required String id,
  required String username,
}) {
  final now = DateTime.utc(2026, 7, 23);
  final user = AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: username,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
  return AuthState.authenticated(user, emailVerified: true);
}

Widget _wrap(Widget child, {AuthState? authState}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          authState ?? _authenticatedState(id: 'viewer-1', username: 'viewer'),
        ),
      ),
      followRepositoryProvider.overrideWithValue(_FakeFollowRepository()),
    ],
    child: MaterialApp(
      home: Material(child: Center(child: child)),
    ),
  );
}

void main() {
  testWidgets('active user card uses ProfileAvatar and canonical handle', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        UserCard(
          showFollowButton: true,
          user: FollowableUser(
            id: 'user-1',
            username: 'alice',
            avatar: 'https://cdn.example.com/a.jpg',
            userType: UserType.buyer,
            lifecycle: ContentLifecycle.active,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(ProfileAvatar), findsOneWidget);
    expect(find.text('@alice'), findsOneWidget);
    expect(find.byType(FollowButton), findsOneWidget);
  });

  testWidgets('active user card with empty username shows generic label', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        UserCard(
          showFollowButton: false,
          user: FollowableUser(
            id: 'user-2',
            username: '',
            avatar: null,
            userType: UserType.buyer,
            lifecycle: ContentLifecycle.active,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(ProfileAvatar), findsOneWidget);
    expect(find.text('User'), findsOneWidget);
    expect(find.byType(FollowButton), findsNothing);
  });

  testWidgets('degraded user card disables tap and follow action', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        UserCard(
          showFollowButton: true,
          user: FollowableUser(
            id: 'user-3',
            username: 'ghost',
            avatar: 'https://cdn.example.com/g.jpg',
            userType: UserType.buyer,
            lifecycle: ContentLifecycle.removed,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(ProfileAvatar), findsOneWidget);
    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.byType(FollowButton), findsNothing);

    final inkWell = tester.widget<InkWell>(find.byType(InkWell));
    expect(inkWell.onTap, isNull);
  });
}
