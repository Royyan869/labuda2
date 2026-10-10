// SAFE-AREA-23 — SELLER EARNINGS SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the invariant on the `/seller/earnings` surface:
//   * the Withdraw CTA ("Tarik Dana") is the LAST child of the body
//     `SingleChildScrollView`, so the body-level `SafeArea` is the ONE
//     authority that consumes the live system bottom inset — never a
//     body-side second reservation, never a fixed clearance, never manual
//     inset arithmetic;
//   * the CTA consumes the REAL, LIVE inset: 0 → 24 → 34 → 48 moves it 1:1,
//     the body viewport shrinks by exactly the inset, and the CTA never
//     enters the system navigation region;
//   * the scroll CONTENT extent stays inset-invariant: this screen has NO
//     nested scrollable (the withdrawal history renders plain widgets inside
//     the card), so no mid-page widget can absorb the inset. Measured
//     pre-fix the inset was consumed by NOTHING (contentExtent 939 at every
//     inset) and the CTA rested on the 16 px design padding alone, which
//     provably put its bottom 8/18/32 px INSIDE the system region at
//     inset 24/34/48;
//   * the trailing 16 px below the CTA at inset 0 is the DESIGN spacing (the
//     scroll view's explicit `AppMetrics.p16` padding) — measured separately
//     so design spacing is never mistaken for inset authority;
//   * the loading/error branches carry no bottom content, so the authority
//     belongs to the CTA-bearing data branch only (same contract as the
//     SAFE-AREA-21 renewal twin).
//
// KEYBOARD: the screen has NO text input (no TextField / EditableText), so no
// user-driven keyboard exists. The keyboard case below injects viewInsets to
// prove the Scaffold resize lift happens exactly ONCE and the `SafeArea`
// yields to it — no fabricated keyboard requirement.
//
// RENDER-BASED on purpose: every number comes from the actual rendered screen
// under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_earnings.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/withdrawal.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/withdraw_notifier.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_earnings_screen.dart';
import 'package:hishumi/domains/user/preference/seller/seller_di.dart';

const String _sellerId = 'seller-1';

/// Withdraw CTA label (kept exact so the finder binds to the real button).
const String _ctaLabel = 'Tarik Dana';

class _StaticAuthController extends AuthController {
  _StaticAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

AuthUser _sellerUser() {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: _sellerId,
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
  );
}

/// Withdrawable balance above the Rp 10.000 minimum, so the CTA branch
/// (not the min-balance info card) is the bottom-most content.
SellerEarnings _earnings() => SellerEarnings(
  sellerId: _sellerId,
  totalRevenue: 1250000,
  pendingRevenue: 300000,
  totalPlatformFees: 0,
  availableBalance: 125000,
  withdrawalFeeAmount: 5000,
  totalWithdrawn: 420000,
  totalWithdrawals: 2,
  totalCompletedOrders: 0,
  calculatedAt: DateTime.utc(2026, 1, 1),
  grossPayable: 130000,
);

/// [earnings] may be a never-completing future (loading branch) or a failing
/// one (error branch); `retry` is disabled so a failed load never schedules
/// timers.
Widget _app({Future<SellerEarnings>? earnings}) => ProviderScope(
  retry: (retryCount, error) => null,
  overrides: [
    authControllerProvider.overrideWith(
      () => _StaticAuthController(
        AuthState.authenticated(_sellerUser(), emailVerified: true),
      ),
    ),
    sellerEarningsProvider(
      _sellerId,
    ).overrideWith((ref) async => earnings ?? _earnings()),
    withdrawalHistoryProvider.overrideWith((ref) async => const <Withdrawal>[]),
  ],
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    home: const SellerEarningsScreen(),
  ),
);

