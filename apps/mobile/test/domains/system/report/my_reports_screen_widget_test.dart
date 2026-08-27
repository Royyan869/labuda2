import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/data/data.dart'
    show ReportRepositoryException;
import 'package:labuda/domains/system/report/domain/entities/report.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/presentation/providers/report/report_state.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart'
    as providers;
import 'package:labuda/domains/system/report/presentation/screens/my_reports_screen.dart';

class FakeReportRepository implements ReportRepository {
  PagedReports? getReportsByUserResult;
  Object? getReportsByUserError;
  int getReportsByUserCallCount = 0;
  String? lastStatusFilter;
  int lastPage = -1;
  int lastLimit = -1;
  Completer<void>? delayCompleter;

  @override
  Future<PagedReports> getReportsByUser({
    required String userId,
    String? status,
    int page = 1,
    int limit = 20,
  }) async {
    getReportsByUserCallCount++;
    lastStatusFilter = status;
    lastPage = page;
    lastLimit = limit;

    if (delayCompleter != null) {
      await delayCompleter!.future;
    }

    if (getReportsByUserError != null) {
      throw getReportsByUserError!;
    }

    return getReportsByUserResult!;
  }

  @override
  Future<Report> createReport({
    required String reporterId,
    required CreateReportRequest request,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Report?> getReportById(String reportId) async {
    throw UnimplementedError();
  }

  @override
  Future<String> uploadEvidence({
    required String reporterId,
    required String filePath,
  }) async {
    throw UnimplementedError();
  }
}

Report _r(
  String id, {
  ReportTargetType type = ReportTargetType.content,
  ReportStatus status = ReportStatus.pending,
  String? description,
  DateTime? createdAt,
}) {
  return Report(
    id: id,
    reporterId: 'u',
    targetId: 't-$id',
    targetType: type,
    targetTitle: 'Target $id',
    reason: ReportReasonType.spam,
    description: description,
    status: status,
    createdAt: createdAt ?? DateTime.now(),
  );
}

PagedReports _page(
  List<Report> reports, {
  int page = 1,
  int limit = 20,
  int? count,
}) {
  return PagedReports(
    reports: reports,
    page: page,
    limit: limit,
    count: count ?? reports.length,
  );
}

Future<void> _pumpScreen(
  WidgetTester tester,
  ProviderContainer container,
) async {
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: MyReportsScreen()),
    ),
  );
  await tester.pump();
}

