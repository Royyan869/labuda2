import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/system/report/domain/entities/entities.dart';
import 'package:hishumi/domains/system/report/domain/repositories/report_repository.dart';
import 'package:hishumi/domains/system/report/presentation/providers/report_providers.dart';
import 'package:hishumi/domains/system/report/presentation/screens/my_reports_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

/// Scripted repository: the screen runs the REAL ReportListNotifier, so
/// every load (initial, retry, refresh, filter render) is observable per
/// call at the canonical producer boundary.
class _ScriptedReportRepository implements ReportRepository {
  _ScriptedReportRepository({required this.onFetch});

  Future<List<Report>> Function(int call) onFetch;

  int fetchCalls = 0;

  @override
  Future<List<Report>> getReportsByUser({
    required String userId,
    int page = 1,
    int limit = 20,
  }) {
    fetchCalls++;
    return onFetch(fetchCalls);
  }

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

Report _report(
  String id, {
  String title = 'Target',
  ReportCaseProjection? caseProjection,
  ReportDecisionProjection? decisionProjection,
}) => Report(
  id: id,
  reporterId: 'user-1',
  subjectId: 's-$id',
  subjectType: ReportTargetType.content,
  reason: ReportReasonType.scamOrFraud,
  createdAt: DateTime.utc(2026, 1, 1),
  caseProjection: caseProjection,
  decisionProjection: decisionProjection,
  targetProjection: ReportTargetProjection(
    subjectType: 'content',
    subjectId: 's-$id',
    title: '$title $id',
  ),
);

Report _underReview(String id) => _report(
  id,
  title: 'Target',
  caseProjection: ReportCaseProjection(
    id: 'case-$id',
    status: 'open',
    createdAt: DateTime.utc(2026, 1, 2),
  ),
);

void main() {
  Future<void> pumpScreen(
    WidgetTester tester,
    _ScriptedReportRepository repository, {
    bool settle = true,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          reportCurrentUserIdProvider.overrideWithValue('user-1'),
          reportRepositoryProvider.overrideWithValue(repository),
        ],
        child: MaterialApp(
          home: const MyReportsScreen(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('id'),
        ),
      ),
    );
    if (settle) await tester.pumpAndSettle();
  }

  _ScriptedReportRepository repoOf(List<Report> reports) =>
      _ScriptedReportRepository(onFetch: (_) => Future.value(reports));