/// Window metrics for the TEST VIEW (physical pixels, like a device).
///
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding` — an open keyboard zeroes the bottom padding
/// exactly like Android does, so the `SafeArea` yields to it.
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

/// Short window (800×450 logical at dpr 1): a second layout regime where the
/// content still overflows, so the scroll-end contract is proven independent
/// of one viewport size.
void _setShortWindow(WidgetTester tester) {
  tester.view.devicePixelRatio = 1.0;
  tester.view.physicalSize = const Size(800, 450);
}

Finder _bodyScroll() => find.byType(SingleChildScrollView);

ScrollPosition _position(WidgetTester tester) => tester
    .state<ScrollableState>(
      find.descendant(of: _bodyScroll(), matching: find.byType(Scrollable)).first,
    )
    .position;

/// Puts the body at its scroll END — the state where the Withdraw CTA rests
/// lowest and therefore bounds "never enters the system region".
Future<void> _scrollToEnd(WidgetTester tester) async {
  expect(_bodyScroll(), findsOneWidget, reason: 'body scroll view not found');
  for (int i = 0; i < 100; i++) {
    final ScrollPosition position = _position(tester);
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(_bodyScroll(), const Offset(0, -600));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = _position(tester);
  fail(
    'the earnings body never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the Withdraw CTA.
double _ctaBottom(WidgetTester tester) =>
    tester.getBottomRight(find.widgetWithText(ElevatedButton, _ctaLabel)).dy;

/// Bottom of the body scroll viewport — the layer that must own the inset.
double _viewportBottom(WidgetTester tester) =>
    tester.getRect(_bodyScroll()).bottom;

/// maxScrollExtent of the body at the current metrics (premise check).
double _maxExtent(WidgetTester tester) => _position(tester).maxScrollExtent;

/// Total scrollable CONTENT height (extent + viewport). Must stay invariant to
/// the system inset: the inset belongs to the viewport, never to the content.
double _contentExtent(WidgetTester tester) =>
    _maxExtent(tester) + _position(tester).viewportDimension;

void main() {
  testWidgets(
    'default view: the Withdraw CTA follows the LIVE inset 1:1 and never '
    'enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      await _scrollToEnd(tester);
      expect(
        find.widgetWithText(ElevatedButton, _ctaLabel),
        findsOneWidget,
        reason: 'the screen did not render its Withdraw CTA',
      );

      // Contract premise: the content exceeds one viewport at inset 0 (the CTA
      // rides the scroll end). If this fails, the layout regime changed and the
      // scroll-end geometry contract needs a re-audit — not a silent pass.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the earnings content no longer exceeds one viewport — re-audit '
            'the scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);
      final double content0 = _contentExtent(tester);

      // inset 0 → design spacing only: the explicit `AppMetrics.p16` scroll
      // padding. No phantom inset reservation, no fixed clearance.
      expect(
        surface - cta0,
        closeTo(AppMetrics.p16, 0.01),
        reason:
            'at inset 0 the gap below the Withdraw CTA must be the design '
            'spacing p16 (${AppMetrics.p16}px), got ${surface - cta0} — larger '
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
              'the Withdraw CTA did not follow the live system inset $inset: '
              'bottom $cta, expected ${cta0 - inset} (inset-0 design $cta0 '
              'minus $inset) — the body has no bottom-inset authority',
        );
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the Withdraw CTA entered the system navigation region at '
              'inset $inset (bottom $cta, region starts ${surface - inset})',
        );
        expect(
          _viewportBottom(tester),
          closeTo(surface - inset, 0.01),
          reason:
              'the body viewport did not shrink by the live inset $inset — '
              'the inset is being consumed somewhere other than the body',
        );
        expect(
          _contentExtent(tester),
          closeTo(content0, 0.01),
          reason:
              'the live inset $inset leaked into the scroll CONTENT extent '
              '(${_contentExtent(tester)} vs $content0) — a mid-page widget is '
              'absorbing the bottom inset instead of the body',
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
    'short window (800×450): same contract — the CTA never enters the system '
    'region at any inset',
    (tester) async {
      addTearDown(tester.view.reset);
      _setShortWindow(tester);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();
      expect(find.widgetWithText(ElevatedButton, _ctaLabel), findsOneWidget);

      await _scrollToEnd(tester);
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the earnings content no longer overflows this short window — '
            're-audit the scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);
      for (final double inset in const <double>[24, 34, 48, 96]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double cta = _ctaBottom(tester);
        expect(
          cta,
          closeTo(cta0 - inset, 0.01),
          reason:
              'short window: the Withdraw CTA did not follow the live inset '
              '$inset (bottom $cta, expected ${cta0 - inset})',
        );
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'short window: the Withdraw CTA entered the system navigation '
              'region at inset $inset (bottom $cta, region starts '
              '${surface - inset})',
        );
      }
    },
  );

  testWidgets(
    'exactly ONE body SafeArea owns the inset — no duplicate authority',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 34);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      // Supplementary structure check ONLY — the geometry tests above are the
      // proof. Exactly ONE SafeArea may wrap the body scroll view (i.e. the
      // Withdraw CTA): a second/nested one would be a duplicate bottom-inset
      // authority. The AppBar's own framework `SafeArea(bottom: false)` wraps
      // only the app bar, never the scroll view, so it is not counted here.
      expect(
        find.ancestor(
          of: _bodyScroll(),
          matching: find.byType(SafeArea),
        ),
        findsOneWidget,
        reason: 'exactly ONE SafeArea must wrap the body scroll view — the '
            'Withdraw CTA must sit under a single bottom-inset authority',
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

      // No text input exists on this surface, so no user-driven keyboard is
      // fabricated — the injected inset only bounds the lift contract.
      expect(find.byType(EditableText), findsNothing);

      await _scrollToEnd(tester);
      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);
      final double design = surface - cta0;

      // Keyboard opens on top of system inset 34. Platform semantics: the
      // keyboard consumes the bottom padding (the SafeArea yields), and the
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
        reason: 'the Withdraw CTA was pushed under the keyboard',
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
            'after the keyboard closed the CTA must rest at the inset-0 design '
            'position minus the live inset 34 (got ${_ctaBottom(tester)}, '
            'expected ${cta0 - 34}) — a stale gap or a missing inset authority '
            'survives here',
      );
    },
  );

  testWidgets(
    'loading branch carries no bottom content — the inset authority is not '
    'needed there',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 48);
      const double regionStart = 600.0 - 48;

      // A pending load renders only the centered spinner (never settles:
      // `pump`, not `pumpAndSettle`, because the spinner animates forever).
      await tester.pumpWidget(
        _app(earnings: Completer<SellerEarnings>().future),
      );
      await tester.pump();

      final Rect spinner = tester.getRect(
        find.byType(CircularProgressIndicator),
      );
      expect(
        spinner.bottom,
        lessThanOrEqualTo(regionStart + 0.01),
        reason:
            'the loading branch reached the system navigation region '
            '(spinner bottom ${spinner.bottom}, region starts $regionStart)',
      );
    },
  );

  testWidgets(
    'error branch carries no bottom content — the inset authority is not '
    'needed there',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 48);
      const double regionStart = 600.0 - 48;

      final completer = Completer<SellerEarnings>();
      await tester.pumpWidget(_app(earnings: completer.future));
      await tester.pump();

      // Fail the load (the provider already listens, so no unhandled error).
      completer.completeError(Exception('boom'));
      await tester.pump();

      expect(
        find.widgetWithText(ElevatedButton, 'Retry'),
        findsOneWidget,
        reason: 'the error branch did not render',
      );
      final Rect retry = tester.getRect(
        find.widgetWithText(ElevatedButton, 'Retry'),
      );
      expect(
        retry.bottom,
        lessThanOrEqualTo(regionStart + 0.01),
        reason:
            'the error branch reached the system navigation region '
            '(Retry bottom ${retry.bottom}, region starts $regionStart)',
      );
    },
  );
}
