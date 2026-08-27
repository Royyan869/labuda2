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

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _FakePresenceRegistry extends PresenceSubscriptionRegistry {
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
    provider: ShonaAuthProvider.email,
  );
}

Widget _wrap({
  required Widget child,
  required String trackedUserId,
  required AuthState authState,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      avatarCacheServiceProvider.overrideWith((_) => _NoOpAvatarCacheService()),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _FakePresenceRegistry(),
      ),
      userOnlineStatusProvider(trackedUserId).overrideWithValue(false),
    ],
    child: MaterialApp(
      home: Scaffold(body: Center(child: child)),
    ),
  );
}

void main() {
  List<String> collectDetailTexts(WidgetTester tester) {
    return tester
        .widgetList<Text>(
          find.descendant(
            of: find.byType(SellerIdentityView),
            matching: find.byType(Text),
          ),
        )
        .map((text) => text.data)
        .whereType<String>()
        .toList();
  }

  testWidgets('SellerIdentityView.detail renders store name before handle', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        trackedUserId: '123e4567-e89b-12d3-a456-426614174001',
        authState: AuthState.authenticated(
          _user(
            id: '123e4567-e89b-12d3-a456-426614174099',
            username: 'viewer',
            avatarUrl: 'https://example.com/viewer.png',
          ),
          emailVerified: true,
        ),
        child: SellerIdentityView(
          identity: const SellerIdentityData(
            userId: '123e4567-e89b-12d3-a456-426614174001',
            username: '@@qiqijho',
            storeName: 'Qiqi Store',
            avatarUrl: 'https://example.com/avatar.jpg',
            storeImageUrl: 'https://example.com/store.jpg',
            publicOriginLine: 'Magelang, Jawa Tengah',
            isSeller: true,
          ),
          variant: SellerIdentityViewVariant.detail,
          size: 48,
        ),
      ),
    );

    final detailTexts = collectDetailTexts(tester);

    expect(detailTexts.take(3).toList(), [
      'Qiqi Store',
      '@qiqijho',
      'Magelang, Jawa Tengah',
    ]);
    expect(find.text('@Qiqi Store'), findsNothing);
    expect(find.byType(SellerDualAvatar), findsOneWidget);
  });

  testWidgets(
    'SellerIdentityView.detail keeps separate store and avatar fallbacks',
    (tester) async {
      await tester.pumpWidget(
        _wrap(
          trackedUserId: '123e4567-e89b-12d3-a456-426614174002',
          authState: AuthState.authenticated(
            _user(
              id: '123e4567-e89b-12d3-a456-426614174099',
              username: 'viewer',
              avatarUrl: 'https://example.com/viewer.png',
            ),
            emailVerified: true,
          ),
          child: SellerIdentityView(
            identity: const SellerIdentityData(
              userId: '123e4567-e89b-12d3-a456-426614174002',
              username: '@@qiqijho',
              storeName: 'Qiqi Store',
              avatarUrl: null,
              storeImageUrl: null,
              isSeller: true,
            ),
            variant: SellerIdentityViewVariant.detail,
            size: 48,
          ),
        ),
      );

      expect(find.byIcon(Icons.storefront), findsOneWidget);
      expect(find.byType(ProfileAvatar), findsOneWidget);
      expect(find.text('Magelang, Jawa Tengah'), findsNothing);

      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.imageUrl, isNull);
      expect(avatar.username, 'qiqijho');
      expect(find.text('@Qiqi Store'), findsNothing);
    },
  );
}
