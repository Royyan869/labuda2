import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart' as api;
import 'package:labuda/domains/system/report/data/dto/dto.dart';
import 'package:labuda/domains/system/report/data/remote/report_api_datasource.dart';
import 'package:labuda/domains/system/report/data/repositories/report_repository_impl.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';

void main() {
  group('ReportRepositoryImpl.getReportsByUser', () {
    test('maps backend-unreachable DioException to backendUnavailable', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/moderation/my-cases'),
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
        throwsA(
          isA<ReportRepositoryException>().having(
            (e) => e.type,
            'type',
            ReportFailureType.backendUnavailable,
          ),
        ),
      );
    });

    test('maps timeout DioException to timeout', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/moderation/my-cases'),
            type: DioExceptionType.receiveTimeout,
            error: const api.TimeoutException(),
          ),
        ),
        imageUploader: _FakeImageUploader(),
      );

      await expectLater(
        () => repo.getReportsByUser(userId: 'user-1'),
        throwsA(
          isA<ReportRepositoryException>().having(
            (e) => e.type,
            'type',
            ReportFailureType.timeout,
          ),
        ),
      );
    });

    test('maps malformed payload to malformedResponse', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: const FormatException('bad json'),
        ),
        imageUploader: _FakeImageUploader(),
      );

      await expectLater(
        () => repo.getReportsByUser(userId: 'user-1'),
        throwsA(
          isA<ReportRepositoryException>().having(
            (e) => e.type,
            'type',
            ReportFailureType.malformedResponse,
          ),
        ),
      );
    });
  });
}

class FakeReportApiDatasource implements ReportApiDatasource {
  final Object? error;

  FakeReportApiDatasource({this.error});

  @override
  Future<PagedModerationCases> getMyCases({
    String? status,
    int page = 1,
    int limit = 20,
  }) async {
    if (error != null) {
      throw error!;
    }
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
  Future<ModerationCaseDto> getCase(String caseId) => throw UnimplementedError();

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
