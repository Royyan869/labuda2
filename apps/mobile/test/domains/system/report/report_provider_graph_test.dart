import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/data/providers.dart'
    as data_providers;
import 'package:labuda/domains/system/report/data/dto/dto.dart';
import 'package:labuda/domains/system/report/data/remote/report_api_datasource.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart'
    as providers;
import 'package:labuda/domains/system/report/presentation/screens/my_reports_screen.dart';

class _FakeReportApiDatasource implements ReportApiDatasource {
  @override
  Future<PagedModerationCases> getMyCases({
    String? status,
    int page = 1,
    int limit = 20,
  }) async {
    return PagedModerationCases.fromDataJson({
      'cases': const [],
      'page': page,
      'limit': limit,
      'count': 0,
    });
  }

  @override
  Future<ModerationCaseDto> createCase(CreateCaseRequestDto request) =>
      throw UnimplementedError();

  @override
  Future<ModerationCaseDto> getCase(String caseId) =>
      throw UnimplementedError();

  @override
  Future<AppealDto> createAppeal(CreateAppealRequestDto request) =>
      throw UnimplementedError();

  @override
  Future<AppealDto> getAppeal(String appealId) => throw UnimplementedError();

  @override
  Future<List<AppealDto>> getMyAppeals({String? status, int page = 1}) =>
      throw UnimplementedError();

  @override
  Future<UserWarningDto> getWarning(String warningId) =>
      throw UnimplementedError();

  @override
  Future<List<UserWarningDto>> getUserWarnings(
    String userId, {
    String? status,
    int page = 1,
  }) => throw UnimplementedError();

  @override
  Future<List<UserWarningDto>> getActiveWarnings(String userId) =>
      throw UnimplementedError();
}

void main() {
  test('report S3 provider delegates to canonical core S3 service', () {
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(ApiClient.testing()),
      ],
    );
    addTearDown(container.dispose);

    final coreS3 = container.read(s3ServiceProvider);
    final reportS3 = container.read(providers.reportS3ServiceProvider);

    expect(identical(reportS3, coreS3), isTrue);
    expect(() => container.read(providers.reportRepositoryProvider), returnsNormally);
  });

  testWidgets('My Reports builds without ProviderException on production-like graph', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(ApiClient.testing()),
        data_providers.reportApiDatasourceProvider.overrideWithValue(
          _FakeReportApiDatasource(),
        ),
        providers.reportCurrentUserIdProvider.overrideWithValue('user-1'),
      ],
    );
    addTearDown(container.dispose);
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(home: MyReportsScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('Belum Ada Laporan'), findsOneWidget);
  });
}
