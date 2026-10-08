// SAFE-AREA-36 — NOTIFICATION LIST SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the system-window authorities on NotificationListScreen — a FLAT
// top-level GoRoute (ProfileModule → `/notifications`): no shell bar, no
// nested route, no ancestor SafeArea can own any inset.
//
//   * top inset      → Scaffold.appBar (AppBar claims the status-bar
//     region via its internal `SafeArea(bottom: false)`; the body slot
//     has its top padding removed because appBar != null). The body's
//     first element — the horizontal filter rail — must start exactly at
//     the AppBar bottom (no phantom top).
//   * body bottom    → the body `SafeArea` wrapping the Column
//     (filter rail + Expanded body). The populated list carries an
//     EXPLICIT `padding: symmetric(vertical: p12)`, and BoxScrollView
//     only consumes window padding when `padding == null`
//     (scroll_view.dart buildSlivers) — so before this authority the
//     viewport bottom tracked nothing.
//   * design spacing → the list's vertical p12 tail below the last
//     notification tile, a constant measured BELOW the live inset.
//   * FAB / CTA      → ABSENT: no FloatingActionButton, no
//     bottomNavigationBar, no BottomActionBar (proved below).
//   * keyboard       → N/A: the screen owns no text input (proved below).
//
// Branches with a DIFFERENT geometry surface are covered separately:
// the populated list (scrollable) and the centered fill states (loading /
// initial error / empty) each prove their content stays outside the
// system region at every inset.
//
// Window metrics are injected on the TEST VIEW (series idiom): top inset
// fixed at 24, bottom inset varied.
import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/domains/system/notification/notification.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';

const _uid = 'u1';

/// Design tail below the last notification tile: the list
/// `padding: EdgeInsets.symmetric(vertical: AppMetrics.p12)`.
const double _designTail = 12;

/// Fixed TOP inset of the injected window (series idiom): the status-bar
/// region the AppBar must clear.
const double _topInset = 24;

enum _Branch { populated, loading, empty, error }

/// Static repository on the geometry path only: the list stream and the
/// unread-count stream are scripted; everything else fails loudly via
/// noSuchMethod instead of being masked.
class _StaticNotificationRepository implements INotificationRepository {
  _StaticNotificationRepository({required this.onGetNotifications});

  final Stream<Result<List<NotificationEntity>>> Function(int call)
  onGetNotifications;

  int listCalls = 0;

  @override
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) {
    listCalls++;
    return onGetNotifications(listCalls);
  }

  @override
  Stream<Result<int>> getUnreadCount({required String userId}) =>
      const Stream.empty();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

List<NotificationEntity> _notifications(int count) => [
  for (var i = 1; i <= count; i++)
    NotificationEntity(
      id: 'n$i',
      userId: _uid,
      type: NotificationType.orderCreated,
      title: 'Title n$i',
      body: 'Body n$i',
      isRead: false,
      createdAt: DateTime(2026, 1, 1),
    ),
];

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

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  final Completer<void> gate = Completer<void>();
  final _StaticNotificationRepository repository;
  switch (branch) {
    case _Branch.populated:
      repository = _StaticNotificationRepository(
        onGetNotifications: (_) =>
            Stream.value(Result.success(_notifications(10))),
      );
    case _Branch.empty:
      repository = _StaticNotificationRepository(
        onGetNotifications: (_) =>
            Stream.value(Result.success(const <NotificationEntity>[])),
      );
    case _Branch.error:
      repository = _StaticNotificationRepository(
        onGetNotifications: (_) =>
            Stream.value(Result.error('INTERNAL_SERVER_ERROR: boom')),
      );
    case _Branch.loading:
      // Never emits: the screen stays on the canonical initial loading.
      repository = _StaticNotificationRepository(
        onGetNotifications: (_) async* {
          await gate.future;
          yield Result.success(_notifications(1));
        },
      );
  }

  await tester.pumpWidget(
    ProviderScope(
      // No retry: a static load must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [notificationRepositoryProvider.overrideWithValue(repository)],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const NotificationListScreen(userId: _uid),
      ),
    ),
  );

  if (branch == _Branch.loading) {
    // The loading fill renders an indeterminate spinner, which
    // pumpAndSettle would wait on forever; bounded frames settle layout.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
  } else {
    await tester.pumpAndSettle();
  }
}

