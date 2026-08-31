import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart' as api;
import 'package:labuda/domains/system/report/data/dto/dto.dart';
import 'package:labuda/domains/system/report/data/remote/report_api_datasource.dart';
import 'package:labuda/domains/system/report/data/repositories/report_repository_impl.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';

void main() {
  group('ReportRepositoryImpl.getReportsByUser', () {
    test('maps backend-unreachable DioException to network failure', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/reports/mine'),
            type: DioExceptionType.connectionError,
            error: const api.NetworkException(
              message: 'Cannot reach Labuda server',
            ),
          ),
        ),
        imageUploader: _FakeImageUploader(),
      );

      await expectLater(
        () => repo.getReportsByUser(userId: 'user-1'),
        throwsA(isA<ReportRepositoryException>()),
      );
    });

    test('maps timeout DioException to network failure', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/reports/mine'),
            type: DioExceptionType.receiveTimeout,
            error: const api.TimeoutException(),
          ),
        ),
        imageUploader: _FakeImageUploader(),
      );

      await expectLater(
        () => repo.getReportsByUser(userId: 'user-1'),
        throwsA(isA<ReportRepositoryException>()),
      );
    });

    test('maps malformed payload to network failure', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: const FormatException('bad json'),
        ),
        imageUploader: _FakeImageUploader(),
      );

      await expectLater(
        () => repo.getReportsByUser(userId: 'user-1'),
        throwsA(isA<ReportRepositoryException>()),
      );
    });
  });
}

class FakeReportApiDatasource implements ReportApiDatasource {
  final Object? error;

  FakeReportApiDatasource({this.error});

  @override
  Future<List<ReportDto>> getMyReports({int page = 1}) async {
    if (error != null) {
      throw error!;
    }
    return const [];
  }

  @override
  Future<ReportDto> createReport(CreateReportRequestDto request) =>
      throw UnimplementedError();

  @override
  Future<ReportDto> getReport(String reportId) => throw UnimplementedError();

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

class _FakeImageUploader implements ImageUploader {
  @override
  Future<String> uploadImage({
    required String userId,
    required String filePath,
  }) async {
    return 'https://example.com/evidence.jpg';
  }
}
