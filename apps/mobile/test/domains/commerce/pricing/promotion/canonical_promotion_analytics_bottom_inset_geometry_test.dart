// SAFE-AREA-39 — CANONICAL PROMOTION ANALYTICS: BOTTOM-INSET GEOMETRY.
//
// Locks the system-window authorities on CanonicalPromotionAnalyticsScreen —
// a FLAT top-level GoRoute (SellerModule →
// `/seller/promotions/:contractId/analytics`): no shell bar, no nested
// route, no ancestor SafeArea/MediaQuery can own any inset for it
// (MaterialApp.builder chain: SessionLifecycleObserver →
// NotificationInitializer → KeyboardDismissWrapper (a GestureDetector) →
// NavigationScope — free of inset authorities).
//
//   * top inset         → the Scaffold `AppBar` ('Promotion Analytics'). The
//     bar reserves `MediaQuery.paddingOf(context).top` in its preferred
//     height and runs the framework's own `SafeArea(bottom: false)`
//     (app_bar.dart:1194), so the toolbar/title sit below the status bar
//     while the body starts exactly at the bar bottom. No phantom top.
//   * body bottom inset  → the body `SafeArea` wrapping the whole state
//     branch (LIVE: the scroll viewport bottom tracks 0/24/34/48). The
//     populated branch is a `SingleChildScrollView` with an EXPLICIT
//     `padding` (design p16); the generic ScrollView performs NO
//     MediaQuery auto-consumption at all (only BoxScrollView.buildSlivers
//     does, and only when `padding == null`) — without the body SafeArea
//     nothing owns the bottom inset (the baseline Outcome-A defect).
//   * design spacing     → the scroll `padding` tail (p16 below the last
//     content block at scroll end) — constant, measured BELOW the live
//     inset.
//   * FAB/CTA            → NONE: the screen owns no floatingActionButton,
//     no bottomNavigationBar, no BottomActionBar (absence proved below).
//     The 'Retry' button lives INSIDE the centered error fill, so it
//     inherits the body SafeArea authority.
//   * keyboard           → N/A: the screen owns no text input (absence
//     proved below).
//   * BottomSheet        → this screen calls NO bottom sheet (recorded as a
//     scope boundary: the create/queue product pickers do, on other
//     screens, under their own authorities).
//
// The non-scrolling branches (loading / error) are Center fills; each must
// keep its content outside the system region and below the app bar at
// every inset.
//
// Geometry is measured on the REAL screen with injected window metrics
// (top inset fixed at 24, bottom inset varied — the series idiom).
import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_analytics_screen.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

/// Design tail below the last meaningful content of the populated branch:
/// `SingleChildScrollView(padding: EdgeInsets.all(AppMetrics.p16))`.
const double _designTail = 16;

/// Fixed TOP inset of the injected window (series idiom): the status-bar
/// region the AppBar must clear.
const double _topInset = 24;

enum _Branch { populated, loading, error }

/// Stateful fake backend for GET /promotions/contracts/:id/analytics.
class _AnalyticsGeoClient implements ApiClient {
  final int included;
  final int impressions;
  final int clicks;
  final Exception? error;
  final Completer<void>? _gate;

  _AnalyticsGeoClient({
    required this.included,
    required this.impressions,
    required this.clicks,
    this.error,
    Completer<void>? gate,
  }) : _gate = gate;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    if (error case final e?) throw e;
    final gate = _gate;
    if (gate != null) await gate.future;
    return Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          data: <String, dynamic>{
            'success': true,
            'data': <String, dynamic>{
              'contract_id': 'ctr-1',
              'included_count': included,
              'impression_count': impressions,
              'click_count': clicks,
            },
          },
          statusCode: 200,
        )
        as Response<T>;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// Injects window metrics on the TEST VIEW (series idiom) so every inset
