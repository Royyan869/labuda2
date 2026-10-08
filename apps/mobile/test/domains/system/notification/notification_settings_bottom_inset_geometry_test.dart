// SAFE-AREA-35 — NOTIFICATION SETTINGS SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the system-window authorities on NotificationSettingsScreen — a
// FLAT top-level GoRoute (ProfileModule → `/settings/notifications`): no
// shell bar, no nested route, no ancestor SafeArea can own any inset.
//
//   * top inset      → Scaffold.appBar: the AppBar claims the status-bar
//     region via its internal `SafeArea(bottom: false)`, and the Scaffold
//     body slot has its top padding removed because appBar != null — so
//     the body starts exactly at the AppBar bottom (no phantom top).
//   * body bottom    → the body `SafeArea` wrapping the ListView. The
//     ListView carries an EXPLICIT `padding: p16`, and BoxScrollView only
//     consumes MediaQuery padding when `padding == null`
//     (scroll_view.dart buildSlivers) — so without the body SafeArea the
//     viewport bottom tracks nothing (LIVE proof: 0/24/34/48 below).
//   * design spacing → the ListView p16 tail plus the last card's p16
//     inner padding = 32 below the last meaningful text, a constant
//     measured BELOW the live inset.
//   * FAB / CTA      → ABSENT: no FloatingActionButton, no
//     bottomNavigationBar, no BottomActionBar (proved below).
//   * keyboard       → N/A: the screen is a read-only placeholder with no
//     text input (proved below).
//
// The screen has ONE static state (no loading/error/empty branches: it is
// a StatelessWidget that calls no backend), so the geometry contract
// covers the single populated body.
//
// Window metrics are injected on the TEST VIEW (series idiom): top inset
// fixed at 24, bottom inset varied.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/system/notification/presentation/screens/notification_settings_screen.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

/// Design tail below the last meaningful text of the populated body:
/// the last card's `Padding(p16)` + the ListView `padding(p16)`.
const double _designTailFromText = 32;

/// Fixed TOP inset of the injected window (series idiom): the status-bar
/// region the AppBar must clear.
const double _topInset = 24;

/// Injects window metrics on the TEST VIEW so every inset below is the
/// REAL, LIVE one.
void _setInsets(WidgetTester tester, {required double bottom}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    top: _topInset * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(
    top: _topInset * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewInsets = const FakeViewPadding();
}

Future<void> _pump(WidgetTester tester, {required double inset}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  await tester.pumpWidget(
    const MaterialApp(home: NotificationSettingsScreen()),
  );
  await tester.pumpAndSettle();
}

ScrollableState _scrollableOf(WidgetTester tester) =>
    tester.state<ScrollableState>(
      find
          .descendant(
            of: find.byType(ListView),
            matching: find.byType(Scrollable),
          )
          .first,
    );

/// Scrolls to the REAL end: `maxScrollExtent` is an estimate until the
/// children at the new offset are laid out, so re-jump until stable.
Future<void> _scrollToEnd(WidgetTester tester) async {
  final ScrollableState scrollable = _scrollableOf(tester);
  for (var i = 0; i < 4; i++) {
    scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
    await tester.pump();
  }
}

/// Measured geometry of the single body at [inset], after scrolling to
/// the very end. The last MEANINGFUL content is the final text of the
/// "What changed" card — measured as the text itself, never as a
/// container bottom.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  expect(find.byType(AppBar), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect scrollBox = tester.getRect(find.byType(ListView));
  final Rect appBar = tester.getRect(find.byType(AppBar));
  final Rect title = tester.getRect(find.text('Notification Settings'));

  await _scrollToEnd(tester);

  final Finder lastText = find.textContaining(
    'any unsupported preference updates',
  );
  expect(lastText, findsOneWidget, reason: 'the card must be rendered');
  final Rect last = tester.getRect(lastText);

  final ScrollableState scrollable = _scrollableOf(tester);
  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surfaceBox.bottom - inset;
  final double systemGap = regionStart - last.bottom;
  final double scrollGap = scrollBox.bottom - last.bottom;
  final double reachable =
      (last.bottom <= scrollBox.bottom + 0.01 &&
          last.top >= scrollBox.top - 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastTextBottom=${last.bottom.toStringAsFixed(2)} '
    'scrollTop=${scrollBox.top.toStringAsFixed(2)} '
    'scrollBottom=${scrollBox.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'systemGap=$systemGap scrollGap=$scrollGap reachable=$reachable '
    'appBarTop=${appBar.top.toStringAsFixed(2)} '
    'appBarBottom=${appBar.bottom.toStringAsFixed(2)} '
    'titleTop=${title.top.toStringAsFixed(2)}',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'lastTextBottom': last.bottom,
    'scrollTop': scrollBox.top,
    'scrollBottom': scrollBox.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'systemGap': systemGap,
    'scrollGap': scrollGap,
    'reachable': reachable,
    'appBarTop': appBar.top,
    'appBarBottom': appBar.bottom,
    'titleTop': title.top,
  };
}

