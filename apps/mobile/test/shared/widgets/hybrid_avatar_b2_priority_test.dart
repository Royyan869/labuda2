import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/shared.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);
  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _SpyAvatarCacheService extends AvatarCacheService {
  int getUserAvatarUrlCallCount = 0;
  List<String> getUserAvatarUrlCalls = [];

  _SpyAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async {
    getUserAvatarUrlCallCount++;
    getUserAvatarUrlCalls.add(userId);
    return 'https://cache.example/stale.png';
  }
}

class _SpyPresenceRegistry extends PresenceSubscriptionRegistry {
  int acquireCount = 0;

  @override
  PresenceSubscriptionHandle acquire(Set<String> userIds) {
    acquireCount++;
    return PresenceSubscriptionHandle(() async {});
  }

  @override
  Future<void> prepareForLogout() async {}

  @override
  PresenceState? lookup(String userId) => null;

  @override
  Map<String, PresenceState?> lookupMany(Iterable<String> userIds) => {};

  @override
  Future<void> publishSelfPresence({required bool isOnline}) async {}

  @override
  Future<void> setForeground(bool isForeground) async {}
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

AuthUser _user({
  required String id,
  required String username,
  String? avatarUrl,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$username@test.com',
    username: username,
    avatarUrl: avatarUrl,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

Widget _wrap({
  required _FakeAuthController authController,
  required _SpyAvatarCacheService cacheSpy,
  required _SpyPresenceRegistry presenceRegistry,
  required String trackedUserId,
  required Widget child,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      avatarCacheServiceProvider.overrideWith((_) => cacheSpy),
      presenceSubscriptionRegistryProvider.overrideWithValue(presenceRegistry),
      userOnlineStatusProvider(trackedUserId).overrideWithValue(false),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

void main() {
  group('HybridAvatar Tier 1 priority (current principal)', () {
    testWidgets('avatar URL sourced from auth state, cache never called', (
      tester,
    ) async {
      final user = _user(
        id: '123e4567-e89b-12d3-a456-426614174010',
        username: 'me',
        avatarUrl: 'https://auth.example/me.png',
      );
      final authController = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final cacheSpy = _SpyAvatarCacheService();
      final presenceRegistry = _SpyPresenceRegistry();

      await tester.pumpWidget(
        _wrap(
          authController: authController,
          cacheSpy: cacheSpy,
          presenceRegistry: presenceRegistry,
          trackedUserId: user.id,
          child: HybridAvatar(
            userId: user.id,
            savedAvatarUrl: 'https://stale.example/old.png',
            size: 40,
          ),
        ),
      );
      await tester.pump();

      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.imageUrl, 'https://auth.example/me.png');
      expect(cacheSpy.getUserAvatarUrlCallCount, 0);
      expect(presenceRegistry.acquireCount, 1);
    });

    testWidgets('current principal without saved URL still uses auth state', (
      tester,
    ) async {
      final user = _user(
        id: '123e4567-e89b-12d3-a456-426614174010',
        username: 'me',
        avatarUrl: 'https://auth.example/me.png',
      );
      final authController = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final cacheSpy = _SpyAvatarCacheService();
      final presenceRegistry = _SpyPresenceRegistry();

      await tester.pumpWidget(
        _wrap(
          authController: authController,
          cacheSpy: cacheSpy,
          presenceRegistry: presenceRegistry,
          trackedUserId: user.id,
          child: HybridAvatar(userId: user.id, size: 40),
        ),
      );
      await tester.pump();

      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.imageUrl, 'https://auth.example/me.png');
      expect(cacheSpy.getUserAvatarUrlCallCount, 0);
      expect(presenceRegistry.acquireCount, 1);
    });
  });

  group('HybridAvatar Tier 2+3 priority (non-current user)', () {
    testWidgets(
      'saved URL used initially; cache fetch succeeds and overrides',
      (tester) async {
        final currentUser = _user(
          id: '123e4567-e89b-12d3-a456-426614174010',
          username: 'me',
          avatarUrl: 'https://auth.example/me.png',
        );
        final authController = _FakeAuthController(
          AuthState.authenticated(currentUser, emailVerified: true),
        );
        final cacheSpy = _SpyAvatarCacheService();
        final presenceRegistry = _SpyPresenceRegistry();

        const otherUserId = '123e4567-e89b-12d3-a456-426614174011';
        await tester.pumpWidget(
          _wrap(
            authController: authController,
            cacheSpy: cacheSpy,
            presenceRegistry: presenceRegistry,
            trackedUserId: otherUserId,
            child: HybridAvatar(
              userId: otherUserId,
              savedAvatarUrl: 'https://saved.example/other.png',
              size: 40,
            ),
          ),
        );
        await tester.pump();

        final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
        expect(avatar.imageUrl, 'https://cache.example/stale.png');
        expect(cacheSpy.getUserAvatarUrlCallCount, 1);
        expect(cacheSpy.getUserAvatarUrlCalls, contains(otherUserId));
        expect(presenceRegistry.acquireCount, 1);
      },
    );

    testWidgets('no saved URL â€” falls through to cache fetch directly', (
      tester,
    ) async {
      final currentUser = _user(
        id: '123e4567-e89b-12d3-a456-426614174010',
        username: 'me',
        avatarUrl: 'https://auth.example/me.png',
      );
      final authController = _FakeAuthController(
        AuthState.authenticated(currentUser, emailVerified: true),
      );
      final cacheSpy = _SpyAvatarCacheService();
      final presenceRegistry = _SpyPresenceRegistry();

      const otherUserId = '123e4567-e89b-12d3-a456-426614174011';
      await tester.pumpWidget(
        _wrap(
          authController: authController,
          cacheSpy: cacheSpy,
          presenceRegistry: presenceRegistry,
          trackedUserId: otherUserId,
          child: HybridAvatar(
            userId: otherUserId,
            size: 40,
          ),
        ),
      );
      await tester.pump();

      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.imageUrl, 'https://cache.example/stale.png');
      expect(cacheSpy.getUserAvatarUrlCallCount, 1);
      expect(presenceRegistry.acquireCount, 1);
    });
  });
}
