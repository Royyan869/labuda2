// SAFE-AREA-21 — SELLER RENEWAL SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the invariant on the `/seller/renewal` surface:
//   * the renewal CTA ("Bayar & …") is the LAST child of the body ListView
//     (after the design `SizedBox(height: 24)` separator), so the body-level
//     `SafeArea` is the ONE authority that consumes the live system bottom
//     inset — never a body-side second reservation, never a fixed clearance,
//     never manual inset arithmetic;
//   * the CTA consumes the REAL, LIVE inset: 0 → 24 → 34 → 48 moves it 1:1
//     and it never enters the system navigation region.
//
// REGIME NOTE (why the primary fixture is a SHORT window):
// The renewal content extent is ~465px. At the default 800×600 test view the
// list does NOT overflow (maxExtent = 0): the CTA is top-anchored and passes
// "no entry" only through accidental viewport slack — measured pre-fix as a
// collapsing margin (95px → 32px → 2px at phone width) that already BROKE at
// inset 48 on a 360×640 phone viewport (cta 604 > region start 592). The
// short-window fixture (800×500, e.g. landscape/split-screen) keeps the list
// overflowing at every inset, which is where the scroll-end contract is
// observable and where the missing authority provably fails. The default-view
// test below documents the non-overflow regime stays no-entry (it is a
// regression guard, not the ratchet workhorse).
//
// KEYBOARD: the screen has NO text input (no form fields), so no user-driven
// keyboard exists; the keyboard case below injects viewInsets to prove the
// Scaffold resize lift happens exactly once and the SafeArea yields (no
// duplicate lift) if a keyboard ever covers this surface.
//
// RENDER-BASED on purpose: every number comes from the actual rendered
// screen under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/config/seller_upgrade_config_entity.dart';
import 'package:hishumi/core/config/seller_upgrade_config_provider.dart'
    as upgrade_config;
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:hishumi/domains/user/preference/seller/data/dto/seller_dto.dart';
import 'package:hishumi/domains/user/preference/seller/data/remote/seller_remote_datasource.dart';
import 'package:hishumi/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:mockito/mockito.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;

  @override
  Future<void> forceRefreshAuthState() async {}
}