/// below is the REAL, LIVE one.
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

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  final _AnalyticsGeoClient client = switch (branch) {
    _Branch.populated => _AnalyticsGeoClient(
      included: 12,
      impressions: 34,
      clicks: 5,
    ),
    _Branch.error => _AnalyticsGeoClient(
      included: 0,
      impressions: 0,
      clicks: 0,
      error: Exception('boom'),
    ),
    _Branch.loading => _AnalyticsGeoClient(
      included: 0,
      impressions: 0,
      clicks: 0,
      gate: Completer<void>(),
    ),
  };

  await tester.pumpWidget(
    ProviderScope(
      // No retry: a failing static load must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [apiClientProvider.overrideWithValue(client)],
      child: const MaterialApp(
        home: CanonicalPromotionAnalyticsScreen(contractId: 'ctr-1'),
      ),
    ),
  );

  if (branch == _Branch.loading) {
    // The loading fill renders an indeterminate spinner, which
    // pumpAndSettle would wait on forever; bounded frames settle layout.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 100));
  } else {
    await tester.pumpAndSettle();
  }
}

ScrollableState _scrollableOf(WidgetTester tester) =>
    tester.state<ScrollableState>(
      find
          .descendant(
            of: find.byType(SingleChildScrollView),
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

/// The last content block of the populated branch: the "About These
/// Metrics" info `Container(padding: p16, decoration: BoxDecoration)` —
/// `.last` in DFS order, i.e. the last meaningful analytics content.
Finder _contentBlocks() => find.byWidgetPredicate(
  (w) =>
      w is Container &&
      w.padding == const EdgeInsets.all(AppMetrics.p16) &&
      w.decoration is BoxDecoration,
  description: 'analytics content block',
);

/// Measured geometry of the POPULATED branch at [inset], after scrolling
/// to the very end.
Future<Map<String, double>> _measurePopulated(
  WidgetTester tester, {
  required double inset,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect scrollBox = tester.getRect(find.byType(SingleChildScrollView));
  expect(find.byType(AppBar), findsOneWidget);
  final Rect appBar = tester.getRect(find.byType(AppBar));
  final Rect title = tester.getRect(find.text('Promotion Analytics'));

  await _scrollToEnd(tester);

  expect(_contentBlocks(), findsWidgets);
  final double blockBottom = tester.getRect(_contentBlocks().last).bottom;

  final ScrollableState scrollable = _scrollableOf(tester);
  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surfaceBox.bottom - inset;
  final double gap = regionStart - blockBottom;
  final double reachable =
      (blockBottom <= scrollBox.bottom + 0.01 && blockBottom >= scrollBox.top)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM branch=populated inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'blockBottom=${blockBottom.toStringAsFixed(2)} '
    'scrollTop=${scrollBox.top.toStringAsFixed(2)} '
    'scrollBottom=${scrollBox.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap reachable=$reachable '
    'appBarTop=${appBar.top.toStringAsFixed(2)} '
    'appBarBottom=${appBar.bottom.toStringAsFixed(2)} '
    'titleTop=${title.top.toStringAsFixed(2)}',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'blockBottom': blockBottom,
    'scrollTop': scrollBox.top,
    'scrollBottom': scrollBox.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'reachable': reachable,
    'appBarTop': appBar.top,
    'appBarBottom': appBar.bottom,
    'titleTop': title.top,
  };
}

/// Populated-state contract at [inset]: the body SafeArea owns the
/// bottom, the Scaffold AppBar owns the top, and the design tail sits
/// below the live system region.
Future<void> _expectPopulatedGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measurePopulated(tester, inset: inset);

  // — body bottom authority —
  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the analytics scroll viewport bottom must follow the live system '
        'inset ($inset) — the body SafeArea, not the explicit design '
        'padding, owns the bottom inset (measured ${m['scrollBottom']} vs '
        'surface ${m['surfaceBottom']})',
  );

  expect(
    m['gap'],
    closeTo(_designTail, 0.01),
    reason:
        'at scroll end the gap to the system region must be exactly the '
        'design scroll padding ($_designTail) at inset $inset: negative '
        'means the last content block enters the system region '
        '(${m['gap']}), larger means a fixed clearance standing in for the '
        'live inset',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason:
        'the analytics content must actually scroll for the end contract '
        'to hold (maxScroll ${m['maxScroll']})',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last content block (${m['blockBottom']}) must be fully '
        'reachable inside the viewport at scroll end at inset $inset',
  );

  // — top authority: the Scaffold AppBar —
  expect(
    m['appBarTop'],
    closeTo(0, 0.01),
    reason:
        'the app bar must start at the surface top (measured '
        '${m['appBarTop']})',
  );

  expect(
    m['appBarBottom']! - m['appBarTop']!,
    closeTo(kToolbarHeight + _topInset, 0.01),
    reason:
        'the AppBar must reserve the top inset: height = toolbar '
        '(${kToolbarHeight.toStringAsFixed(0)}) + status bar ($_topInset) '
        '(measured ${m['appBarBottom']! - m['appBarTop']!})',
  );

  expect(
    m['titleTop'],
    greaterThanOrEqualTo(_topInset - 0.01),
    reason:
        'the bar title must sit below the status-bar region '
        '(title top ${m['titleTop']})',
  );

  expect(
    m['scrollTop'],
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the analytics content must start exactly at the AppBar bottom — '
        'no phantom top reservation between bar and body (top '
        '${m['scrollTop']}, bar bottom ${m['appBarBottom']})',
  );
}

