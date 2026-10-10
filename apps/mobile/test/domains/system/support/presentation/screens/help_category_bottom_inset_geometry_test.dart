// SAFE-AREA-24 — HELP CATEGORY SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the invariant on the `/help/category/:category` surface:
//   * the last article row is the bottom-most content of the body
//     `ListView.separated`, so the body-level `SafeArea` is the ONE authority
//     that consumes the live system bottom inset — never a body-side second
//     reservation, never a fixed clearance, never manual inset arithmetic;
//   * the last row consumes the REAL, LIVE inset: 0 → 24 → 34 → 48 moves it
//     1:1, the body viewport shrinks by exactly the inset, and the row never
//     enters the system navigation region;
//   * the scroll CONTENT extent stays inset-invariant: this screen has NO
//     nested scrollable (each row is a plain `InkWell` + `Container`), so no
//     mid-page widget can absorb the inset the way SAFE-AREA-22 proved for
//     the Help Center's category grid. Measured pre-fix the inset was
//     consumed by NOTHING (content extent 540 at every inset);
//   * the trailing 16 px below the last row at inset 0 (overflow regime) is
//     the DESIGN spacing (the list's explicit `AppMetrics.p16` padding) —
//     measured separately so design spacing is never mistaken for inset
//     authority.
//
// REGIME NOTE (why the short window is the primary ratchet):
// The `order` category holds six rows = 540 px of content, which FITS inside
// the default 800×600 test view (viewport 544 px → `maxExtent == 0`). In that
// non-overflow regime the list cannot scroll, so whichever part of the last
// row sits under the system bar is unreachable — measured pre-fix as the row
// bottom at 580 px entering the region at every inset > 20 (gap −4 / −14 /
// −28 at 24/34/48). The short-window fixture (800×360, e.g. landscape or
// split screen) keeps the list overflowing at every inset, where the clean
// 1:1 tracking and the 16 px design gap are observable. Both regimes assert
// the contract; the default-view test additionally documents the
// non-overflow regime (it is a regression guard, and after the fix the row
// clears the region AND regains scroll room).
//
// KEYBOARD: the screen has NO text input (no `TextField` / `EditableText`),
// so no user-driven keyboard exists — no keyboard regime is fabricated here.
//
// RENDER-BASED on purpose: every number comes from the actual rendered screen
// under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/generated/app_localizations_id.dart';

Widget _app() => MaterialApp(
  theme: AppTheme.lightTheme,
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  locale: const Locale('id'),
  // The longest category (six rows) — the only one that can overflow a short
  // window, so the scroll-end contract is observable.
  home: const HelpCategoryScreen(category: HelpCategory.order),
);