/// The signed-in, already-a-seller session this payment-only screen serves.
AuthUser _sellerUser() {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller-1@example.com',
    username: 'seller-1',
    bio: 'Bio',
    phoneNumber: '+628123456789',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'expired',
    hasMarketAuthority: false,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

class _FakeSellerRemoteDatasource extends Mock
    implements SellerRemoteDatasource {
  @override
  Future<SellerSubscriptionPaymentMethodsDto>
  getSubscriptionPaymentMethods() async {
    return const SellerSubscriptionPaymentMethodsDto(
      principalAmount: 250000,
      currency: 'IDR',
      methods: [
        SellerSubscriptionPaymentMethodDto(
          methodCode: 'bca_va',
          displayName: 'BCA Virtual Account',
          serviceFeeAmount: 6250,
          grossAmount: 256250,
        ),
      ],
    );
  }
}

Widget _app() => ProviderScope(
  overrides: [
    authControllerProvider.overrideWith(
      () => _FakeAuthController(
        AuthState.authenticated(_sellerUser(), emailVerified: true),
      ),
    ),
    sellerRemoteDatasourceProvider
        .overrideWithValue(_FakeSellerRemoteDatasource()),
    upgrade_config.sellerUpgradeConfigProvider.overrideWith(
      (ref) async => const SellerUpgradeConfigEntity(
        yearlyFee: 250000,
        durationDays: 365,
        isEnabled: true,
        renewalReminderDays: 30,
      ),
    ),
  ],
  child: const MaterialApp(home: SellerRenewalScreen()),
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

/// Short window (800×500 logical at dpr 1): the content extent (~465px)
/// exceeds the body viewport at EVERY inset, so the CTA rides the scroll end
/// and the scroll-end geometry contract is observable.
void _setShortWindow(WidgetTester tester) {
  tester.view.devicePixelRatio = 1.0;
  tester.view.physicalSize = const Size(800, 500);
}

/// Puts the list at its scroll END. Drags like a user (re-reading maxExtent
/// every step). Re-run after every metrics change: the max extent moves when
/// the body/SafeArea resize.
Future<void> _scrollToEnd(WidgetTester tester) async {
  final Finder scroll = find.byType(ListView);
  expect(scroll, findsOneWidget, reason: 'body ListView not found');
  final Finder scrollable = find.descendant(
    of: scroll,
    matching: find.byType(Scrollable),
  );
  for (int i = 0; i < 100; i++) {
    final ScrollPosition position = tester.state<ScrollableState>(
      scrollable.first,
    ).position;
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(scroll, const Offset(0, -800));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = tester.state<ScrollableState>(
    scrollable.first,
  ).position;
  fail(
    'the renewal list never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface.
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the renewal CTA ("Bayar & Aktifkan" / "Bayar & Perpanjang").
Finder _cta() => find.byWidgetPredicate(
  (w) =>
      w is ElevatedButton &&
      w.child is Text &&
      ((w.child! as Text).data?.startsWith('Bayar & ') ?? false),
);

double _ctaBottom(WidgetTester tester) => tester.getBottomRight(_cta()).dy;

/// maxScrollExtent of the body at the current metrics (premise check).
double _maxExtent(WidgetTester tester) {
  final Finder scroll = find.byType(ListView);
  return tester.state<ScrollableState>(
    find.descendant(of: scroll, matching: find.byType(Scrollable)).first,
  ).position.maxScrollExtent;
}

void main() {
  testWidgets(
    'short window: the renewal CTA rides the scroll end, follows the LIVE '
    'inset 1:1 and never enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setShortWindow(tester);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      expect(_cta(), findsOneWidget, reason: 'renewal CTA not rendered');
      await _scrollToEnd(tester);

      // Contract premise: the list overflows one viewport at inset 0 (the CTA
      // rides the scroll end). If this fails, the layout regime changed and
      // the scroll-end geometry contract needs a re-audit — not a silent pass.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the renewal content no longer overflows this short window — '
            're-audit the scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);

      // Design trailing spacing at scroll end (inset 0): the explicit
      // `AppMetrics.p16` ListView padding. Measured so design spacing is
      // never mistaken for inset authority and no fixed clearance hides here.
      final double design = surface - cta0;
      expect(
        design,
        closeTo(AppMetrics.p16, 0.01),
        reason:
            'at inset 0 the scroll-end gap below the renewal CTA must be the '
            'design spacing p16 (${AppMetrics.p16}px), got $design — larger '
            'would be a fixed bottom clearance, smaller would eat design '
            'spacing',
      );

      // Live insets: the CTA moves 1:1 with the system inset.
      final Map<double, double> bottoms = <double, double>{};
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);
        final double cta = _ctaBottom(tester);
        bottoms[inset] = cta;

        expect(
          cta,
          closeTo(cta0 - inset, 0.01),
          reason:
              'the renewal CTA did not follow the live system inset $inset: '
              'bottom $cta, expected ${cta0 - inset} (inset-0 design $cta0 '
              'minus $inset) — the body has no bottom-inset authority',
        );
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason: 'the renewal CTA entered the system navigation region at '
              'inset $inset (bottom $cta, region starts ${surface - inset})',
        );
      }

      // A changed inset moves the CTA 1:1 — no stale gap, no stand-in.
      expect(
        bottoms[24]! - bottoms[48]!,
        closeTo(24, 0.01),
        reason: 'did not track a changed system inset 1:1 '
            '(24→48 delta was ${bottoms[24]! - bottoms[48]!})',
      );
      expect(
        bottoms[34]! - bottoms[48]!,
        closeTo(14, 0.01),
        reason: 'did not track a changed system inset 1:1 '
            '(34→48 delta was ${bottoms[34]! - bottoms[48]!})',
      );
    },
  );

  testWidgets(
    'default view (non-overflow regime): the CTA never enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();
      expect(_cta(), findsOneWidget);

      final double surface = _surfaceBottom(tester);
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double cta = _ctaBottom(tester);
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason: 'the renewal CTA entered the system navigation region at '
              'inset $inset (bottom $cta, region starts ${surface - inset})',
        );
      }
    },
  );

  testWidgets(
    'exactly ONE SafeArea wraps the list body — no duplicate authority',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 34);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      // Supplementary structure check ONLY — the geometry tests above are
      // the proof. Exactly ONE SafeArea may wrap the body ListView (i.e. the
      // renewal CTA): a second/nested one would be a duplicate bottom-inset
      // authority. The AppBar's own framework `SafeArea(bottom: false)` wraps
      // only the app bar, never the list, so it is not counted here.
      expect(
        find.ancestor(
          of: find.byType(ListView),
          matching: find.byType(SafeArea),
        ),
        findsOneWidget,
        reason: 'exactly ONE SafeArea must wrap the body ListView — the '
            'renewal CTA must sit under a single bottom-inset authority',
      );
    },
  );

  testWidgets(
    'keyboard 300 (short window): Scaffold lifts the CTA with no duplicate '
    'clearance, closing restores the system-inset geometry',
    (tester) async {
      addTearDown(tester.view.reset);
      _setShortWindow(tester);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      await _scrollToEnd(tester);
      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);
      final double design = surface - cta0;

      // Keyboard opens on top of system inset 34. Platform semantics: the
      // keyboard consumes the bottom padding (SafeArea yields), and the
      // Scaffold body resize lifts the content — exactly ONCE.
      _setWindowInsets(tester, systemBottom: 34, keyboard: 300);
      await tester.pumpAndSettle();
      await _scrollToEnd(tester);

      final double kbCta = _ctaBottom(tester);
      expect(
        kbCta,
        closeTo(surface - 300 - design, 0.01),
        reason:
            'with the keyboard open the CTA must sit at body-bottom '
            '(surface − 300) minus the design spacing $design — got $kbCta, '
            'expected ${surface - 300 - design}. A duplicate keyboard '
            'reservation would push it higher; no lift would leave it under '
            'the keyboard.',
      );
      expect(
        kbCta,
        lessThanOrEqualTo(surface - 300 + 0.01),
        reason: 'the renewal CTA was pushed under the keyboard',
      );

      // Keyboard closes → back to the live system-inset geometry (inset 34);
      // a keyboard-sized gap must not survive.
      _setWindowInsets(tester, systemBottom: 34, keyboard: 0);
      await tester.pumpAndSettle();
      await _scrollToEnd(tester);
      expect(
        _ctaBottom(tester),
        closeTo(cta0 - 34, 0.01),
        reason:
            'after the keyboard closed the CTA must rest at the inset-0 '
            'scroll-end design position minus the live inset 34 '
            '(got ${_ctaBottom(tester)}, expected ${cta0 - 34}) — '
            'a stale gap or a missing inset authority survives here',
      );
    },
  );
}