/// Measured geometry of a FILL branch at [inset]: the centered content
/// must stay clear of the system region and of the app bar.
Future<Map<String, double>> _measureFill(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  expect(find.byType(AppBar), findsOneWidget);
  final Rect appBar = tester.getRect(find.byType(AppBar));

  late final Finder firstFinder;
  late final Finder lastFinder;
  switch (branch) {
    case _Branch.loading:
      firstFinder = find.byType(CircularProgressIndicator);
      lastFinder = find.text('Loading analytics...');
    case _Branch.error:
      firstFinder = find.byIcon(Icons.error_outline);
      lastFinder = find.widgetWithText(ElevatedButton, 'Retry');
    case _Branch.populated:
      throw StateError('the populated branch has its own measurement');
  }

  final Rect first = tester.getRect(firstFinder);
  final Rect last = tester.getRect(lastFinder);

  final double regionStart = surfaceBox.bottom - inset;
  final double gap = regionStart - last.bottom;
  final double topGap = first.top - appBar.bottom;

  // ignore: avoid_print
  print(
    'GEOM branch=${branch.name} inset=$inset '
    'firstTop=${first.top.toStringAsFixed(2)} '
    'lastBottom=${last.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap topGap=${topGap.toStringAsFixed(2)}',
  );

  return <String, double>{
    'firstTop': first.top,
    'lastBottom': last.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'topGap': topGap,
  };
}

Future<void> _expectFillGeometry(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  final Map<String, double> m = await _measureFill(
    tester,
    inset: inset,
    branch: branch,
  );

  expect(
    m['lastBottom'],
    lessThanOrEqualTo(m['regionStart']! - 0.01),
    reason:
        'the ${branch.name} fill content (${m['lastBottom']}) must stay '
        'outside the system region (start ${m['regionStart']}) at inset '
        '$inset',
  );

  expect(
    m['topGap'],
    greaterThanOrEqualTo(-0.01),
    reason:
        'the ${branch.name} fill content must not intrude into the '
        'app-bar boundary (gap ${m['topGap']})',
  );
}

