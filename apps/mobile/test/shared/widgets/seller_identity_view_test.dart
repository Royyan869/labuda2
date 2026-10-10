import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:hishumi/shared/models/seller_identity_data.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/widgets/seller_identity_view.dart';

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
  required Widget child,
  required String trackedUserId,
  required AuthState authState,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      avatarCacheServiceProvider.overrideWith((_) => _NoOpAvatarCacheService()),
      userOnlineStatusProvider(
        trackedUserId,
      ).overrideWith((ref) => Stream.value(false)),
    ],
    child: MaterialApp(home: Scaffold(body: Center(child: child))),
  );
}

void main() {
  const trackedUserId = '123e4567-e89b-12d3-a456-426614174001';

  List<Text> identityTexts(WidgetTester tester) {
    return tester
        .widgetList<Text>(
          find.descendant(
            of: find.byType(SellerIdentityView),
            matching: find.byType(Text),
          ),
        )
        .toList();
  }

  Widget wrapIdentity(SellerIdentityData identity) {
    return _wrap(
      trackedUserId: trackedUserId,
      authState: AuthState.authenticated(
        _user(id: 'viewer-1', username: 'viewer'),
        emailVerified: true,
      ),
      child: SellerIdentityView(identity: identity, size: 48),
    );
  }

  testWidgets('store name is the primary line and the handle the secondary', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrapIdentity(
        const SellerIdentityData(
          userId: trackedUserId,
          username: '@@qiqijho',
          storeName: 'Qiqi Store',
          avatarUrl: 'https://example.com/avatar.jpg',
          storeImageUrl: 'https://example.com/store.jpg',
          isSeller: true,
        ),
      ),
    );

    final texts = identityTexts(tester);
    expect(texts.map((t) => t.data).toList(), ['Qiqi Store', '@qiqijho']);

    // OWNER TRUTH: the store name sits above and renders larger than the
    // handle.
    expect(
      tester.getTopLeft(find.text('Qiqi Store')).dy,
      lessThan(tester.getTopLeft(find.text('@qiqijho')).dy),
    );
    expect(
      texts.first.style!.fontSize!,
      greaterThan(texts.last.style!.fontSize!),
    );
  });

  testWidgets('non-seller renders the handle as the primary line only', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrapIdentity(
        const SellerIdentityData(
          userId: trackedUserId,
          username: 'yayan',
          isSeller: false,
        ),
      ),
    );

    final texts = identityTexts(tester);
    expect(texts.map((t) => t.data).toList(), ['@yayan']);
  });

  testWidgets('no identity label renders nothing', (tester) async {
    await tester.pumpWidget(
      wrapIdentity(
        const SellerIdentityData(userId: trackedUserId, username: '   '),
      ),
    );

    expect(find.byType(SellerIdentityView), findsOneWidget);
    expect(identityTexts(tester), isEmpty);
  });
}
