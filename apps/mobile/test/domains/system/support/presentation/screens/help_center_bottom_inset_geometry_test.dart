// SAFE-AREA-22 — HELP CENTER SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the invariant on the `/help` surface (the "Hubungi Support" CTA):
//   * the contact CTA rides the body scroll end, so the body-level `SafeArea`
//     is the ONE authority that consumes the live system bottom inset — never
//     a body-side second reservation, never a fixed clearance, never manual
//     inset arithmetic;
//   * the CTA consumes the REAL, LIVE inset: 0 → 24 → 34 → 48 moves it 1:1;
//   * the BODY owns the inset, not a mid-page widget: the scroll viewport
//     shrinks by exactly the inset (`viewport bottom == surface bottom −
//     inset`) while the scroll CONTENT extent stays inset-invariant. Before
//     this authority existed, the category `GridView` (whose `padding` is
//     `null`) silently absorbed the inset as mid-page dead space right after
//     the category grid, while the scroll end reserved nothing at all;
//   * at every non-zero inset the CTA stays OUTSIDE the system navigation
//     region. The 73 px below the CTA at inset 0 is DESIGN spacing (the body
//     scroll view's own p16 + the "Still Need Help" section's trailing p32 +
//     the container's p24 padding and its 1 px border) — measured separately
//     so design spacing is never mistaken for inset authority. Because that
//     design slack is 73 px, the small inset regimes alone do NOT
//     discriminate authority: the LARGE inset regime (96/120 px > 73 px) is
//     where the missing authority genuinely pushed the CTA into the system
//     region before the fix — so it is asserted here as the real overlap
//     proof, not only the 0/24/34/48 ladder;
//   * the search field is keyboard-capable, and the keyboard stays the
//     Scaffold's business (`resizeToAvoidBottomInset` body resize): open →
//     the body lifts exactly once (the `SafeArea` yields to the keyboard
//     through platform padding semantics), closed → the CTA returns to the
//     system-inset geometry with no stale gap.
//
// PREMISE: the content exceeds one viewport (the CTA rides the scroll end).
// If it ever shrinks below one screen, the scroll-end geometry contract must
// be re-audited — the precondition assertion below fails loud instead of
// passing silently on a different layout regime.
//
// RENDER-BASED on purpose: every number comes from the actual rendered screen
// under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Contact CTA label in the `id` locale (kept exact so the finder binds to the
/// real button — it is the only `ElevatedButton` on the `/help` surface).
const String _ctaLabel = 'Hubungi Support';

/// DESIGN trailing space below the CTA at inset 0 — real design tokens only,
/// never an inset stand-in:
///   * `AppMetrics.p16` — the body scroll view's own bottom padding;
///   * `AppMetrics.p32` — the trailing `SizedBox(height: 32)` after the
///     "Still Need Help" section (spelled as a raw literal in the screen);
///   * `AppMetrics.p24` — the section container's own padding;
///   * `+1` — the container's 1 px border (a border is painted inside the
///     container box, so it adds to its padding).
const double _designBelowCta =
    AppMetrics.p16 + AppMetrics.p32 + AppMetrics.p24 + 1;