void main() {
  group('SAFE-AREA-39 — populated analytics: live inset geometry', () {
    for (final inset in const <double>[0, 24, 34, 48]) {
      testWidgets('inset $inset — viewport, end, design tail', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.populated);
        await _expectPopulatedGeometry(tester, inset: inset);
      });
    }
  });

  group('SAFE-AREA-39 — fill branches: live inset geometry', () {
    for (final inset in const <double>[0, 24, 34, 48]) {
      testWidgets('loading at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.loading);
        await _expectFillGeometry(
          tester,
          inset: inset,
          branch: _Branch.loading,
        );
      });

      testWidgets('error at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.error);
        await _expectFillGeometry(tester, inset: inset, branch: _Branch.error);
      });
    }
  });

  group('SAFE-AREA-39 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space)',
      (tester) async {
        await _pump(tester, inset: 0, branch: _Branch.populated);
        final Map<String, double> at0 = await _measurePopulated(
          tester,
          inset: 0,
        );

        // System bar appears (48 px): a live body authority moves the
        // viewport exactly once.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));
        final Map<String, double> at48 = await _measurePopulated(
          tester,
          inset: 48,
        );

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason:
              'the analytics viewport bottom must follow the system inset — '
              'a fixed clearance would not move '
              '(${at0['scrollBottom']} → ${at48['scrollBottom']})',
        );

        expect(
          at48['viewport']!,
          closeTo(at0['viewport']! - 48, 0.01),
          reason:
              'the viewport must shrink exactly by the live inset '
              '(${at0['viewport']} → ${at48['viewport']})',
        );

        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason:
              'the content extent must stay constant across insets — '
              'growth would prove a second, duplicate inset absorption',
        );

        // Exactly ONE canonical SafeArea authority owns the BODY scroll
        // geometry: count SafeArea ANCESTORS of the scroll view.
        int safeAreaAncestors = 0;
        tester
            .element(find.byType(SingleChildScrollView))
            .visitAncestorElements((element) {
              if (element.widget is SafeArea) safeAreaAncestors++;
              return true;
            });
        expect(
          safeAreaAncestors,
          1,
          reason:
              'exactly one SafeArea must sit between the scroll view and '
              'the root — no duplicate ownership, none missing',
        );
      },
    );

    testWidgets('the screen owns no FAB, no bottom bar, no refresh overlay, no '
        'second scrollable, no text input (their absence is the proof)', (
      tester,
    ) async {
      await _pump(tester, inset: 34, branch: _Branch.populated);

      expect(
        find.byType(FloatingActionButton),
        findsNothing,
        reason: 'the screen owns no FAB — no FAB inset authority exists',
      );
      expect(
        find.byType(NavigationBar),
        findsNothing,
        reason: 'the route carries no bottom navigation bar',
      );
      expect(
        find.byType(BottomAppBar),
        findsNothing,
        reason: 'the screen owns no bottom app bar',
      );
      expect(
        find.byType(BottomActionBar),
        findsNothing,
        reason: 'the screen owns no BottomActionBar CTA',
      );
      expect(
        find.byType(RefreshIndicator),
        findsNothing,
        reason: 'the screen owns no refresh overlay authority',
      );
      expect(
        find.byType(ListView),
        findsNothing,
        reason: 'the body scroll is ONE SingleChildScrollView',
      );
      expect(
        find.byType(Scrollable),
        findsOneWidget,
        reason: 'exactly one scrollable exists on the screen',
      );
      expect(
        find.byType(TextField),
        findsNothing,
        reason:
            'the screen owns no text input — keyboard inset is not in '
            'scope',
      );
      expect(
        find.byType(TextFormField),
        findsNothing,
        reason: 'the screen owns no form input',
      );
    });

    testWidgets('the production screen holds exactly one inset authority', (
      tester,
    ) async {
      await _pump(tester, inset: 34, branch: _Branch.populated);

      final String src = File(
        'lib/domains/commerce/pricing/promotion/presentation/screens/'
        'canonical_promotion_analytics_screen.dart',
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
        'debugPrint',
        'print(',
        'TODO',
        'probe',
        'ratchet',
      ]) {
        expect(
          token.allMatches(src).length,
          0,
          reason:
              'the screen must not carry a second inset authority or '
              'a fixed clearance ($token)',
        );
      }

      // No fixed bottom spacer standing in for the live inset. (`fallback`
      // is deliberately NOT banned: a default error-message argument is
      // not an inset authority.)
      expect(
        RegExp(r'SizedBox\(\s*height:\s*(?:80|96|100)\b').hasMatch(src),
        isFalse,
        reason: 'a fixed inset-sized spacer must not come back',
      );
      expect(
        src.contains('Space for bottom'),
        isFalse,
        reason: 'the clearance comment idiom must not come back',
      );
    });
  });
}
