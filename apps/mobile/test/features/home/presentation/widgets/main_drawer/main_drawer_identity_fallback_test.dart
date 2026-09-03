import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
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

AuthUser _seller({
  required String id,
  required String username,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$username@test.com',
    username: username,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
  );
}

AuthUser _buyer({
  required String id,
  required String username,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$username@test.com',
    username: username,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: false,
    sellerSubscriptionStatus: null,
    hasMarketAuthority: false,
  );
}

Widget _wrap(AuthController controller) {
  return ProviderScope(
    overrides: [authControllerProvider.overrideWith(() => controller)],
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
  testWidgets(
    'main drawer drops stale identity immediately and keeps fallback neutral',
    (tester) async {
      final seller = _seller(id: 'seller-1', username: 'sellertest');
      final buyer = _buyer(id: 'buyer-1', username: 'buyertest');
      final controller = _FakeAuthController(
        AuthState.authenticated(seller, emailVerified: true),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pump();

      expect(find.text('@sellertest'), findsOneWidget);
      expect(find.text('Seller Dashboard'), findsOneWidget);

      controller.setAuthState(const AuthState.loading());
      await tester.pump();

      expect(find.text('@sellertest'), findsNothing);
      expect(find.text('Seller Dashboard'), findsNothing);
      expect(find.byIcon(Icons.person_outline), findsOneWidget);

      controller.setAuthState(
        const AuthState.backendUnavailable('Backend down'),
      );
      await tester.pump();

      expect(find.text('@sellertest'), findsNothing);
      expect(find.byIcon(Icons.person_outline), findsOneWidget);

      controller.setAuthState(const AuthState.unauthenticated());
      await tester.pump();

      expect(find.text('@sellertest'), findsNothing);
      expect(find.byIcon(Icons.person_outline), findsNothing);
      expect(find.text('Sign In'), findsOneWidget);

      controller.setAuthState(
        AuthState.authenticated(buyer, emailVerified: true),
      );
      await tester.pump();

      expect(find.text('@buyertest'), findsOneWidget);
      expect(find.text('@sellertest'), findsNothing);
      expect(find.text('Seller Dashboard'), findsNothing);
    },
  );
}