Widget _app() => MaterialApp(
  theme: AppTheme.lightTheme,
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  locale: const Locale('id'),
  home: const HelpCenterScreen(),
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

Finder _bodyScroll() => find.byType(SingleChildScrollView);

ScrollPosition _position(WidgetTester tester) => tester
    .state<ScrollableState>(
      find.descendant(of: _bodyScroll(), matching: find.byType(Scrollable)).first,
    )
    .position;

/// Puts the body at its scroll END — the state where the contact CTA rests
/// lowest and therefore bounds "never enters the system region". Drags like a
/// user (re-reading maxExtent every step). Re-run after every metrics change:
/// the max extent moves when the body/`SafeArea` resize.
Future<void> _scrollToEnd(WidgetTester tester) async {
  expect(_bodyScroll(), findsOneWidget, reason: 'body scroll view not found');
  for (int i = 0; i < 80; i++) {
    final ScrollPosition position = _position(tester);
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(_bodyScroll(), const Offset(0, -800));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = _position(tester);
  fail(
    'the help center never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the "Hubungi Support" CTA.
double _ctaBottom(WidgetTester tester) => tester
    .getBottomRight(find.widgetWithText(ElevatedButton, _ctaLabel))
    .dy;

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
    'system inset 0 → 24/34/48: the contact CTA follows the LIVE inset 1:1',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      await _scrollToEnd(tester);
      expect(
        find.widgetWithText(ElevatedButton, _ctaLabel),
        findsOneWidget,
        reason: 'the screen did not render its contact CTA',
      );

      // Contract premise: the content exceeds one viewport at inset 0 (the CTA
      // rides the scroll end). If this fails, the layout regime changed and the
      // scroll-end geometry contract needs a re-audit — not a silent pass.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the Help Center content no longer exceeds one viewport — '
            're-audit the scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double cta0 = _ctaBottom(tester);

      // inset 0 → design spacing only: no phantom inset reservation, no fixed
      // clearance standing in for the inset.
      expect(
        surface - cta0,
        closeTo(_designBelowCta, 0.01),
        reason:
            'at inset 0 the gap below the contact CTA must be exactly the '
            'design spacing $_designBelowCta px, got ${surface - cta0} — '
            'larger would be a fixed bottom clearance, smaller would eat '
            'design spacing',
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
              'the contact CTA did not follow the live system inset $inset: '
              'bottom $cta, expected ${cta0 - inset} (inset-0 design $cta0 '
              'minus $inset) — the body has no bottom-inset authority',
        );
        expect(
          _viewportBottom(tester),
          closeTo(surface - inset, 0.01),
          reason:
              'the body viewport did not shrink by the live inset $inset — '
              'the inset is being consumed somewhere other than the body',
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
    'system inset 24/34/48 → 120: the CTA never enters the system region, '
    'and the inset never leaks into the content extent',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();
      await _scrollToEnd(tester);

      final double surface = _surfaceBottom(tester);
      final double content0 = _contentExtent(tester);

      // Insets above the 73 px design slack are the DISCRIMINATING regime: with
      // the authority neutralized the CTA provably sits inside the system
      // region there, which is the real defect (at ≤ 73 px the design slack
      // masks it — see the header note).
      for (final double inset in const <double>[24, 34, 48, 96, 120]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double cta = _ctaBottom(tester);
        expect(
          cta,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the contact CTA entered the system navigation region at '
              'inset $inset (bottom $cta, region starts ${surface - inset})',
        );
        expect(
          _contentExtent(tester),
          closeTo(content0, 0.01),
          reason:
              'the live inset $inset leaked into the scroll CONTENT extent '
              '(${_contentExtent(tester)} vs $content0) — a mid-page widget '
              'is absorbing the bottom inset instead of the body',
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
      // contact CTA): a second/nested one would be a duplicate bottom-inset
      // authority. The AppBar's own framework `SafeArea(bottom: false)`
      // wraps only the app bar, never the scroll view, so it is not counted
      // here and never touches the bottom inset.
      expect(
        find.ancestor(
          of: _bodyScroll(),
          matching: find.byType(SafeArea),
        ),
        findsOneWidget,
        reason: 'exactly ONE SafeArea must wrap the body scroll view — the '
            'contact CTA must sit under a single bottom-inset authority',
      );
    },
  );

  testWidgets(
    'keyboard 300 (search field): Scaffold lifts the CTA with no duplicate '
    'clearance, closing restores the system-inset geometry',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      // The surface has a keyboard-capable control (the search field), so the
      // keyboard regime is real here — not a fabricated requirement.
      expect(find.byType(TextField), findsOneWidget);

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
        reason: 'the contact CTA was pushed under the keyboard',
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
            '(got ${_ctaBottom(tester)}, expected ${cta0 - 34}) — a stale gap '
            'or a missing inset authority survives here',
      );
    },
  );
}
