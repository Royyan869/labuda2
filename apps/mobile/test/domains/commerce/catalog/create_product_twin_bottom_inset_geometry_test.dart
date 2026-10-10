// SAFE-AREA-16 — CREATE-PRODUCT TWIN BOTTOM-INSET CONTRACT.
//
// CreateForSaleScreen and CreateAuctionScreen are direct twins: same UI
// responsibility (bottom-of-form publish CTA inside a form ListView), same
// Scaffold defaults (resizeToAvoidBottomInset, no bottomNavigationBar). This
// suite measures BOTH actual screens under injected window metrics with the
// SAME assertions, so any authority drift between them shows up as geometry:
//
//   * inset 0      → the CTA rests at its DESIGN trailing spacing only
//                    (no phantom inset reservation, no fixed clearance).
//   * inset 24/34/48 → the CTA follows the LIVE system inset 1:1 from its
//                    inset-0 design position, and never sits inside the
//                    system navigation region.
//   * keyboard 300 → the Scaffold body resize lifts the CTA (single keyboard
//                    authority: no duplicate reservation stacked by a
//                    SafeArea), and closing it returns to the live
//                    system-inset geometry with no stale gap.
//
// Authority under test: the body-level `SafeArea` (the canonical twin
// mechanism — CreateAuctionScreen has it; CreateForSaleScreen must carry the
// same one). The ListView's EXPLICIT `EdgeInsets.all(16)` padding disables
// the framework's automatic padding consumption, so without that SafeArea
// nothing on the body consumes the system inset and the CTA position becomes
// invariant across insets.
//
// RENDER-BASED on purpose: every number comes from the rendered screen under
// injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';

/// Publish CTA labels (kept exact so the finder binds to the real buttons).
const String _forSaleCta = 'Publikasikan ForSale';
const String _auctionCta = 'Buat Lelang';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;

  @override
  Future<void> forceRefreshAuthState() async {}
}

