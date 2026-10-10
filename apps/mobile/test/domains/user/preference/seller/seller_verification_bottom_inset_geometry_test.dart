// SAFE-AREA-20 — SELLER VERIFICATION SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the invariant on the `/verification/seller` surface:
//   * the "Ajukan Verifikasi" CTA is the LAST child of the body
//     SingleChildScrollView (fixture premise: status notSubmitted renders the
//     form, documents empty so nothing follows the button), so the body-level
//     `SafeArea` is the ONE authority that consumes the live system bottom
//     inset — never a body-side second reservation, never a fixed clearance,
//     never manual inset arithmetic;
//   * the CTA consumes the REAL, LIVE inset: 0 → 24 → 34 → 48 moves it 1:1;
//   * at every non-zero inset the CTA stays OUTSIDE the system navigation
//     region (`cta bottom <= surface bottom - inset`);
//   * the trailing 16 px below the CTA at inset 0 is the DESIGN spacing (the
//     scroll view's explicit `AppMetrics.p16` padding) — measured separately
//     so design spacing is never mistaken for inset authority;
//   * the keyboard stays the Scaffold's business (`resizeToAvoidBottomInset`
//     body resize): open → body lifts exactly once (the SafeArea yields to
//     the keyboard through platform padding semantics), closed → the CTA
//     returns to the system-inset geometry with no stale gap.
//
// PREMISE: the form content exceeds one viewport (the CTA rides the scroll
// end). If the form ever shrinks below one screen, the scroll-end geometry
// contract must be re-audited — the precondition assertion below fails loud
// instead of passing silently on a different layout regime.
//
// RENDER-BASED on purpose: every number comes from the actual rendered
// screen under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:hishumi/domains/user/identity/verification/presentation/providers/seller_verification_v2_provider.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_verification_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';

/// Submit CTA label (kept exact so the finder binds to the real button).
const String _ctaLabel = 'Ajukan Verifikasi';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

/// The signed-in session this screen loads its data from.
AuthUser _authUser() {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'user-1',
    createdAt: now,
    updatedAt: now,
    email: 'user@example.com',
    username: 'user',
    bio: 'Bio',
    phoneNumber: null,
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
    sellerTier: SellerTier.sellerBasic,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

/// Keeps the verification state at its build default: status notSubmitted
/// (form + CTA rendered) and documents empty (CTA is the LAST scroll child —
/// the fixture premise above). No network call can move the state mid-test.
class _FakeVerificationNotifier extends SellerVerificationV2Notifier {
  @override
  SellerVerificationV2State build() => const SellerVerificationV2State();

  @override
  Future<void> loadStatus() async {}

  @override
  Future<void> loadDocuments() async {}
}

Widget _app() => ProviderScope(
  overrides: [
    authControllerProvider.overrideWith(
      () => _FakeAuthController(
        AuthState.authenticated(_authUser(), emailVerified: true),
      ),
    ),
    sellerVerificationV2NotifierProvider.overrideWith(
      () => _FakeVerificationNotifier(),
    ),
  ],
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: const SellerVerificationScreen(),
  ),
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

/// Puts the form at its scroll END — the state where the CTA rests lowest
/// and therefore bounds "never enters the system region". Drags like a user
/// (re-reading maxExtent every step: a sliver's extent starts as a lazy
/// estimate and grows as children inflate). Re-run after every metrics
/// change: the max extent moves when the body/SafeArea resize.
Future<void> _scrollToEnd(WidgetTester tester) async {
  final Finder scroll = find.byType(SingleChildScrollView);
  expect(scroll, findsOneWidget, reason: 'body scroll view not found');
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
    'the verification form never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the verification CTA.
double _ctaBottom(WidgetTester tester) =>
    tester.getBottomRight(find.widgetWithText(ElevatedButton, _ctaLabel)).dy;

/// maxScrollExtent of the body at the current metrics (premise check).
double _maxExtent(WidgetTester tester) {
  final Finder scroll = find.byType(SingleChildScrollView);
  return tester.state<ScrollableState>(
    find.descendant(of: scroll, matching: find.byType(Scrollable)).first,
  ).position.maxScrollExtent;
}

void main() {
  testWidgets(
    'system inset 0 → 24/34/48: the verification CTA follows the LIVE inset 1:1',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      await _scrollToEnd(tester);
      expect(
        find.widgetWithText(ElevatedButton, _ctaLabel),
        findsOneWidget,
        reason: 'the screen did not render its verification CTA '
            '(auth/status gate did not reach the form)',
      );

      // Contract premise: the form exceeds one viewport at inset 0 (the CTA
      // rides the scroll end). If this fails, the layout regime changed and
      // the scroll-end geometry contract needs a re-audit — not a silent pass.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the seller verification form no longer exceeds one viewport — '
            're-audit the scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);

      // Design trailing spacing at inset 0: the explicit `AppMetrics.p16`
      // scroll padding. Measured here so it is never mistaken for inset
      // authority, and so a fixed bottom clearance cannot hide behind it.
      final double design = surface - cta0;
      expect(
        design,
        closeTo(AppMetrics.p16, 0.01),
        reason:
            'at inset 0 the gap below the verification CTA must be the design '
            'spacing p16 (${AppMetrics.p16}px), got $design — larger would be '
            'a fixed bottom clearance, smaller would eat design spacing',
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
              'the verification CTA did not follow the live system inset '
              '$inset: bottom $cta, expected ${cta0 - inset} (inset-0 design '
              '$cta0 minus $inset) — the body has no bottom-inset authority',
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
    'system inset 24/34/48: the verification CTA never enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      final double surface = _surfaceBottom(tester);
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double cta = _ctaBottom(tester);
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason: 'the verification CTA entered the system navigation region '
              'at inset $inset (bottom $cta, region starts ${surface - inset})',
        );
      }
    },
  );

  testWidgets(
    'exactly ONE SafeArea wraps the body scroll view — no duplicate authority',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 34);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      // Supplementary structure check ONLY — the geometry tests above are
      // the proof. Exactly ONE SafeArea may wrap the body scroll view (i.e.
      // the verification CTA): a second/nested one would be a duplicate
      // bottom-inset authority. The AppBar's own framework
      // `SafeArea(bottom: false)` (app_bar.dart) wraps only the app bar,
      // never the scroll view, so it is not counted here and never touches
      // the bottom inset.
      expect(
        find.ancestor(
          of: find.byType(SingleChildScrollView),
          matching: find.byType(SafeArea),
        ),
        findsOneWidget,
        reason: 'exactly ONE SafeArea must wrap the body scroll view — the '
            'verification CTA must sit under a single bottom-inset authority',
      );
    },
  );

  testWidgets(
    'keyboard 300: Scaffold lifts the CTA with no duplicate clearance, '
    'closing restores the system-inset geometry',
    (tester) async {
      addTearDown(tester.view.reset);
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
        reason: 'the verification CTA was pushed under the keyboard',
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
            'design position minus the live inset 34 '
            '(got ${_ctaBottom(tester)}, expected ${cta0 - 34}) — '
            'a stale gap or a missing inset authority survives here',
      );
    },
  );
}
