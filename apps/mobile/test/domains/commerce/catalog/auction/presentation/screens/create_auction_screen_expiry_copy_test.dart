// RF-02 contract — CreateAuctionScreen gate COPY follows the canonical expiry
// axis (`sellerSubscriptionStatus`), never the capability axis.
//
// `hasMarketAuthority` stays the authority for "may sell right now": the form
// is still blocked for every capability-false seller. What changes is the
// wording: only an ENDED subscription period ('expired') may say "Langganan
// Seller Habis" / "Perpanjang Langganan". A seller whose subscription is not
// active yet ('none') is a not-yet-activated seller, not an expired one, and
// must be told to activate instead.
//
// Harness mirrors create_for_sale_screen_authority_test.dart: the screen's gate
// renders purely from `authControllerProvider` (and the canonical expiry
// provider derived from it), so a fake auth controller is enough — no network,
// no media picker, no auction notifier.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  final AuthState _state;

  _FakeAuthController(this._state);

  @override
  bool get shouldInitializeAuthListener => false;

  @override
  AuthState build() => _state;
}

/// The two seller axes are passed independently on purpose: capability gates
/// selling, `sellerSubscriptionStatus` is the only expiry authority (RF-02).
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
    child: const MaterialApp(home: CreateAuctionScreen()),
  );
}

void main() {
  group('CreateAuctionScreen expiry copy (RF-02)', () {
    testWidgets(
      'capability false + subscription none: activation copy, never expiry',
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

        // Capability gate still blocks the form...
        expect(find.text('Informasi Dasar'), findsNothing);
        // ...with activation wording, not an expiry claim.
        expect(find.text('Langganan Belum Aktif'), findsOneWidget);
        expect(find.text('Aktifkan Langganan'), findsOneWidget);
        expect(find.text('Langganan Seller Habis'), findsNothing);
        expect(find.text('Perpanjang Langganan'), findsNothing);
      },
    );

    testWidgets('capability false + subscription expired: expiry copy', (
      tester,
    ) async {
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

      expect(find.text('Informasi Dasar'), findsNothing);
      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(find.text('Perpanjang Langganan'), findsOneWidget);
      expect(find.text('Langganan Belum Aktif'), findsNothing);
      expect(find.text('Aktifkan Langganan'), findsNothing);
    });

    testWidgets('capability true + subscription active: no expiry copy', (
      tester,
    ) async {
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

      expect(find.text('Langganan Seller Habis'), findsNothing);
      expect(find.text('Perpanjang Langganan'), findsNothing);
      expect(find.text('Langganan Belum Aktif'), findsNothing);
      expect(find.text('Aktifkan Langganan'), findsNothing);
    });

    testWidgets('seller without a profile still gets the registration gate', (
      tester,
    ) async {
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
      await tester.pumpAndSettle();

      expect(find.text('Jadi Seller Dulu'), findsOneWidget);
      expect(find.text('Mulai Jualan'), findsOneWidget);
      expect(find.text('Langganan Seller Habis'), findsNothing);
      expect(find.text('Langganan Belum Aktif'), findsNothing);
    });
  });
}