  group('MyReportsScreen — initial load authority', () {
    testWidgets('initial request is triggered exactly once', (tester) async {
      final repository = repoOf([_report('r1')]);
      await pumpScreen(tester, repository);

      expect(repository.fetchCalls, 1);
      expect(find.text('Target r1'), findsOneWidget);
    });

    testWidgets('first request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = Completer<List<Report>>();
      final repository = _ScriptedReportRepository(onFetch: (_) => gate.future);
      await pumpScreen(tester, repository, settle: false);
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete([_report('r1')]);
      await tester.pumpAndSettle();
      expect(find.text('Target r1'), findsOneWidget);
    });

    testWidgets('non-empty reports render', (tester) async {
      await pumpScreen(tester, repoOf([_report('r1'), _underReview('r2')]));

      expect(find.text('Target r1'), findsOneWidget);
      expect(find.text('Target r2'), findsOneWidget);
    });

    testWidgets('successful zero-result shows EmptyState', (tester) async {
      await pumpScreen(tester, repoOf(const []));

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Belum ada laporan'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });

    testWidgets('status filter does not refetch the collection', (
      tester,
    ) async {
      final repository = repoOf([_report('r1'), _underReview('r2')]);
      await pumpScreen(tester, repository);
      expect(repository.fetchCalls, 1);

      await tester.tap(find.byType(PopupMenuButton<ReportDisplayState?>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Submitted').last);
      await tester.pumpAndSettle();

      expect(repository.fetchCalls, 1);
      expect(find.text('Target r1'), findsOneWidget);
      expect(find.text('Target r2'), findsNothing);
    });
  });

  group('MyReportsScreen — initial failure and retry', () {
    testWidgets('failure with no data shows PageErrorState, retry reloads', (
      tester,
    ) async {
      final repository = _ScriptedReportRepository(
        onFetch: (call) => call == 1
            ? Future<List<Report>>.error(Exception('boom-initial'))
            : Future.value([_report('r1')]),
      );
      await pumpScreen(tester, repository);

      // CANONICAL error surface: safe localized copy only — the raw
      // backend text must never reach the screen.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.textContaining('boom-initial'), findsNothing);
      expect(find.text('Failed to load reports'), findsNothing);
      expect(find.text('Retry'), findsNothing);
      expect(find.byType(EmptyState), findsNothing);
    });

    testWidgets('retry clears the stale error and shows loading, not frozen', (
      tester,
    ) async {
      final gate = Completer<List<Report>>();
      var first = true;
      final repository = _ScriptedReportRepository(
        onFetch: (call) {
          if (first) {
            first = false;
            return Future<List<Report>>.error(Exception('boom-initial'));
          }
          return gate.future;
        },
      );
      await pumpScreen(tester, repository);
      expect(find.byType(PageErrorState), findsOneWidget);

      // Retry executes the canonical initial load.
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pump();
      await tester.pump();

      // The stale error is gone and the retry is visibly in flight — the
      // UI must not sit frozen on the previous error.
      expect(repository.fetchCalls, 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.textContaining('boom-initial'), findsNothing);
      expect(find.byType(LoadingIndicator), findsOneWidget);

      gate.complete([_report('r1')]);
      await tester.pumpAndSettle();
      expect(find.text('Target r1'), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
    });
  });

  group('MyReportsScreen — refresh', () {
    testWidgets('existing reports stay visible with refresh indicator', (
      tester,
    ) async {
      final gate = Completer<List<Report>>();
      final repository = _ScriptedReportRepository(
        onFetch: (call) =>
            call == 1 ? Future.value([_report('r1')]) : gate.future,
      );
      await pumpScreen(tester, repository);
      expect(find.text('Target r1'), findsOneWidget);

      await tester.fling(
        find.byType(CustomScrollView),
        const Offset(0, 300),
        1000,
      );
      // Allow the RefreshIndicator to fire onRefresh and the reload to
      // start (bounded: the gate stays open, so never settle here).
      for (var i = 0; i < 50 && repository.fetchCalls < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repository.fetchCalls, 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('Target r1'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete([_report('r2')]);
      await tester.pumpAndSettle();

      expect(find.text('Target r2'), findsOneWidget);
      expect(find.text('Target r1'), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });

    testWidgets(
      'refresh failure keeps rows with inline banner, retry recovers',
      (tester) async {
        final repository = _ScriptedReportRepository(
          onFetch: (call) {
            if (call == 1) return Future.value([_report('r1')]);
            if (call == 2) {
              return Future<List<Report>>.error(Exception('boom-refresh'));
            }
            return Future.value([_report('r2')]);
          },
        );
        await pumpScreen(tester, repository);
        expect(find.text('Target r1'), findsOneWidget);

        await tester.fling(
          find.byType(CustomScrollView),
          const Offset(0, 300),
          1000,
        );
        await tester.pumpAndSettle();

        // Valid data is preserved; failure renders inline, never full-page.
        expect(find.text('Target r1'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsOneWidget,
        );
        expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
        expect(find.textContaining('boom-refresh'), findsNothing);

        // Retry executes refresh; success replaces stale data and clears banner.
        await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
        await tester.pumpAndSettle();

        expect(repository.fetchCalls, 3);
        expect(find.text('Target r2'), findsOneWidget);
        expect(find.text('Target r1'), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsNothing,
        );
      },
    );
  });

  group('MyReportsScreen — negative proof (static contract)', () {
    String screenSource() => File(
      'lib/domains/system/report/presentation/screens/my_reports_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    String notifierSource() => File(
      'lib/domains/system/report/presentation/providers/report/report_notifier.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = screenSource();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
    });

    test('no raw spinner or local error renderer remains', () {
      final src = screenSource();
      expect(src.contains('CircularProgressIndicator('), isFalse);
      expect(src.contains('_buildErrorView'), isFalse);
      expect(src.contains('Failed to load reports'), isFalse);
    });

    test('no raw technical error reaches the widget tree', () {
      final src = screenSource();
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
      expect(src.contains('state.error!'), isFalse);
    });

    test('one initial-load trigger: screen mount only', () {
      final screen = screenSource();
      final notifier = notifierSource();
      // The screen triggers exactly one load on mount.
      expect('Future.microtask'.allMatches(screen).length, 1);
      expect(screen.contains('_loadReports()'), isTrue);
      // The notifier no longer fetches in build().
      expect(notifier.contains('Future.microtask'), isFalse);
      expect('loadReports()'.allMatches(notifier).length, 2);
    });

    test('no obsolete refresh flag or duplicate authority in scope', () {
      final notifier = notifierSource();
      expect(notifier.contains('refresh = false'), isFalse);
      expect(notifier.contains('refresh: true'), isFalse);
      // No provider is *defined* in the screen (consuming the canonical
      // reportListNotifierProvider is required, defining one is not).
      expect(screenSource().contains('NotifierProvider('), isFalse);
      expect(screenSource().contains('NotifierProvider<'), isFalse);
    });
  });
}
