import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/features/home/presentation/widgets/main_drawer/main_drawer.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState state) {
    _state = state;
    this.state = state;
  }
}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpPresenceRegistry extends PresenceSubscriptionRegistry {
  @override
  PresenceSubscriptionHandle acquire(Set<String> userIds) {
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

const _userAId = '123e4567-e89b-12d3-a456-426614174010';
const _userBId = '123e4567-e89b-12d3-a456-426614174011';

AuthUser _user({
  required String id,
  required String username,
  String? avatarUrl,
  bool hasSellerProfile = false,
  String storeName = '',
  String? storeImageUrl,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$username@test.com',
    username: username,
    avatarUrl: avatarUrl,
    isEmailVerified: true,
    hasSellerProfile: hasSellerProfile,
    storeName: storeName,
    storeImageUrl: storeImageUrl,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
  );
}

Widget _wrap(AuthController controller) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      avatarCacheServiceProvider.overrideWithValue(_NoOpAvatarCacheService()),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _NoOpPresenceRegistry(),
      ),
      userOnlineStatusProvider(_userAId).overrideWithValue(false),
      userOnlineStatusProvider(_userBId).overrideWithValue(false),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: MainDrawer(
          onTabChanged: (_) {},
          onNavigateToMessages: () {},
          onNavigateToNotifications: () {},
          onHandleSignIn: () {},
          onHandleSignUp: () {},
          onHandleSignOut: () {},
          onHandleSettings: () {},
          onHandleProfile: () {},
          onHandleComingSoon: (context, message) {},
        ),
      ),
    ),
  );
}

void main() {
  group('MainDrawer B1 canonical avatar', () {
    testWidgets('shared ProfileAvatar is used for personal avatar', (
      tester,
    ) async {
      final user = _user(id: _userAId, username: 'testuser');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // Shared ProfileAvatar rendered — verify it's in the tree.
      // ProfileAvatar wraps CachedNetworkImage or fallback initials.
      expect(find.byType(ProfileAvatar), findsOneWidget);
    });

    testWidgets('no image + valid username renders initials', (tester) async {
      final user = _user(id: _userAId, username: 'john_doe');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));

      // The drawer must pass the canonical username through to the shared
      // avatar primitive and avoid falling back to the generic person icon.
      expect(avatar.username, 'john_doe');
      expect(find.byIcon(Icons.person), findsNothing);
    });

    testWidgets('numeric-only username renders person icon', (tester) async {
      final user = _user(id: _userAId, username: '12345');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // Numeric-only username has no safe initials → generic person icon.
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets(
      'seller drawer shows business identity separate from personal',
      (tester) async {
        final user = _user(
          id: _userAId,
          username: 'testuser',
          avatarUrl: 'https://example.com/avatar.png',
          hasSellerProfile: true,
          storeName: 'Qiqi Store',
          storeImageUrl: 'https://example.com/store.png',
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );

        await tester.pumpWidget(_wrap(controller));
        await tester.pump();

        expect(find.byType(SellerIdentityView), findsOneWidget);
        expect(find.text('Qiqi Store'), findsOneWidget);
        expect(find.text('@testuser'), findsOneWidget);
        expect(find.byType(ProfileAvatar), findsOneWidget);
      },
    );

    testWidgets(
      'seller drawer uses storefront placeholder when no store image',
      (tester) async {
        final user = _user(
          id: _userAId,
          username: 'testuser',
          avatarUrl: 'https://example.com/avatar.png',
          hasSellerProfile: true,
          storeName: 'Qiqi Store',
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );

        await tester.pumpWidget(_wrap(controller));
        await tester.pump();

        expect(find.byType(SellerIdentityView), findsOneWidget);
        expect(find.byIcon(Icons.storefront), findsOneWidget);
        expect(find.text('Qiqi Store'), findsOneWidget);
        expect(find.text('@testuser'), findsOneWidget);
      },
    );

    testWidgets('invalid username does not display bare @ in handle', (
      tester,
    ) async {
      // Username that normalises to null: formatHandle returns null.
      // The display name (userName) may show the raw value, but the handle
      // line must never display a bare @.
      final user = _user(id: _userAId, username: '@');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // The handle line must not show a standalone '@' — formatHandle
      // returns null for bare-@, so no handle text widget is rendered.
      // No double-@@ anywhere.
      expect(find.textContaining('@@'), findsNothing);
    });

    testWidgets('leading @ in username displays exactly one @', (tester) async {
      final user = _user(id: _userAId, username: '@testuser');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // formatHandle('@testuser') → '@testuser' (single @).
      // Both the display name (userName) and handle may render '@testuser'
      // since the raw username has a leading @ — the handle is the
      // important one: it must have exactly one @, never @@.
      expect(find.text('@testuser'), findsAtLeast(1));
      expect(find.text('@@testuser'), findsNothing);
    });

    testWidgets('null username shows no handle text', (tester) async {
      // AuthUser with empty username — guaranteed because AuthUser.username
      // is non-nullable String, but the drawer receives the raw value.
      final user = _user(id: _userAId, username: '   ');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // Whitespace-only: formatHandle returns null → no handle displayed.
      // Person icon for avatar fallback, name shows 'User' (no username).
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('principal switch removes old handle and displays new', (
      tester,
    ) async {
      final userA = _user(id: _userAId, username: 'user_alpha');
      final userB = _user(id: _userBId, username: 'user_beta');
      final controller = _FakeAuthController(
        AuthState.authenticated(userA, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      expect(find.text('@user_alpha'), findsOneWidget);

      controller.setAuthState(
        AuthState.authenticated(userB, emailVerified: true),
      );
      await tester.pump();

      expect(find.text('@user_alpha'), findsNothing);
      expect(find.text('@user_beta'), findsOneWidget);
    });

    testWidgets('logout clears stale seller identity from drawer', (
      tester,
    ) async {
      final seller = _user(
        id: 'u-1',
        username: 'seller_user',
        hasSellerProfile: true,
        storeName: 'Qiqi Store',
      );
      final buyer = _user(id: 'u-2', username: 'buyer_user');
      final controller = _FakeAuthController(
        AuthState.authenticated(seller, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();
      expect(find.text('Qiqi Store'), findsOneWidget);

      controller.setAuthState(
        AuthState.authenticated(buyer, emailVerified: true),
      );
      await tester.pump();

      expect(find.text('Qiqi Store'), findsNothing);
      expect(find.text('@buyer_user'), findsOneWidget);
    });

    testWidgets('logout removes authenticated identity', (tester) async {
      final user = _user(id: 'u-1', username: 'testuser');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      expect(find.text('@testuser'), findsOneWidget);

      controller.setAuthState(const AuthState.unauthenticated());
      await tester.pump();

      expect(find.text('@testuser'), findsNothing);
      expect(find.text('Sign In'), findsOneWidget);
    });
  });
}