/// BODY contract at [inset]: the live body SafeArea owns the bottom, the
/// Scaffold AppBar owns the top, and the design tail sits below the live
/// system region.
Future<void> _expectGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measure(tester, inset: inset);

  // — body bottom authority —
  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the list viewport bottom must follow the live system inset '
        '($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset (measured ${m['scrollBottom']} vs surface '
        '${m['surfaceBottom']})',
  );

  expect(
    m['systemGap'],
    closeTo(_designTailFromText, 0.01),
    reason:
        'at scroll end the gap from the last meaningful text to the '
        'system region must be exactly the design tail '
        '(card p16 + list p16 = $_designTailFromText) at inset $inset: '
        'negative means the text entered the system region '
        '(${m['systemGap']}), smaller means the live inset is missing',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last meaningful text (${m['lastTextBottom']}) must be fully '
        'visible inside the viewport at scroll end at inset $inset',
  );

  // — top authority: Scaffold.appBar claims the status-bar region, the
  // body starts exactly at the AppBar bottom —
  expect(
    m['scrollTop'],
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the body must start exactly at the AppBar bottom — a phantom top '
        'reservation would push the list down (scrollTop '
        '${m['scrollTop']} vs appBarBottom ${m['appBarBottom']})',
  );

  expect(
    m['appBarTop'],
    closeTo(0, 0.01),
    reason:
        'the AppBar must start at the top of the surface '
        '(measured ${m['appBarTop']})',
  );

  expect(
    m['appBarBottom']! - m['appBarTop']!,
    closeTo(kToolbarHeight + _topInset, 0.01),
    reason:
        'the AppBar must reserve the top inset: toolbar '
        '(${kToolbarHeight.toStringAsFixed(0)}) + status bar '
        '($_topInset) (measured ${m['appBarBottom']! - m['appBarTop']!})',
  );

  expect(
    m['titleTop'],
    greaterThanOrEqualTo(_topInset - 0.01),
    reason:
        'the bar title must sit below the status-bar region '
        '(title top ${m['titleTop']})',
  );
}

void main() {
  group('SAFE-AREA-35 — populated body: live inset geometry', () {
    testWidgets('inset 0 — design tail only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0);
      await _expectGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last text clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24);
      await _expectGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last text clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34);
      await _expectGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last text clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48);
      await _expectGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-35 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space)',
      (tester) async {
        await _pump(tester, inset: 0);
        final Map<String, double> at0 = await _measure(tester, inset: 0);

        // System bar appears (48 px): a live body authority moves the
        // viewport exactly once.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measure(tester, inset: 48);

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason:
              'the list viewport bottom must follow the system inset — '
              'a fixed clearance would not move',
        );

        expect(
          at48['viewport']!,
          closeTo(at0['viewport']! - 48, 0.01),
          reason: 'the viewport must shrink exactly by the live inset',
        );

        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason:
              'the content extent must stay constant across insets — '
              'growth would prove a second, duplicate inset absorption',
        );

        // Exactly ONE canonical SafeArea authority owns the BODY list
        // geometry: count SafeArea ANCESTORS of the ListView.
        int safeAreaAncestors = 0;
        tester.element(find.byType(ListView)).visitAncestorElements((element) {
          if (element.widget is SafeArea) safeAreaAncestors++;
          return true;
        });
        expect(
          safeAreaAncestors,
          1,
          reason:
              'exactly one SafeArea must sit between the ListView and the '
              'root — no duplicate ownership, none missing',
        );
      },
    );

    testWidgets('the screen owns no FAB, no bottom bar, no text input '
        '(their absence is the proof)', (tester) async {
      await _pump(tester, inset: 34);

      expect(
        find.byType(FloatingActionButton),
        findsNothing,
        reason: 'the screen owns no FAB — no FAB inset authority',
      );
      expect(
        find.byType(NavigationBar),
        findsNothing,
        reason: 'the screen route carries no bottom navigation bar',
      );
      expect(
        find.byType(BottomActionBar),
        findsNothing,
        reason: 'the screen owns no BottomActionBar CTA',
      );
      expect(
        find.byType(TextField),
        findsNothing,
        reason:
            'the screen owns no text input — keyboard inset is not '
            'in scope for this screen',
      );
      expect(
        find.byType(TextFormField),
        findsNothing,
        reason: 'the screen owns no form input',
      );
    });

    testWidgets('the production screen holds no second inset authority', (
      tester,
    ) async {
      await _pump(tester, inset: 34);

      final String src = File(
        'lib/domains/system/notification/presentation/screens/'
        'notification_settings_screen.dart',
      ).readAsStringSync();

      expect(
        'SafeArea('.allMatches(src).length,
        1,
        reason:
            'the screen must own exactly one SafeArea authority — no '
            'duplicate ownership, none missing',
      );
      for (final token in const <String>[
        'MediaQuery',
        'viewPadding',
        'viewInsets',
        'bottomNavigationBar',
        'bottomBarClearance',
        'fabClearance',
        'FloatingActionButton',
      ]) {
        expect(
          token.allMatches(src).length,
          0,
          reason:
              'the screen must not carry a second inset authority or a '
              'fixed clearance ($token)',
        );
      }
    });
  });
}
