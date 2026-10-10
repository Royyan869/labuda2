// SAFE-AREA-25 — HELP ARTICLE SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the invariant on the `/help/article` surface:
//   * the "Was this helpful" block is the LAST meaningful content of the body
//     `SingleChildScrollView` and it carries the Yes/No actions, so the
//     body-level `SafeArea` is the ONE authority that consumes the live system
//     bottom inset — never a body-side second reservation, never a fixed
//     clearance, never manual inset arithmetic;
//   * the block consumes the REAL, LIVE inset: 0 → 24 → 34 → 48 moves it 1:1,
//     the body scroll viewport shrinks by exactly the inset, and neither the
//     block nor its Yes/No buttons enter the system navigation region;
//   * the scroll CONTENT extent stays inset-invariant: this screen has NO
//     nested scrollable (badge, Texts and the helpful block only), so no
//     mid-page widget re-absorbs the inset. Measured pre-fix the inset was
//     consumed by NOTHING (content extent 3942 at every inset) and the block
//     rested on the 24 px design padding alone, which put it 10 px / 24 px
//     INSIDE the system region at inset 34 / 48 and the buttons 8 px inside
//     at inset 48;
//   * the trailing 24 px below the block at inset 0 is the DESIGN spacing (the
//     scroll view's explicit `AppMetrics.p24` padding) — measured separately
//     so design spacing is never mistaken for inset authority;
//   * a NON-OVERFLOWING article is covered too: the block stays clear of the
//     system region, and the scroll view's own bottom never dips into it.
//     (Note the framework detail proven here: `_SingleChildViewport` sets
//     `size = constraints.constrain(child.size)`, so a short article reports a
//     viewport equal to its content height and `maxExtent == 0`.)
//
// KEYBOARD: the screen has NO text input (no `TextField` / `EditableText`),
// so no user-driven keyboard interaction exists — no keyboard regime is
// fabricated here; the contract asserts that absence instead.
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

/// A long article: overflows the default 800×600 test view (content extent
/// ~3942 px), so the helpful block rides the scroll end.
HelpArticle _longArticle() => HelpArticle(
  title: 'Judul Artikel Panjang',
  category: 'Pesanan',
  content: List.filled(
    60,
    'Paragraf isi artikel yang panjang untuk memaksa halaman melakukan '
    'scroll pada viewport normal.',
  ).join(' '),
);

/// A short article: fits the default view (maxExtent == 0).
HelpArticle _shortArticle() => HelpArticle(
  title: 'Judul Pendek',
  category: 'Pembayaran',
  content: 'Isi sangat pendek.',
);

