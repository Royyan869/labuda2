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

/// The three seller axes are passed independently on purpose: workspace
/// (profile), capability (market authority) and expiry (subscription status).
AuthUser _seller({
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  required String sellerSubscriptionStatus,
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
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
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
      expect(find.text('Buat ForSale'), findsNothing);
    });

    testWidgets('unauthenticated state shows the login gate', (tester) async {
      await tester.pumpWidget(_wrap(const AuthState.unauthenticated()));

      expect(find.text('Login Diperlukan'), findsOneWidget);
      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.text('Buat ForSale'), findsNothing);
    });

    testWidgets('restricted account follows the canonical restricted flow', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrap(
          AuthState.accountRestricted(
            _seller(
              hasSellerProfile: true,
              hasMarketAuthority: true,
              sellerSubscriptionStatus: 'active',
            ),
            restrictionType: AccountStatus.suspended,
          ),
        ),
      );

      expect(find.text('Akun Ditangguhkan'), findsOneWidget);
      expect(find.text('Buat ForSale'), findsNothing);
    });

    testWidgets('non-seller gets the seller registration gate', (tester) async {
      await tester.pumpWidget(
        _wrap(
          AuthState.authenticated(
            _seller(
              hasSellerProfile: false,
              hasMarketAuthority: false,
              sellerSubscriptionStatus: 'none',
            ),
            emailVerified: true,
          ),
        ),
      );

      expect(find.text('Jadi Seller Dulu'), findsOneWidget);
      // Discriminator for "the create form is NOT reachable" (the form's first
      // section title; the submit button is lazily built further down).
      expect(find.text('Informasi Dasar'), findsNothing);
      expect(find.byType(TextFormField), findsNothing);
    });

    // -----------------------------------------------------------------------
    // CREATE = PUBLISH. Completing the form publishes to the market, so the
    // screen gates ENTRY on market authority — same as the auction create
    // screen. An expired seller sees the renewal CTA, never the form.
    // -----------------------------------------------------------------------
    testWidgets(
      'seller with a profile but no market authority gets the subscription gate',
      (tester) async {
        await tester.pumpWidget(
          _wrap(
            AuthState.authenticated(
              _seller(
                hasSellerProfile: true,
                hasMarketAuthority: false,
                sellerSubscriptionStatus: 'none',
              ),
              emailVerified: true,
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Entry blocked: the form never renders.
        expect(find.text('Langganan Belum Aktif'), findsOneWidget);
        expect(find.text('Aktifkan Langganan'), findsOneWidget);
        expect(find.text('Informasi Dasar'), findsNothing);
      },
    );

    testWidgets(
      'expired-subscription seller gets the renewal gate',
      (tester) async {
        await tester.pumpWidget(
          _wrap(
            AuthState.authenticated(
              _seller(
                hasSellerProfile: true,
                hasMarketAuthority: false,
                sellerSubscriptionStatus: 'expired',
              ),
              emailVerified: true,
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Langganan Seller Habis'), findsOneWidget);
        expect(find.text('Perpanjang Langganan'), findsOneWidget);
        expect(find.text('Informasi Dasar'), findsNothing);
      },
    );

    testWidgets('active seller still reaches the create form', (tester) async {
      await tester.pumpWidget(
        _wrap(
          AuthState.authenticated(
            _seller(
              hasSellerProfile: true,
              hasMarketAuthority: true,
              sellerSubscriptionStatus: 'active',
            ),
            emailVerified: true,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Informasi Dasar'), findsOneWidget);
      expect(find.byType(TextFormField), findsWidgets);
      expect(find.text('Langganan Seller Habis'), findsNothing);
      expect(find.text('Langganan Belum Aktif'), findsNothing);
      expect(find.text('Perpanjang Langganan'), findsNothing);
    });
  });
}