ScrollableState _verticalScrollableOf(WidgetTester tester) =>
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
  final ScrollableState scrollable = _verticalScrollableOf(tester);
  for (var i = 0; i < 4; i++) {
    scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
    await tester.pump();
  }
}

/// Measured geometry of the POPULATED body at [inset], after scrolling to
/// the very end. The last meaningful content is the LAST notification
/// tile itself (its own text is checked for reachability separately).
Future<Map<String, double>> _measurePopulated(
  WidgetTester tester, {
  required double inset,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  expect(find.byType(AppBar), findsOneWidget);
  expect(find.byType(ListView), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect scrollBox = tester.getRect(find.byType(ListView));
  final Rect appBar = tester.getRect(find.byType(AppBar));
  final Rect title = tester.getRect(find.text('Notifications'));
  // The body's first element: the horizontal filter rail.
  final Rect rail = tester.getRect(find.byType(SingleChildScrollView));

  await _scrollToEnd(tester);

  final Finder lastTile = find.byType(NotificationDismissibleItem);
  expect(lastTile, findsWidgets, reason: 'the list must be populated');
  final Rect last = tester.getRect(lastTile.last);
  final Rect lastText = tester.getRect(find.text('Body n10'));

  final ScrollableState scrollable = _verticalScrollableOf(tester);
  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surfaceBox.bottom - inset;
  final double systemGap = regionStart - last.bottom;
  final double scrollGap = scrollBox.bottom - last.bottom;
  final double reachable =
      (lastText.bottom <= scrollBox.bottom + 0.01 &&
          lastText.top >= scrollBox.top - 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM branch=populated inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastTileBottom=${last.bottom.toStringAsFixed(2)} '
    'lastTextBottom=${lastText.bottom.toStringAsFixed(2)} '
    'railTop=${rail.top.toStringAsFixed(2)} '
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
    'lastTileBottom': last.bottom,
    'lastTextBottom': lastText.bottom,
    'railTop': rail.top,
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
        'the list viewport bottom must follow the live system inset '
        '($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset (measured ${m['scrollBottom']} vs surface '
        '${m['surfaceBottom']})',
  );

  expect(
    m['systemGap'],
    closeTo(_designTail, 0.01),
    reason:
        'at scroll end the gap from the last notification tile to the '
        'system region must be exactly the design tail (list vertical '
        'p12 = $_designTail) at inset $inset: negative means the tile '
        'entered the system region (${m['systemGap']}), smaller means '
        'the live inset is missing',
  );

  expect(
    m['scrollGap'],
    closeTo(_designTail, 0.01),
    reason:
        'the design tail between the last tile and the scroll end must '
        'stay constant (${m['scrollGap']})',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason: 'the list must actually scroll for the end contract to hold',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last meaningful notification text (${m['lastTextBottom']}) '
        'must be fully visible inside the viewport at scroll end at '
        'inset $inset',
  );

  // — top authority: Scaffold.appBar owns the status-bar region; the
  // body starts exactly at the AppBar bottom —
  expect(
    m['railTop'],
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the body (first element: the filter rail) must start exactly at '
        'the AppBar bottom — a phantom top reservation would push it '
        'down (railTop ${m['railTop']} vs appBarBottom '
        '${m['appBarBottom']})',
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

/// Measured geometry of a centered FILL branch (loading / error / empty)
/// at [inset]: its content must stay clear of the system region and of
/// the AppBar.
Future<Map<String, double>> _measureFill(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  expect(find.byType(AppBar), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect appBar = tester.getRect(find.byType(AppBar));

  late final Finder firstFinder;
  late final Finder lastFinder;
  switch (branch) {
    case _Branch.loading:
      firstFinder = find.byType(LoadingIndicator);
      lastFinder = find.byType(LoadingIndicator);
    case _Branch.error:
      firstFinder = find.byIcon(Icons.error_outline);
      lastFinder = find.widgetWithText(ElevatedButton, 'Coba Lagi');
    case _Branch.empty:
      firstFinder = find.byType(EmptyState);
      lastFinder = find.text(
        'Anda akan menerima notifikasi untuk aktivitas penting di sini',
      );
    case _Branch.populated:
      throw StateError('the populated branch has its own measurement');
  }

  expect(firstFinder, findsOneWidget, reason: '${branch.name} must render');
  final Rect first = tester.getRect(firstFinder);
  final Rect last = tester.getRect(lastFinder);
  final double regionStart = surfaceBox.bottom - inset;

  // ignore: avoid_print
  print(
    'GEOM branch=${branch.name} inset=$inset '
    'firstTop=${first.top.toStringAsFixed(2)} '
    'lastBottom=${last.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=${(regionStart - last.bottom).toStringAsFixed(2)} '
    'appBarBottom=${appBar.bottom.toStringAsFixed(2)}',
  );

  return <String, double>{
    'firstTop': first.top,
    'lastBottom': last.bottom,
    'regionStart': regionStart,
    'appBarBottom': appBar.bottom,
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
        'the ${branch.name} content (${m['lastBottom']}) must stay '
        'outside the system region (start ${m['regionStart']}) at inset '
        '$inset',
  );

  expect(
    m['firstTop'],
    greaterThanOrEqualTo(m['appBarBottom']! - 0.01),
    reason:
        'the ${branch.name} content (${m['firstTop']}) must stay below '
        'the AppBar bottom (${m['appBarBottom']}) — never under the '
        'status bar',
  );
}

void main() {
  group('SAFE-AREA-36 — populated list: live inset geometry', () {
    testWidgets('inset 0 — design tail only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0, branch: _Branch.populated);
      await _expectPopulatedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last tile clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24, branch: _Branch.populated);
      await _expectPopulatedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last tile clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34, branch: _Branch.populated);
      await _expectPopulatedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last tile clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48, branch: _Branch.populated);
      await _expectPopulatedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-36 — fill branches: live inset geometry', () {
    for (final inset in const <double>[0, 24, 34, 48]) {
      testWidgets('loading at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.loading);
        await _expectFillGeometry(
          tester,
          inset: inset,
          branch: _Branch.loading,
        );
      });

      testWidgets('initial error at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.error);
        await _expectFillGeometry(tester, inset: inset, branch: _Branch.error);
      });

      testWidgets('empty at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.empty);
        await _expectFillGeometry(tester, inset: inset, branch: _Branch.empty);
      });
    }
  });

  group('SAFE-AREA-36 — authority proof', () {
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
        final Map<String, double> at48 = await _measurePopulated(
          tester,
          inset: 48,
        );

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
      await _pump(tester, inset: 34, branch: _Branch.populated);

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

    testWidgets('the production sources hold no second inset authority', (
      tester,
    ) async {
      await _pump(tester, inset: 34, branch: _Branch.populated);

      final String screenSrc = File(
        'lib/domains/system/notification/presentation/screens/'
        'notification_list_screen.dart',
      ).readAsStringSync();
      final String contentSrc = File(
        'lib/domains/system/notification/presentation/widgets/'
        'notification_list_content.dart',
      ).readAsStringSync();

      expect(
        'SafeArea('.allMatches(screenSrc).length,
        1,
        reason:
            'the screen must own exactly one SafeArea authority — no '
            'duplicate ownership, none missing',
      );
      expect(
        'SafeArea('.allMatches(contentSrc).length,
        0,
        reason:
            'the list widget must not add a second bottom-inset '
            'authority — the body owns it',
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
          token.allMatches(screenSrc).length,
          0,
          reason:
              'the screen must not carry a second inset authority or a '
              'fixed clearance ($token)',
        );
        expect(
          token.allMatches(contentSrc).length,
          0,
          reason:
              'the list widget must not carry a second inset authority '
              'or a fixed clearance ($token)',
        );
      }
    });
  });
}
