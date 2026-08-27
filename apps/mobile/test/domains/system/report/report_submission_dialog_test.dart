import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/data/data.dart'
    show ReportRepositoryException;
import 'package:labuda/domains/system/report/domain/entities/report.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

class _FakeReportRepository implements ReportRepository {
  int createReportCallCount = 0;
  Completer<void>? createDelay;
  Object? createError;

  @override
  Future<Report> createReport({
    required String reporterId,
    required CreateReportRequest request,
  }) async {
    createReportCallCount++;
    if (createDelay != null) {
      await createDelay!.future;
    }
    if (createError != null) {
      throw createError!;
    }
    return Report(
      id: 'case-1',
      reporterId: reporterId,
      targetId: request.targetId,
      targetType: request.targetType,
      reason: request.reason,
      status: ReportStatus.pending,
      createdAt: DateTime(2026, 7, 31),
    );
  }

  @override
  Future<PagedReports> getReportsByUser({
    required String userId,
    String? status,
    int page = 1,
    int limit = 20,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Report?> getReportById(String reportId) {
    throw UnimplementedError();
  }

  @override
  Future<String> uploadEvidence({
    required String reporterId,
    required String filePath,
  }) {
    throw UnimplementedError();
  }
}

Future<void> _openDialog(
  WidgetTester tester, {
  required _FakeReportRepository repo,
}) async {
  final container = ProviderContainer(
    overrides: [
      reportRepositoryProvider.overrideWithValue(repo),
      reportCurrentUserIdProvider.overrideWithValue('user-1'),
      authControllerProvider.overrideWith(_FakeAuthController.new),
    ],
  );
  addTearDown(container.dispose);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.binding.setSurfaceSize(const Size(360, 560));

  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: Scaffold(
          body: Builder(
            builder: (context) {
              return Center(
                child: ElevatedButton(
                  onPressed: () {
                    showModalBottomSheet<void>(
                      context: context,
                      isScrollControlled: true,
                      backgroundColor: Colors.transparent,
                      builder: (_) {
                        return const ReportSubmissionDialog(
                          targetId: 'target-1',
                          targetType: ReportTargetType.content,
                          targetTitle: 'Target 1',
                        );
                      },
                    );
                  },
                  child: const Text('open'),
                ),
              );
            },
          ),
        ),
      ),
    ),
  );

  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('small viewport does not overflow and submit is reachable', (
    tester,
  ) async {
    final repo = _FakeReportRepository()
      ..createDelay = Completer<void>()
      ..createError = const ReportRepositoryException(
        'Backend sedang tidak tersedia. Coba lagi nanti.',
      );
    await _openDialog(tester, repo: repo);

    expect(tester.takeException(), isNull);
    expect(find.text('Kirim Laporan'), findsOneWidget);

    await tester.ensureVisible(find.text('Kirim Laporan'));
    await tester.pumpAndSettle();

    expect(
      tester.getRect(find.text('Kirim Laporan')).bottom,
      lessThanOrEqualTo(560),
    );
  });

  testWidgets('selecting a reason updates selected state', (tester) async {
    final repo = _FakeReportRepository();
    await _openDialog(tester, repo: repo);

    await tester.tap(find.text('Scam'));
    await tester.pump();

    final scamText = tester.widget<Text>(find.text('Scam'));
    expect(scamText.style?.color, AppColors.primaryBlue);
  });

  testWidgets('submit button can be tapped and does not double-submit', (
    tester,
  ) async {
    final repo = _FakeReportRepository()
      ..createDelay = Completer<void>()
      ..createError = const ReportRepositoryException(
        'Backend sedang tidak tersedia. Coba lagi nanti.',
      );
    await _openDialog(tester, repo: repo);

    await tester.tap(find.text('Scam'));
    await tester.pump();

    await tester.ensureVisible(find.text('Kirim Laporan'));
    await tester.pump();

    await tester.tap(find.text('Kirim Laporan'));
    await tester.pump();

    expect(repo.createReportCallCount, 1);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    repo.createDelay!.complete();
    await tester.pumpAndSettle();
  });
}
