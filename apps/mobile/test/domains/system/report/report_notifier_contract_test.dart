import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/data/data.dart'
    show ReportRepositoryException;
import 'package:labuda/domains/system/report/domain/entities/report.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart'
    as providers;

/// Fake repository with controllable completion via Completers.
class FakeReportRepository implements ReportRepository {
  PagedReports? getReportsByUserResult;
  Object? getReportsByUserError;
  Report? createReportResult;

  int getReportsByUserCallCount = 0;
  String? lastStatusFilter;
  int lastPage = -1;
  int lastLimit = -1;

  /// When non-null, getReportsByUser waits on this completer.
  /// Enables deterministic ordering of async completions.
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
    if (createReportResult != null) return createReportResult!;
    throw ReportRepositoryException('not implemented');
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

PagedReports _p({
  required List<Report> reports,
  int page = 1,
  int limit = 20,
  int count = 0,
}) => PagedReports(
  reports: reports,
  page: page,
  limit: limit,
  count: count == 0 ? reports.length : count,
);

Report _r(String id, {ReportStatus status = ReportStatus.pending}) => Report(
  id: id,
  reporterId: 'user-1',
  targetId: 'target-$id',
  targetType: ReportTargetType.content,
  reason: ReportReasonType.spam,
  status: status,
  createdAt: DateTime(2026, 7, 31),
);

Future<ProviderContainer> _setup({
  required FakeReportRepository repo,
  String? currentUserId,
}) async {
  final overrides = [
    providers.reportRepositoryProvider.overrideWithValue(repo),
    providers.reportCurrentUserIdProvider.overrideWithValue(currentUserId),
  ];
  final container = ProviderContainer(overrides: overrides);
  addTearDown(container.dispose);
  container.read(providers.reportListNotifierProvider);
  await Future.delayed(Duration.zero);
  return container;
}

void main() {
  // ===========================================================================
  // 1. Initial load
  // ===========================================================================
  test('1. initial load success populates reports', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('r1'), _r('r2')], count: 2);

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.error, isNull);
    expect(state.reports, hasLength(2));
    expect(state.count, 2);
    expect(state.hasMore, isFalse);
  });

  test('2. initial genuine empty', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [], count: 0);

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.reports, isEmpty);
    expect(state.count, 0);
    expect(state.hasMore, isFalse);
  });

  test('3. null user ID → auth error, not empty', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [], count: 0);

    final container = await _setup(repo: repo, currentUserId: null);
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.error, isNotNull);
    expect(state.error, contains('login'));
    expect(state.reports, isEmpty);
    expect(repo.getReportsByUserCallCount, 0);
  });

  test('4. initial datasource failure sets error', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserError = Exception('Network failure');

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.error, isNotNull);
    expect(state.reports, isEmpty);
  });

  test('4b. backend unavailable stays distinct from login', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserError = const ReportRepositoryException(
      'Backend sedang tidak tersedia. Coba lagi nanti.',
      type: ReportFailureType.backendUnavailable,
    );

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.error, 'Backend sedang tidak tersedia. Coba lagi nanti.');
    expect(state.error, isNot(contains('login')));
  });

  test('4c. timeout stays distinct from backend unavailable', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserError = const ReportRepositoryException(
      'Permintaan ke server terlalu lama. Coba lagi.',
      type: ReportFailureType.timeout,
    );

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.error, 'Permintaan ke server terlalu lama. Coba lagi.');
    expect(state.error, isNot(contains('login')));
  });

  test('4d. malformed response stays distinct from network/login', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserError = const ReportRepositoryException(
      'Respons server tidak valid. Coba lagi.',
      type: ReportFailureType.malformedResponse,
    );

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    final state = container.read(providers.reportListNotifierProvider);

    expect(state.isLoading, isFalse);
    expect(state.error, 'Respons server tidak valid. Coba lagi.');
    expect(state.error, isNot(contains('login')));
  });

  // ===========================================================================
  // 5. Refresh
  // ===========================================================================
  test('5. refresh success replaces page 1', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('initial')], count: 1);

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    expect(
      container.read(providers.reportListNotifierProvider).reports,
      hasLength(1),
    );

    repo.getReportsByUserResult = _p(
      reports: [_r('a'), _r('b'), _r('c')],
      count: 3,
    );

    await container
        .read(providers.reportListNotifierProvider.notifier)
        .refresh();
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    expect(state.reports, hasLength(3));
    expect(state.isRefreshing, isFalse);
    expect(state.refreshError, isNull);
  });

  test(
    '6. refresh failure retains last-good reports and publishes error',
    () async {
      final repo = FakeReportRepository();
      repo.getReportsByUserResult = _p(reports: [_r('r1')], count: 1);

      final container = await _setup(repo: repo, currentUserId: 'user-1');
      expect(
        container.read(providers.reportListNotifierProvider).reports,
        hasLength(1),
      );

      repo.getReportsByUserError = Exception('Timeout');

      await container
          .read(providers.reportListNotifierProvider.notifier)
          .refresh();
      await Future.delayed(Duration.zero);

      final state = container.read(providers.reportListNotifierProvider);
      expect(state.reports, hasLength(1), reason: 'data retained');
      expect(state.refreshError, isNotNull, reason: 'error surfaced');
      expect(state.isRefreshing, isFalse);
    },
  );

  // ===========================================================================
  // 7. Load next page with ID dedupe
  // ===========================================================================
  test('7. load next page appends items', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(
      reports: [_r('r1'), _r('r2')],
      limit: 2,
      count: 4,
    );

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    expect(
      container.read(providers.reportListNotifierProvider).hasMore,
      isTrue,
    );

    repo.getReportsByUserResult = _p(
      reports: [_r('r3'), _r('r4')],
      page: 2,
      limit: 2,
      count: 4,
    );

    await container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    expect(state.reports, hasLength(4));
    expect(state.currentPage, 2);
    expect(state.hasMore, isFalse);
    expect(state.isLoadingMore, isFalse);
  });

  test('8. ID deduplication — overlapping page items skipped', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(
      reports: [_r('A'), _r('B'), _r('C')],
      limit: 3,
      count: 5,
    );

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    // Page 2 has C (duplicate) + D + E
    repo.getReportsByUserResult = _p(
      reports: [_r('C'), _r('D'), _r('E')],
      page: 2,
      limit: 3,
      count: 5,
    );

    await container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    final ids = state.reports.map((r) => r.id).toList();
    expect(ids, [
      'A',
      'B',
      'C',
      'D',
      'E',
    ], reason: 'C should not appear twice; dedupe keeps first occurrence');
    expect(state.reports, hasLength(5));
  });

  test('9. next-page failure retains existing data', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('r1')], limit: 1, count: 3);

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    repo.getReportsByUserError = Exception('Page 2 failed');

    await container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    expect(state.reports, hasLength(1), reason: 'data retained');
    expect(state.loadMoreError, isNotNull);
    expect(state.isLoadingMore, isFalse);
  });

  test('10. hasMore=false prevents additional request', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('r1')], limit: 1, count: 1);

    final container = await _setup(repo: repo, currentUserId: 'user-1');
    expect(
      container.read(providers.reportListNotifierProvider).hasMore,
      isFalse,
    );

    final callCountBefore = repo.getReportsByUserCallCount;
    await container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();
    await Future.delayed(Duration.zero);

    expect(repo.getReportsByUserCallCount, callCountBefore);
  });

  test('11. isLoadingMore prevents duplicate concurrent load-more', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('r1')], limit: 1, count: 3);

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    // Block the first load-more
    final delay = Completer<void>();
    repo.delayCompleter = delay;

    final future = container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();
    await Future.delayed(Duration.zero); // let load-more start

    // Second call while first is in-flight
    await container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();

    delay.complete();
    await future;
    await Future.delayed(Duration.zero);

    // Only 2 total repo calls: 1 initial load + 1 actual load-more
    expect(
      repo.getReportsByUserCallCount,
      2,
      reason:
          'only 1 initial + 1 load-more; second was blocked by isLoadingMore',
    );
  });

  // ===========================================================================
  // 12. Status filter
  // ===========================================================================
  test('12. changing status resets to page 1', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('r1')], count: 1);

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    repo.getReportsByUserResult = _p(reports: [], count: 0);

    container
        .read(providers.reportListNotifierProvider.notifier)
        .setStatusFilter('enforced');
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    expect(state.selectedStatus, 'enforced');
    expect(state.currentPage, 1);
    expect(repo.lastStatusFilter, 'enforced');
    expect(repo.lastPage, 1);
  });

  // ===========================================================================
  // 13-16. Stale response protection
  // ===========================================================================
  test('13. old filter response ignored after filter change', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('initial')], count: 1);

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    // Start filter A (slow)
    final delayA = Completer<void>();
    repo.delayCompleter = delayA;

    final futureA = container
        .read(providers.reportListNotifierProvider.notifier)
        .loadReports(status: 'pending');
    await Future.delayed(Duration.zero);

    // Start filter B (fast) — this resets generation and filter
    repo.delayCompleter = null;
    repo.getReportsByUserResult = _p(reports: [_r('b1'), _r('b2')], count: 2);

    await container
        .read(providers.reportListNotifierProvider.notifier)
        .setStatusFilter('enforced');
    await Future.delayed(Duration.zero);

    // Now complete slow filter A
    repo.getReportsByUserResult = _p(reports: [_r('stale')], count: 1);
    delayA.complete();
    await futureA;
    await Future.delayed(Duration.zero);

    // Filter B result should survive — not overwritten by stale A
    final state = container.read(providers.reportListNotifierProvider);
    expect(
      state.selectedStatus,
      'enforced',
      reason:
          'active filter should be enforced, not overwritten by stale pending',
    );
    expect(
      state.reports,
      hasLength(2),
      reason: 'should still be filter B items',
    );
    expect(state.reports.first.id, 'b1');
  });

  test('14. old refresh response ignored after filter change', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(reports: [_r('r1')], count: 1);

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    // Start slow refresh
    final delayRefresh = Completer<void>();
    repo.delayCompleter = delayRefresh;

    final refreshFuture = container
        .read(providers.reportListNotifierProvider.notifier)
        .refresh();
    await Future.delayed(Duration.zero);

    // Change filter while refresh is in-flight
    repo.delayCompleter = null;
    repo.getReportsByUserResult = _p(reports: [_r('filtered')], count: 1);

    container
        .read(providers.reportListNotifierProvider.notifier)
        .setStatusFilter('enforced');
    await Future.delayed(Duration.zero);

    // Complete stale refresh
    delayRefresh.complete();
    await refreshFuture;
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    expect(
      state.selectedStatus,
      'enforced',
      reason: 'filter from setStatusFilter should survive stale refresh',
    );
    expect(state.reports.first.id, 'filtered');
  });

  test('15. stale load-more ignored after filter change', () async {
    final repo = FakeReportRepository();
    repo.getReportsByUserResult = _p(
      reports: [_r('p1a'), _r('p1b')],
      limit: 2,
      count: 5,
    );

    final container = await _setup(repo: repo, currentUserId: 'user-1');

    // Start slow load-more
    final delayMore = Completer<void>();
    repo.delayCompleter = delayMore;

    final loadMoreFuture = container
        .read(providers.reportListNotifierProvider.notifier)
        .loadNextPage();
    await Future.delayed(Duration.zero);

    // Change filter before load-more completes
    repo.delayCompleter = null;
    repo.getReportsByUserResult = _p(reports: [_r('new-filter')], count: 1);

    container
        .read(providers.reportListNotifierProvider.notifier)
        .setStatusFilter('approved');
    await Future.delayed(Duration.zero);

    // Complete stale load-more
    delayMore.complete();
    await loadMoreFuture;
    await Future.delayed(Duration.zero);

    final state = container.read(providers.reportListNotifierProvider);
    expect(
      state.selectedStatus,
      'approved',
      reason: 'filter should be approved from setStatusFilter',
    );
    // Should have filter results, not stale page 2 appended to old data
    expect(
      state.reports.length,
      1,
      reason: 'stale load-more should not append; filter reset replaced data',
    );
    expect(state.reports.first.id, 'new-filter');
  });

  test(
    '16. in-flight request can complete after dispose without publishing',
    () async {
      final repo = FakeReportRepository();
      repo.getReportsByUserResult = _p(reports: [_r('r1')], count: 1);
      repo.delayCompleter = Completer<void>();

      final overrides = [
        providers.reportRepositoryProvider.overrideWithValue(repo),
        providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
      ];
      final container = ProviderContainer(overrides: overrides);

      container.read(providers.reportListNotifierProvider);
      await Future.delayed(Duration.zero);
      expect(repo.getReportsByUserCallCount, 1);

      container.dispose();
      repo.delayCompleter!.complete();
      await Future.delayed(Duration.zero);
    },
  );
}