void main() {
  group('Status labels', () {
    test('pending -> Menunggu peninjauan', () {
      expect(ReportStatus.pending.displayName, 'Menunggu peninjauan');
    });

    test('approved -> Tidak melanggar', () {
      expect(ReportStatus.approved.displayName, 'Tidak melanggar');
    });

    test('rejected -> Laporan ditutup', () {
      expect(ReportStatus.rejected.displayName, 'Laporan ditutup');
    });

    test('enforced -> Tindakan telah diambil', () {
      expect(ReportStatus.enforced.displayName, 'Tindakan telah diambil');
    });
  });

  group('ReportStatus enum', () {
    test('has exactly 4 values', () {
      expect(ReportStatus.values.length, 4);
    });

    test('no underReview', () {
      for (final s in ReportStatus.values) {
        expect(s.name, isNot('underReview'));
      }
    });

    test('no resolved', () {
      for (final s in ReportStatus.values) {
        expect(s.name, isNot('resolved'));
      }
    });
  });

  group('ReportTargetType labels', () {
    test('auction -> Auction', () {
      expect(ReportTargetType.auction.displayName, 'Auction');
    });

    test('fixedPriceSale -> Fixed-Price Sale', () {
      expect(ReportTargetType.forSale.displayName, 'Fixed-Price Sale');
    });

    test('all 6 types have unique display names', () {
      final names = ReportTargetType.values.map((e) => e.displayName).toSet();
      expect(names.length, 6);
    });
  });

  group('ReportListState', () {
    test('defaults', () {
      const s = ReportListState();
      expect(s.isLoading, isFalse);
      expect(s.isRefreshing, isFalse);
      expect(s.isLoadingMore, isFalse);
      expect(s.reports, isEmpty);
      expect(s.count, 0);
      expect(s.hasMore, isFalse);
      expect(s.currentPage, 1);
      expect(s.limit, 20);
      expect(s.error, isNull);
      expect(s.selectedStatus, isNull);
    });

    test('loaded with data', () {
      final reports = [_r('r1'), _r('r2')];
      final s = ReportListState(reports: reports, count: 2, hasMore: false);
      expect(s.reports, hasLength(2));
      expect(s.count, 2);
      expect(s.hasMore, isFalse);
    });

    test('refreshError with existing data', () {
      final s = ReportListState(
        reports: [_r('r1')],
        refreshError: 'Timeout',
        count: 1,
      );
      expect(s.refreshError, 'Timeout');
      expect(s.reports, hasLength(1));
    });

    test('loadMoreError with existing data', () {
      final s = ReportListState(
        reports: [_r('r1')],
        loadMoreError: 'Failed',
        count: 25,
        hasMore: true,
      );
      expect(s.loadMoreError, 'Failed');
      expect(s.reports, hasLength(1));
    });

    test('selectedStatus filter', () {
      const s = ReportListState(selectedStatus: 'enforced');
      expect(s.selectedStatus, 'enforced');
    });

    test('copyWith preserves unset fields', () {
      const s = ReportListState(isLoading: true, count: 5);
      final updated = s.copyWith(isLoading: false);
      expect(updated.isLoading, isFalse);
      expect(updated.count, 5);
      expect(updated.reports, isEmpty);
    });
  });

  group('ReportCard widget', () {
    testWidgets('renders loaded report card with auction and status labels', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ReportCard(
              report: _r(
                'card-1',
                type: ReportTargetType.auction,
                status: ReportStatus.enforced,
                description: 'Auction description',
                createdAt: DateTime.now(),
              ),
            ),
          ),
        ),
      );

      expect(find.text('Auction'), findsOneWidget);
      expect(find.text(ReportStatus.enforced.displayName), findsOneWidget);
      expect(find.text(ReportReasonType.spam.displayName), findsOneWidget);
      expect(find.text('Auction description'), findsOneWidget);
      expect(find.textContaining('Hari ini'), findsOneWidget);
    });

    testWidgets('renders all four status labels', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Column(
              children: [
                ReportCard(report: _r('p', status: ReportStatus.pending)),
                ReportCard(report: _r('a', status: ReportStatus.approved)),
                ReportCard(report: _r('r', status: ReportStatus.rejected)),
                ReportCard(report: _r('e', status: ReportStatus.enforced)),
              ],
            ),
          ),
        ),
      );

      expect(find.text(ReportStatus.pending.displayName), findsOneWidget);
      expect(find.text(ReportStatus.approved.displayName), findsOneWidget);
      expect(find.text(ReportStatus.rejected.displayName), findsOneWidget);
      expect(find.text(ReportStatus.enforced.displayName), findsOneWidget);
      expect(find.byType(ReportCard), findsNWidgets(4));
    });
  });

  group('MyReportsScreen widget', () {
    testWidgets('initial loading shows CircularProgressIndicator', (
      tester,
    ) async {
      final repo = FakeReportRepository()
        ..getReportsByUserResult = _page([_r('seed')], count: 1)
        ..delayCompleter = Completer<void>();
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);

      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      repo.delayCompleter!.complete();
      await tester.pumpAndSettle();
    });

    testWidgets('retry recovers from initial error', (tester) async {
      final repo = FakeReportRepository()
        ..getReportsByUserError = Exception('Network failure');
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat laporan'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);

      repo.getReportsByUserError = null;
      repo.getReportsByUserResult = _page([_r('ok')], count: 1);
      await tester.tap(find.text('Coba Lagi'));
      await tester.pump();
      await tester.pumpAndSettle();

      expect(find.byType(ReportCard), findsOneWidget);
      expect(find.text('Gagal memuat laporan'), findsNothing);
    });

    testWidgets('backend unavailable renders distinct copy', (tester) async {
      final repo = FakeReportRepository()
        ..getReportsByUserError = const ReportRepositoryException(
          'Backend sedang tidak tersedia. Coba lagi nanti.',
          type: ReportFailureType.backendUnavailable,
        );
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat laporan'), findsOneWidget);
      expect(
        find.text('Backend sedang tidak tersedia. Coba lagi nanti.'),
        findsOneWidget,
      );
      expect(find.textContaining('login'), findsNothing);
    });

    testWidgets('filter invocation updates status and shows filtered empty', (
      tester,
    ) async {
      final repo = FakeReportRepository()
        ..getReportsByUserResult = _page([_r('one')], count: 1);
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();

      repo.getReportsByUserResult = _page([], count: 0);
      await tester.tap(find.byIcon(Icons.filter_list));
      await tester.pumpAndSettle();
      await tester.tap(find.text(ReportStatus.enforced.displayName).last);
      await tester.pumpAndSettle();

      expect(repo.lastStatusFilter, 'enforced');
      expect(find.text('Belum Ada Laporan'), findsOneWidget);
      expect(
        find.text('Tidak ada laporan dengan status "Tindakan telah diambil"'),
        findsOneWidget,
      );
    });

    testWidgets('scroll load more shows progress then appends reports', (
      tester,
    ) async {
      final initialReports = List.generate(12, (index) => _r('page1-$index'));
      final nextReports = List.generate(4, (index) => _r('page2-$index'));
      final repo = FakeReportRepository()
        ..getReportsByUserResult = _page(initialReports, count: 16);
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();
      expect(
        container.read(providers.reportListNotifierProvider).reports,
        hasLength(12),
      );

      repo.getReportsByUserResult = _page(nextReports, page: 2, count: 16);
      repo.delayCompleter = Completer<void>();

      await tester.drag(find.byType(ListView), const Offset(0, -2000));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(
        container.read(providers.reportListNotifierProvider).isLoadingMore,
        isTrue,
      );
      expect(repo.lastPage, 2);

      repo.delayCompleter!.complete();
      await tester.pumpAndSettle();

      final state = container.read(providers.reportListNotifierProvider);
      expect(state.reports, hasLength(16));
      expect(state.currentPage, 2);
      expect(state.isLoadingMore, isFalse);
    });

    testWidgets('load-more error shows retry affordance', (tester) async {
      final initialReports = List.generate(10, (index) => _r('page1-$index'));
      final repo = FakeReportRepository()
        ..getReportsByUserResult = _page(initialReports, count: 14);
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();

      repo.getReportsByUserError = Exception('Page 2 failed');
      await tester.drag(find.byType(ListView), const Offset(0, -2000));
      await tester.pump();
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat laporan berikutnya'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
      final state = container.read(providers.reportListNotifierProvider);
      expect(state.loadMoreError, isNotNull);
      expect(state.isLoadingMore, isFalse);
    });

    testWidgets('refresh pull shows progress indicator', (tester) async {
      final repo = FakeReportRepository()
        ..getReportsByUserResult = _page(
          List.generate(8, (index) => _r('r$index')),
          count: 8,
        );
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();

      repo.delayCompleter = Completer<void>();
      await tester.drag(find.byType(ListView), const Offset(0, 300));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 250));

      expect(find.byType(RefreshProgressIndicator), findsOneWidget);
      expect(
        container.read(providers.reportListNotifierProvider).isRefreshing,
        isTrue,
      );

      repo.delayCompleter!.complete();
      await tester.pumpAndSettle();
    });

    testWidgets('refresh error shows snackbar message', (tester) async {
      final repo = FakeReportRepository()
        ..getReportsByUserResult = _page(
          List.generate(5, (index) => _r('r$index')),
          count: 5,
        );
      final container = ProviderContainer(
        overrides: [
          providers.reportRepositoryProvider.overrideWithValue(repo),
          providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
        ],
      );
      addTearDown(container.dispose);

      await _pumpScreen(tester, container);
      await tester.pumpAndSettle();

      repo.getReportsByUserError = Exception('Refresh failed');
      await tester.drag(find.byType(ListView), const Offset(0, 300));
      await tester.pump();
      await tester.pumpAndSettle();

      expect(find.byType(SnackBar), findsOneWidget);
      expect(find.textContaining('Refresh failed'), findsOneWidget);
    });
  });
}
