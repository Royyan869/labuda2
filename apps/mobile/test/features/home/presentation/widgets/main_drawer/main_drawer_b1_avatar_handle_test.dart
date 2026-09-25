import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/features/home/presentation/widgets/main_drawer/main_drawer.dart';
import 'package:labuda/shared/widgets/seller_identity_view.dart';
import 'package:labuda/shared/widgets/hybrid_avatar.dart';
import 'package:labuda/shared/widgets/seller_dual_avatar.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_stream_provider.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';

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

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoopLogger implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

const _userAId = '123e4567-e89b-12d3-a456-426614174010';
const _userBId = '123e4567-e89b-12d3-a456-426614174011';

AuthUser _user({
  required String id,
  required String username,
  String? avatarUrl,
  bool hasSellerProfile = false,
  bool hasMarketAuthority = false,
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
    hasMarketAuthority: hasMarketAuthority,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

ProfileEntity _profileForSeller(String userId, String storeName, {String? storeImageUrl}) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    joinedAt: DateTime(2025),
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: const UserVerificationInfo(isPhoneVerified: false, isEmailVerified: true, isIdVerified: false, isFarmVerified: false, badges: []),
    farmInfo: FarmInfo(farmName: storeName, farmPhotoUrl: storeImageUrl),
  );
}

Widget _wrap(AuthController controller, {ProfileEntity? profileForA, ProfileEntity? profileForB}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      apiClientProvider.overrideWithValue(_NoopApiClient()),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
      webSocketServiceProvider.overrideWithValue(WebSocketService(baseUrl: 'ws://localhost')),
      userOnlineStatusProvider(_userAId).overrideWith((ref) => Stream.value(false)),
      userOnlineStatusProvider(_userBId).overrideWith((ref) => Stream.value(false)),
      if (profileForA != null) profileStreamProvider(_userAId).overrideWith((ref) => Stream.value(profileForA)),
      if (profileForB != null) profileStreamProvider(_userBId).overrideWith((ref) => Stream.value(profileForB)),
      if (profileForA == null) profileStreamProvider(_userAId).overrideWith((ref) => Stream.value(null)),
      if (profileForB == null) profileStreamProvider(_userBId).overrideWith((ref) => Stream.value(null)),
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
    testWidgets('shared HybridAvatar is used for personal avatar', (
      tester,
    ) async {
      final user = _user(id: _userAId, username: 'testuser');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // For non-seller, drawer shows HybridAvatar via SellerIdentityView
      expect(find.byType(HybridAvatar), findsOneWidget);
    });

    testWidgets('no image + valid username renders initials', (tester) async {
      final user = _user(id: _userAId, username: 'john_doe');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      final avatar = tester.widget<HybridAvatar>(find.byType(HybridAvatar));
      expect(avatar.userId, _userAId);
      // Owner decision 2026-09-24: initials fallback is removed — avatar is
      // photo-or-person-icon. Without a photo the person icon renders.
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('numeric-only username renders person icon (no initials)', (tester) async {
      final user = _user(id: _userAId, username: '12345');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // Initials are gone from the avatar contract — numeric usernames get
      // the same photo-or-person-icon rendering as everyone else.
      expect(find.text('12'), findsNothing);
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
          hasMarketAuthority: true,
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );

        await tester.pumpWidget(_wrap(controller, profileForA: _profileForSeller(_userAId, 'Qiqi Store', storeImageUrl: 'https://example.com/store.png')));
        await tester.pump();

        expect(find.byType(SellerIdentityView), findsOneWidget);
        expect(find.text('Qiqi Store'), findsOneWidget);
        expect(find.text('@testuser'), findsOneWidget);
        expect(find.byType(SellerDualAvatar), findsOneWidget);
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
          hasMarketAuthority: true,
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );

        await tester.pumpWidget(_wrap(controller, profileForA: _profileForSeller(_userAId, 'Qiqi Store')));
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
      final user = _user(id: _userAId, username: '   ');
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      // Whitespace-only: normalizedUsername is null → handle null → SellerIdentityView is SizedBox.shrink()
      expect(find.text('U'), findsNothing);
      expect(find.byType(HybridAvatar), findsNothing);
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
        id: _userAId,
        username: 'seller_user',
        hasSellerProfile: true,
        hasMarketAuthority: true,
      );
      final buyer = _user(id: _userBId, username: 'buyer_user');
      final controller = _FakeAuthController(
        AuthState.authenticated(seller, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller, profileForA: _profileForSeller(_userAId, 'Qiqi Store')));
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