Widget _app(HelpArticle article) => MaterialApp(
  theme: AppTheme.lightTheme,
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  locale: const Locale('id'),
  home: HelpArticleScreen(article: article),
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

/// Short window (800×360 logical at dpr 1): landscape / split-screen regime
/// where the article overflows at every inset.
void _setShortWindow(WidgetTester tester) {
  tester.view.devicePixelRatio = 1.0;
  tester.view.physicalSize = const Size(800, 360);
}

Finder _bodyScroll() => find.byType(SingleChildScrollView);

ScrollPosition _position(WidgetTester tester) => tester
    .state<ScrollableState>(
      find
          .descendant(of: _bodyScroll(), matching: find.byType(Scrollable))
          .first,
    )
    .position;

/// Puts the body at its scroll END — the state where the "Was this helpful"
/// block rests lowest and therefore bounds "never enters the system region".
Future<void> _scrollToEnd(WidgetTester tester) async {
  expect(_bodyScroll(), findsOneWidget, reason: 'body scroll view not found');
  for (int i = 0; i < 80; i++) {
    final ScrollPosition position = _position(tester);
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(_bodyScroll(), const Offset(0, -700));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = _position(tester);
  fail(
    'the article body never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom of the "Was this helpful" block — the last meaningful content
/// (it carries the Yes/No actions).
double _helpfulBottom(WidgetTester tester) => tester
    .getRect(
      find.ancestor(
        of: find.text(AppLocalizationsId().wasThisHelpful),
        matching: find.byType(Container),
      ),
    )
    .bottom;

/// Bottom edge of the Yes/No row inside that block.
double _buttonsBottom(WidgetTester tester) =>
    tester.getRect(find.byType(OutlinedButton).last).bottom;

/// Bottom of the body scroll viewport — the layer that must own the inset.
double _scrollRectBottom(WidgetTester tester) =>
    tester.getRect(_bodyScroll()).bottom;

/// maxScrollExtent of the body at the current metrics (premise check).
double _maxExtent(WidgetTester tester) => _position(tester).maxScrollExtent;

/// Total scrollable CONTENT height (extent + viewport). Must stay invariant to
/// the system inset: the inset belongs to the viewport, never to the content.
double _contentExtent(WidgetTester tester) =>
    _maxExtent(tester) + _position(tester).viewportDimension;

void main() {
  testWidgets(
    'overflow regime (long article, default view): the helpful block follows '
    'the LIVE inset 1:1 and never enters the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app(_longArticle()));
      await tester.pumpAndSettle();

      expect(find.byType(OutlinedButton), findsNWidgets(2));
      await _scrollToEnd(tester);

      // Contract premise: the article overflows one viewport at inset 0 (the
      // helpful block rides the scroll end). If this fails, the layout regime
      // changed and the scroll-end geometry contract needs a re-audit.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the article no longer overflows the default view — re-audit the '
            'scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double block0 = _helpfulBottom(tester);
      final double content0 = _contentExtent(tester);

      // inset 0 → design spacing only: the explicit `AppMetrics.p24` scroll
      // padding. No phantom inset reservation, no fixed clearance.
      expect(
        surface - block0,
        closeTo(AppMetrics.p24, 0.01),
        reason:
            'at inset 0 the scroll-end gap below the "Was this helpful" block '
            'must be the design spacing p24 (${AppMetrics.p24}px), got '
            '${surface - block0} — larger would be a fixed bottom clearance, '
            'smaller would eat design spacing',
      );

      // Live insets: the block moves 1:1 with the system inset.
      final Map<double, double> bottoms = <double, double>{};
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double block = _helpfulBottom(tester);
        final double buttons = _buttonsBottom(tester);
        bottoms[inset] = block;

        expect(
          block,
          closeTo(block0 - inset, 0.01),
          reason:
              'the "Was this helpful" block did not follow the live system '
              'inset $inset: bottom $block, expected ${block0 - inset} '
              '(inset-0 design $block0 minus $inset) — the body has no '
              'bottom-inset authority',
        );
        expect(
          block,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the "Was this helpful" block entered the system navigation '
              'region at inset $inset (bottom $block, region starts '
              '${surface - inset})',
        );
        expect(
          buttons,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the Yes/No actions were pushed under the system navigation '
              'region at inset $inset (bottom $buttons, region starts '
              '${surface - inset})',
        );
        expect(
          _scrollRectBottom(tester),
          closeTo(surface - inset, 0.01),
          reason:
              'the body scroll viewport did not shrink by the live inset '
              '$inset — the inset is being consumed somewhere other than the '
              'body',
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

      // A changed inset moves the block 1:1 — no stale gap, no stand-in.
      expect(
        bottoms[24]! - bottoms[48]!,
        closeTo(24, 0.01),
        reason:
            'did not track a changed system inset 1:1 '
            '(24→48 delta was ${bottoms[24]! - bottoms[48]!})',
      );
      expect(
        bottoms[34]! - bottoms[48]!,
        closeTo(14, 0.01),
        reason:
            'did not track a changed system inset 1:1 '
            '(34→48 delta was ${bottoms[34]! - bottoms[48]!})',
      );
    },
  );

  testWidgets(
    'short window (800×360): same contract — the block and the actions never '
    'enter the system region',
    (tester) async {
      addTearDown(tester.view.reset);
      _setShortWindow(tester);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app(_longArticle()));
      await tester.pumpAndSettle();
      await _scrollToEnd(tester);

      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason:
            'the article no longer overflows this short window — re-audit the '
            'scroll-end geometry contract for this screen',
      );

      final double surface = _surfaceBottom(tester);
      final double block0 = _helpfulBottom(tester);
      expect(
        surface - block0,
        closeTo(AppMetrics.p24, 0.01),
        reason:
            'design spacing below the block must be p24, got '
            '${surface - block0}',
      );

      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double block = _helpfulBottom(tester);
        expect(
          block,
          closeTo(block0 - inset, 0.01),
          reason:
              'short window: the block did not follow the live inset '
              '$inset (bottom $block, expected ${block0 - inset})',
        );
        expect(
          block,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'short window: the "Was this helpful" block entered the '
              'system navigation region at inset $inset (bottom $block, '
              'region starts ${surface - inset})',
        );
        expect(
          _buttonsBottom(tester),
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'short window: the Yes/No actions were pushed under the '
              'system navigation region at inset $inset',
        );
        expect(
          _scrollRectBottom(tester),
          closeTo(surface - inset, 0.01),
          reason:
              'short window: the body scroll viewport did not shrink by '
              'the live inset $inset',
        );
      }
    },
  );

  testWidgets(
    'non-overflow regime (short article): the block stays clear of the system '
    'region and the scroll view itself never dips into it',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await tester.pumpWidget(_app(_shortArticle()));
      await tester.pumpAndSettle();
      await _scrollToEnd(tester);

      // Non-overflow premise: this content fits, so the block is top-anchored
      // and the geometry must be safe WITHOUT relying on a scroll.
      expect(
        _maxExtent(tester),
        closeTo(0, 0.01),
        reason:
            'the short article no longer fits the default view — re-audit '
            'this non-overflow regime',
      );

      final double surface = _surfaceBottom(tester);
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();
        await _scrollToEnd(tester);

        final double block = _helpfulBottom(tester);
        expect(
          block,
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the "Was this helpful" block entered the system navigation '
              'region at inset $inset (bottom $block, region starts '
              '${surface - inset})',
        );
        expect(
          _scrollRectBottom(tester),
          lessThanOrEqualTo(surface - inset + 0.01),
          reason:
              'the body scroll view extended into the system navigation '
              'region at inset $inset (bottom ${_scrollRectBottom(tester)}, '
              'region starts ${surface - inset})',
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
      await tester.pumpWidget(_app(_longArticle()));
      await tester.pumpAndSettle();

      // Supplementary structure check ONLY — the geometry tests above are the
      // proof. Exactly ONE SafeArea may wrap the body scroll view: a
      // second/nested one would be a duplicate bottom-inset authority. The
      // AppBar's own framework `SafeArea(bottom: false)` wraps only the app
      // bar, never the scroll view, so it is not counted here.
      expect(
        find.ancestor(of: _bodyScroll(), matching: find.byType(SafeArea)),
        findsOneWidget,
        reason:
            'exactly ONE SafeArea must wrap the body scroll view — the '
            '"Was this helpful" block must sit under a single bottom-inset '
            'authority',
      );
      // Exactly ONE scrollable: with no nested scrollable there is no widget
      // whose implicit MediaQuery padding could re-absorb the inset mid-page.
      expect(
        find.byType(Scrollable),
        findsOneWidget,
        reason:
            'a nested scrollable appeared — audit it for implicit inset '
            'absorption before trusting the geometry contract',
      );
    },
  );

  testWidgets('no text input exists — no fabricated keyboard regime', (
    tester,
  ) async {
    addTearDown(tester.view.reset);
    await tester.pumpWidget(_app(_longArticle()));
    await tester.pumpAndSettle();

    expect(find.byType(TextField), findsNothing);
    expect(find.byType(EditableText), findsNothing);
  });
}