/// Window metrics for the TEST VIEW (physical pixels, like a device).
///
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding`, so an Android-style bottom padding is what
/// the body `SafeArea` must consume.
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

/// Short window (800×360 logical at dpr 1): the landscape / split-screen
/// regime where the list overflows at every inset.
void _setShortWindow(WidgetTester tester) {
  tester.view.devicePixelRatio = 1.0;
  tester.view.physicalSize = const Size(800, 360);
}

Finder _bodyList() => find.byType(ListView);

/// The article rows only — the AppBar leading button is an `InkWell` too, so
/// the rows are scoped to the body list.
Finder _rows() => find.descendant(
  of: _bodyList(),
  matching: find.byType(InkWell),
);

/// The LAST article row of the `order` category, identified by its own title
/// (`articleItemNotReceived`) rather than by position: `ListView.separated`
/// builds lazily, so only a window of rows exists at any scroll offset.
Finder _lastRow() => find.ancestor(
  of: find.text(AppLocalizationsId().articleItemNotReceived),
  matching: find.byType(InkWell),
);

ScrollPosition _position(WidgetTester tester) => tester
    .state<ScrollableState>(
      find.descendant(of: _bodyList(), matching: find.byType(Scrollable)).first,
    )
    .position;

/// Puts the list at its scroll END — the state where the last article row
/// rests lowest and therefore bounds "never enters the system region".
Future<void> _scrollToEnd(WidgetTester tester) async {
  expect(_bodyList(), findsOneWidget, reason: 'body ListView not found');
  for (int i = 0; i < 60; i++) {
    final ScrollPosition position = _position(tester);
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(_bodyList(), const Offset(0, -500));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = _position(tester);
  fail(
    'the category list never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the LAST article row — the bottom-most content of the body.
double _lastRowBottom(WidgetTester tester) =>
    tester.getRect(_lastRow()).bottom;

/// Bottom of the body list viewport — the layer that must own the inset.
double _viewportBottom(WidgetTester tester) =>
    tester.getRect(_bodyList()).bottom;

/// maxScrollExtent of the body at the current metrics (premise check).
double _maxExtent(WidgetTester tester) => _position(tester).maxScrollExtent;

/// Total scrollable CONTENT height (extent + viewport). Must stay invariant to
/// the system inset: the inset belongs to the viewport, never to the content.
double _contentExtent(WidgetTester tester) =>
    _maxExtent(tester) + _position(tester).viewportDimension;

void main() {
  testWidgets(
    'short window: the last article row follows the LIVE inset 1:1 and never '
    'enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setShortWindow(tester);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      // The list builds lazily, so the last row only exists once scrolled to.
      await _scrollToEnd(tester);
      expect(
        _lastRow(),
        findsOneWidget,
        reason: 'the last category row was not built at the scroll end',
      );

      // Contract premise: the list overflows one viewport at inset 0 (the last
      // row rides the scroll end). If this fails, the layout regime changed and
      // the scroll-end geometry contract needs a re-audit — not a silent pass.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the category list no longer overflows this short window — '
            're-audit the scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double row0 = _lastRowBottom(tester);
      final double content0 = _contentExtent(tester);

      // Design trailing spacing at scroll end (inset 0): the explicit
      // `AppMetrics.p16` list padding. Measured so design spacing is never
      // mistaken for inset authority and no fixed clearance hides here.
      expect(
        surface - row0,
        closeTo(AppMetrics.p16, 0.01),
        reason:
            'at inset 0 the scroll-end gap below the last article row must be '
            'the design spacing p16 (${AppMetrics.p16}px), got '
            '${surface - row0} — larger would be a fixed bottom clearance, '
            'smaller would eat design spacing',
      );

      // Live insets: the last row moves 1:1 with the system inset.
      final Map<double, double> bottoms = <double, double>{};
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);
        final double row = _lastRowBottom(tester);
        bottoms[inset] = row;

        expect(
          row,
          closeTo(row0 - inset, 0.01),
          reason:
              'the last article row did not follow the live system inset '
              '$inset: bottom $row, expected ${row0 - inset} (inset-0 design '
              '$row0 minus $inset) — the body has no bottom-inset authority',
        );
        expect(
          row,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the last article row entered the system navigation region at '
              'inset $inset (bottom $row, region starts ${surface - inset})',
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
              '(${_contentExtent(tester)} vs $content0) — a nested scrollable '
              'is absorbing the bottom inset instead of the body',
        );
      }

      // A changed inset moves the row 1:1 — no stale gap, no stand-in.
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
    'default view (non-overflow regime, unscrollable list): the last article '
    'row still never enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();
      expect(
        _rows(),
        findsAtLeastNWidgets(2),
        reason: 'the body list did not render its rows',
      );

      await _scrollToEnd(tester);
      expect(_lastRow(), findsOneWidget);
      // Non-overflow premise: at inset 0 the six rows FIT the body viewport,
      // so the list cannot scroll. Whatever sits under the system bar here is
      // unreachable — the sharpest form of the defect this contract forbids.
      expect(
        _maxExtent(tester),
        closeTo(0, 0.01),
        reason:
            'the category list no longer fits the default view — this test '
            'documents the non-overflow regime; re-audit it',
      );

      final double surface = _surfaceBottom(tester);
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double row = _lastRowBottom(tester);
        expect(
          row,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the last article row entered the system navigation region at '
              'inset $inset (bottom $row, region starts ${surface - inset})',
        );
        expect(
          _viewportBottom(tester),
          closeTo(surface - inset, 0.01),
          reason:
              'the body viewport did not shrink by the live inset $inset — '
              'the inset is being consumed somewhere other than the body',
        );
      }
    },
  );

  testWidgets(
    'exactly ONE body SafeArea owns the inset and NO nested scrollable can '
    'absorb it',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 34);
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      // Supplementary structure check ONLY — the geometry tests above are the
      // proof. Exactly ONE SafeArea may wrap the body list: a second/nested one
      // would be a duplicate bottom-inset authority. The AppBar's own framework
      // `SafeArea(bottom: false)` wraps only the app bar, never the list.
      expect(
        find.ancestor(of: _bodyList(), matching: find.byType(SafeArea)),
        findsOneWidget,
        reason: 'exactly ONE SafeArea must wrap the body list — the last '
            'article row must sit under a single bottom-inset authority',
      );
      // Exactly ONE scrollable: with no nested scrollable there is no widget
      // whose implicit MediaQuery padding could re-absorb the inset mid-page.
      expect(
        find.byType(Scrollable),
        findsOneWidget,
        reason: 'a nested scrollable appeared — audit it for implicit inset '
            'absorption before trusting the geometry contract',
      );
    },
  );

  testWidgets('no text input exists — no fabricated keyboard regime', (
    tester,
  ) async {
    addTearDown(tester.view.reset);
    await tester.pumpWidget(_app());
    await tester.pumpAndSettle();

    expect(find.byType(TextField), findsNothing);
    expect(find.byType(EditableText), findsNothing);
  });
}