class _FakeShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async =>
      Result.success(const <ShippingSetup>[]);

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async =>
      Result.success(const <ShippingSetup>[]);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAddressRepository implements IAddressRepository {
  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(
    String userId,
  ) async => Result.success(const <AddressEntity>[]);

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(String userId) async =>
      Result.success(null);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// The active-seller session both create screens require to reach the form
/// (hasSellerProfile + hasMarketAuthority + active subscription).
AuthUser _activeSeller() {
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
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
    sellerTier: SellerTier.sellerBasic,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

AuthState _activeAuthState() =>
    AuthState.authenticated(_activeSeller(), emailVerified: true);

/// Harness mirroring `create_for_sale_screen_authority_test.dart` (auth
/// override only — the proven minimal setup for the for-sale form).
Widget _forSaleApp() => ProviderScope(
  overrides: [
    authControllerProvider.overrideWith(
      () => _FakeAuthController(_activeAuthState()),
    ),
  ],
  child: const MaterialApp(home: CreateForSaleScreen()),
);

/// Harness mirroring `create_auction_screen_authority_test.dart` (its
/// container overrides minus the notifier — the notifier is only read on
/// submit, never during build).
Widget _auctionApp() => ProviderScope(
  overrides: [
    authControllerProvider.overrideWith(
      () => _FakeAuthController(_activeAuthState()),
    ),
    shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
    addressRepositoryProvider.overrideWithValue(_FakeAddressRepository()),
  ],
  child: const MaterialApp(home: CreateAuctionScreen()),
);

/// Window metrics for the TEST VIEW (physical pixels, like a device).
///
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding` — an open keyboard zeroes the bottom padding
/// exactly like Android does, so the SafeArea yields to it.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    bottom: math.max(0.0, systemBottom - keyboard) * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Finder _formList() => find.byWidgetPredicate(
  (w) => w is ListView && w.scrollDirection == Axis.vertical,
);

/// Puts the form ListView at its scroll END — the state where the CTA rests
/// lowest and therefore bounds "never enters the system region". Drags like a
/// user (re-reading maxExtent every step: a sliver's extent starts as a lazy
/// estimate and grows as children inflate). Re-run after every metrics
/// change: the max extent moves when the body/SafeArea resize.
Future<void> _scrollFormToEnd(WidgetTester tester) async {
  final Finder list = _formList();
  expect(list, findsOneWidget, reason: 'form ListView not found');
  final Finder scrollable = find.descendant(
    of: list,
    matching: find.byType(Scrollable),
  );
  for (int i = 0; i < 80; i++) {
    final ScrollPosition position = tester.state<ScrollableState>(
      scrollable.first,
    ).position;
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(list, const Offset(0, -600));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = tester.state<ScrollableState>(
    scrollable.first,
  ).position;
  fail(
    'the form never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the publish CTA.
double _ctaBottom(WidgetTester tester, String ctaLabel) =>
    tester.getBottomRight(find.widgetWithText(ElevatedButton, ctaLabel)).dy;

void _registerSuite({
  required String screenName,
  required Widget Function() app,
  required String ctaLabel,
}) {
  testWidgets(
    '$screenName: inset 0 → 24/34/48 — the CTA follows the LIVE inset 1:1 '
    'and never enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      await _scrollFormToEnd(tester);
      // The CTA is the LAST sliver child — it only inflates once scrolled to.
      expect(
        find.widgetWithText(ElevatedButton, ctaLabel),
        findsOneWidget,
        reason: '$screenName did not reach the form state',
      );
      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester, ctaLabel);

      // ---- inset 0: DESIGN spacing only. ----
      // Trailing space below the CTA at rest = the ListView bottom padding +
      // the screen's trailing design spacer. Design-scale, NOT a system-inset
      // stand-in (a fixed 80/96/100 clearance would blow this bound), and not
      // a phantom inset reservation (nothing may push the CTA up at inset 0).
      final double design = surface - cta0;
      expect(
        design,
        greaterThan(0),
        reason: '$screenName CTA sits at/below the surface edge at inset 0',
      );
      expect(
        design,
        lessThan(96),
        reason:
            '$screenName carried $design px below the CTA at inset 0 — larger '
            'than design spacing, looks like a fixed bottom clearance',
      );
      expect(
        cta0,
        lessThanOrEqualTo(surface + 0.01),
        reason: '$screenName CTA left the surface at inset 0',
      );

      // ---- live insets: the CTA moves 1:1 with the system inset. ----
      final Map<double, double> bottoms = <double, double>{};
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollFormToEnd(tester);

        final double cta = _ctaBottom(tester, ctaLabel);
        bottoms[inset] = cta;

        expect(
          cta,
          closeTo(cta0 - inset, 0.01),
          reason:
              '$screenName CTA did not follow the live system inset $inset: '
              'bottom $cta, expected ${cta0 - inset} (inset-0 design $cta0 '
              'minus $inset) — the body has no bottom-inset authority',
        );
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason: '$screenName CTA entered the system navigation region at '
              'inset $inset (bottom $cta, region starts ${surface - inset})',
        );
      }

      // A changed inset moves the CTA 1:1 — no stale gap, no stand-in.
      expect(
        bottoms[24]! - bottoms[48]!,
        closeTo(24, 0.01),
        reason: '$screenName did not track a changed system inset 1:1 '
            '(24→48 delta was ${bottoms[24]! - bottoms[48]!})',
      );
      expect(
        bottoms[34]! - bottoms[48]!,
        closeTo(14, 0.01),
        reason: '$screenName did not track a changed system inset 1:1 '
            '(34→48 delta was ${bottoms[34]! - bottoms[48]!})',
      );
    },
  );

  testWidgets(
    '$screenName: keyboard 300 — Scaffold resize lifts the CTA with no '
    'duplicate reservation, closing restores the inset geometry',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      await _scrollFormToEnd(tester);
      expect(
        find.widgetWithText(ElevatedButton, ctaLabel),
        findsOneWidget,
        reason: '$screenName did not reach the form state',
      );
      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester, ctaLabel);
      final double design = surface - cta0;

      // Keyboard opens on top of system inset 24. Platform semantics: the
      // keyboard consumes the bottom padding (SafeArea yields), and the
      // Scaffold body resize lifts the content — exactly ONCE.
      _setWindowInsets(tester, systemBottom: 24, keyboard: 300);
      await tester.pumpAndSettle();
      await _scrollFormToEnd(tester);

      final double kbCta = _ctaBottom(tester, ctaLabel);
      expect(
        kbCta,
        closeTo(surface - 300 - design, 0.01),
        reason:
            'with the keyboard open the CTA must sit at body-bottom '
            '(surface − 300) minus the design spacing $design — got $kbCta, '
            'expected ${surface - 300 - design}. A duplicate keyboard '
            'reservation would push it higher; no lift at all would leave it '
            'under the keyboard.',
      );
      expect(
        kbCta,
        lessThanOrEqualTo(surface - 300 + 0.01),
        reason: 'the CTA was pushed under the keyboard',
      );

      // Keyboard closes → back to the live system-inset geometry (inset 24);
      // a keyboard-sized gap must not survive.
      _setWindowInsets(tester, systemBottom: 24, keyboard: 0);
      await tester.pumpAndSettle();
      await _scrollFormToEnd(tester);
      expect(
        _ctaBottom(tester, ctaLabel),
        closeTo(cta0 - 24, 0.01),
        reason:
            'after the keyboard closed the CTA must rest at the inset-0 '
            'design position minus the live inset 24 '
            '(got ${_ctaBottom(tester, ctaLabel)}, expected ${cta0 - 24}) — '
            'a stale gap or a missing inset authority survives here',
      );
    },
  );
}

void main() {
  _registerSuite(
    screenName: 'CreateForSaleScreen',
    app: _forSaleApp,
    ctaLabel: _forSaleCta,
  );
  _registerSuite(
    screenName: 'CreateAuctionScreen',
    app: _auctionApp,
    ctaLabel: _auctionCta,
  );
}
