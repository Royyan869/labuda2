import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  final AuthState _state;

  _FakeAuthController(this._state);

  @override
  AuthState build() => _state;
}

AuthUser _seller({
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

Widget _wrap(AuthState state) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(state)),
    ],
    child: const MaterialApp(home: CreateForSaleScreen()),
  );
}

void main() {
  group('CreateForSaleScreen authority gate', () {
    testWidgets('loading state fails closed while auth hydrates', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(const AuthState.loading()));

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('Buat Listing'), findsNothing);
    });

    testWidgets('unauthenticated state shows the login gate', (tester) async {
      await tester.pumpWidget(_wrap(const AuthState.unauthenticated()));

      expect(find.text('Login Diperlukan'), findsOneWidget);
      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.text('Buat Listing'), findsNothing);
    });

    testWidgets('restricted account follows the canonical restricted flow', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrap(
          AuthState.accountRestricted(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            restrictionType: AccountStatus.suspended,
          ),
        ),
      );

      expect(find.text('Akun Ditangguhkan'), findsOneWidget);
      expect(find.text('Buat Listing'), findsNothing);
    });

    testWidgets('non-seller gets the seller registration gate', (tester) async {
      await tester.pumpWidget(
        _wrap(
          AuthState.authenticated(
            _seller(hasSellerProfile: false, hasMarketAuthority: false),
            emailVerified: true,
          ),
        ),
      );

      expect(find.text('Jadi Seller Dulu'), findsOneWidget);
      expect(find.text('Buat Listing'), findsNothing);
    });

    testWidgets('expired seller gets the renewal gate', (tester) async {
      await tester.pumpWidget(
        _wrap(
          AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: false),
            emailVerified: true,
          ),
        ),
      );

      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(find.text('Buat Listing'), findsNothing);
    });
  });
}
