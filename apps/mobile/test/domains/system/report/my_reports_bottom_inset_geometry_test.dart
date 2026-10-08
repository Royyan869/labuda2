// SAFE-AREA-27 — MY REPORTS SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the single canonical bottom system-window authority on
// MyReportsScreen:
//
//   * system bottom inset → the body `SafeArea` (LIVE: the scroll viewport
//     bottom tracks 0 / 24 / 34 / 48 as the window metrics change; a fixed
//     clearance could not);
//   * design spacing      → the list `SliverPadding(AppMetrics.p16)` — a
//     constant measured BELOW the live inset, never a stand-in for it;
//   * keyboard            → N/A: the screen owns no text input, so the
//     Scaffold `resizeToAvoidBottomInset` default is never exercised here.
//
// Geometry is measured on the REAL screen (loaded state, scripted
// repository) with injected window metrics — not on a stand-in scaffold.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'package:labuda/domains/system/report/presentation/screens/my_reports_screen.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Static repository: the screen runs the REAL ReportListNotifier against
/// this list, so the loaded state is the production render path.
class _StaticRepo implements ReportRepository {
  _StaticRepo(this._reports);

  final List<Report> _reports;

  @override
  Future<List<Report>> getReportsByUser({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async => _reports;

  @override
  Future<Report> createReport({
    required String reporterId,
    required CreateReportRequest request,
  }) => throw UnimplementedError();

  @override
  Future<Report?> getReportById(String reportId) async => null;

  @override
  Future<bool> hasUserReported({
    required String userId,
    required String targetId,
    required ReportTargetType targetType,
  }) async => false;
}

Report _report(String id) => Report(
  id: id,
  reporterId: 'user-1',
  subjectId: 's-$id',
  subjectType: ReportTargetType.content,
  reason: ReportReasonType.scamOrFraud,
  createdAt: DateTime.utc(2026, 1, 1),
  targetProjection: ReportTargetProjection(
    subjectType: 'content',
    subjectId: 's-$id',
    title: 'Target $id',
  ),
);

/// Enough cards that the collection always exceeds one viewport, so the
/// scroll-end contract is exercised for real.
List<Report> _reports(int count) => [
  for (var i = 1; i <= count; i++) _report('r$i'),
];

/// Injects window metrics on the TEST VIEW (same idiom as SAFE-AREA-10/17)
/// so every inset below is the REAL, LIVE one.
void _setInsets(WidgetTester tester, {required double bottom}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(top: 24 * dpr, bottom: bottom * dpr);
  tester.view.viewPadding = FakeViewPadding(
    top: 24 * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewInsets = const FakeViewPadding();
}

Future<void> _pump(WidgetTester tester, {required double inset}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        reportCurrentUserIdProvider.overrideWithValue('user-1'),
        reportRepositoryProvider.overrideWithValue(_StaticRepo(_reports(10))),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const MyReportsScreen(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

ScrollableState _scrollableOf(WidgetTester tester) =>
    tester.state<ScrollableState>(
      find
          .descendant(
            of: find.byType(CustomScrollView),
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

/// Measured geometry row at [inset], after scrolling to the very end.
/// Printed so every run documents the raw numbers behind the contract.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surface = tester.getRect(find.byType(Scaffold));
  final Finder scrollFinder = find.byType(CustomScrollView);
  final Rect scrollRect = tester.getRect(scrollFinder);

  final ScrollableState scrollable = _scrollableOf(tester);
  await _scrollToEnd(tester);

  final Rect lastCard = tester.getRect(find.byType(ReportCard).last);
  final double viewport = scrollRect.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surface.bottom - inset;
  final double gap = regionStart - lastCard.bottom;
  final double reachable =
      (lastCard.top >= scrollRect.top - 0.01 &&
          lastCard.bottom <= scrollRect.bottom + 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastCardBottom=${lastCard.bottom.toStringAsFixed(2)} '
    'scrollBottom=${scrollRect.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surface.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap reachable=$reachable',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'lastCardBottom': lastCard.bottom,
    'scrollBottom': scrollRect.bottom,
    'surfaceBottom': surface.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'reachable': reachable,
  };
}

/// Loaded-state contract at [inset]:
/// 1. the scroll viewport boundary follows the LIVE inset (SafeArea);
/// 2. the scroll path is real (maxScrollExtent > 0);
/// 3. at scroll-end the last card sits exactly the design `p16` ABOVE the
///    system region — never inside it, and the 16 px never grows into a
///    fixed inset stand-in;
/// 4. the last card is fully reachable inside the viewport.
Future<void> _expectLoadedGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measure(tester, inset: inset);

  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the scroll viewport bottom must follow the live system inset '
        '($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason: 'the collection must actually scroll for the end contract to hold',
  );

  expect(
    m['gap'],
    closeTo(16, 0.01),
    reason:
        'at scroll-end the gap to the system region must be exactly the '
        'design SliverPadding (AppMetrics.p16 = 16) at inset $inset: '
        'negative would mean overlap (${m['gap']}), larger would mean a '
        'fixed clearance standing in for the live inset',
  );

  expect(
    m['lastCardBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason:
        'the last card (${m['lastCardBottom']}) must stay outside the system '
        'region (start ${m['regionStart']}) at inset $inset',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last card must be fully visible inside the viewport at '
        'scroll-end at inset $inset',
  );
}

void main() {
  group('SAFE-AREA-27 — loaded list: live inset geometry', () {
    testWidgets('inset 0 — design spacing only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0);
      expect(find.text('Target r1'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24);
      expect(find.text('Target r1'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34);
      expect(find.text('Target r1'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48);
      expect(find.text('Target r1'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-27 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space)',
      (tester) async {
        await _pump(tester, inset: 0);
        final Map<String, double> at0 = await _measure(tester, inset: 0);

        // System bar appears (48 px): only a live authority moves the
        // scrollable's bottom boundary.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measure(tester, inset: 48);

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason:
              'the scroll viewport bottom must follow the system inset — a '
              'fixed clearance would not move',
        );

        // The viewport shrinks exactly by the inset; the content extent is
        // invariant: no child scrollable absorbs the inset a second time
        // (that would grow the content extent by the same 48 px).
        expect(
          at48['viewport']!,
          closeTo(at0['viewport']! - 48, 0.01),
          reason: 'the viewport must shrink exactly by the live inset',
        );
        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason:
              'the content extent must stay constant across insets — growth '
              'would prove a second, duplicate inset absorption',
        );

        // Exactly ONE canonical SafeArea authority owns the body scroll
        // geometry: count SafeArea ANCESTORS of the scrollable. (The
        // MaterialAppBar's internal SafeArea is `bottom: false` in a
        // sibling slot — it can never own the body's bottom inset.)
        int safeAreaAncestors = 0;
        tester.element(find.byType(CustomScrollView)).visitAncestorElements((
          element,
        ) {
          if (element.widget is SafeArea) safeAreaAncestors++;
          return true;
        });
        expect(
          safeAreaAncestors,
          1,
          reason:
              'exactly one SafeArea must sit between the scrollable and the '
              'root — no duplicate ownership, none missing',
        );
        final String src = File(
          'lib/domains/system/report/presentation/screens/my_reports_screen.dart',
        ).readAsStringSync();
        expect(
          'SafeArea('.allMatches(src).length,
          1,
          reason:
              'the screen must own exactly one SafeArea authority — no '
              'duplicate ownership',
        );
      },
    );

    testWidgets('empty state shares the same viewport authority', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setInsets(tester, bottom: 34);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            reportCurrentUserIdProvider.overrideWithValue('user-1'),
            reportRepositoryProvider.overrideWithValue(_StaticRepo(const [])),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('id'),
            home: const MyReportsScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Belum ada laporan'), findsOneWidget);

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect scrollRect = tester.getRect(find.byType(CustomScrollView));
      expect(
        scrollRect.bottom,
        closeTo(surface.bottom - 34, 0.01),
        reason:
            'the empty state (SliverFillRemaining) must end at the same '
            'SafeArea-owned boundary at inset 34',
      );
    });
  });
}
