import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart' as api;
import 'package:labuda/domains/system/report/data/dto/dto.dart';
import 'package:labuda/domains/system/report/data/remote/report_api_datasource.dart';
import 'package:labuda/domains/system/report/data/repositories/report_repository_impl.dart';
import 'package:labuda/domains/system/report/domain/entities/report.dart';
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
      );

      await expectLater(
        () => repo.getReportsByUser(userId: 'user-1'),
        throwsA(isA<ReportRepositoryException>()),
      );
    });
  });

  group('ReportRepositoryImpl.createReport error mapping', () {
    test('maps 409 to alreadyReported', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/reports'),
            response: Response(requestOptions: RequestOptions(path: '/reports'), statusCode: 409, data: {'message': 'already reported'}),
            type: DioExceptionType.badResponse,
          ),
        ),
      );
      try {
        await repo.createReport(reporterId: 'u1', request: const CreateReportRequest(subjectId: 't1', subjectType: ReportTargetType.content, reason: ReportReasonType.other));
        fail('expected exception');
      } catch (e) {
        expect((e as ReportRepositoryException).type, ReportFailureType.alreadyReported);
      }
    });

    test('maps 404 to notFound', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/reports'),
            response: Response(requestOptions: RequestOptions(path: '/reports'), statusCode: 404, data: {'message': 'not found'}),
            type: DioExceptionType.badResponse,
          ),
        ),
      );
      try {
        await repo.createReport(reporterId: 'u1', request: const CreateReportRequest(subjectId: 't1', subjectType: ReportTargetType.content, reason: ReportReasonType.other));
        fail('expected exception');
      } catch (e) {
        expect((e as ReportRepositoryException).type, ReportFailureType.notFound);
      }
    });

    test('maps 400 to validation', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasource(
          error: DioException(
            requestOptions: RequestOptions(path: '/reports'),
            response: Response(requestOptions: RequestOptions(path: '/reports'), statusCode: 400, data: {'message': 'cannot report your own'}),
            type: DioExceptionType.badResponse,
          ),
        ),
      );
      try {
        await repo.createReport(reporterId: 'u1', request: const CreateReportRequest(subjectId: 't1', subjectType: ReportTargetType.content, reason: ReportReasonType.other));
        fail('expected exception');
      } catch (e) {
        expect((e as ReportRepositoryException).type, ReportFailureType.validation);
      }
    });
  });

  group('ReportRepositoryImpl.hasUserReported dedup', () {
    test('matches both type and id — different type same id is not duplicate', () async {
      final repo = ReportRepositoryImpl(
        datasource: FakeReportApiDatasourceWithReports(reports: [
          ReportDto(id: 'r1', reporterId: 'u1', subjectType: 'content', subjectId: 'id-123', reasonCode: 'other', createdAt: DateTime.now()),
        ]),
      );
      expect(await repo.hasUserReported(userId: 'u1', targetId: 'id-123', targetType: ReportTargetType.content), isTrue);
      expect(await repo.hasUserReported(userId: 'u1', targetId: 'id-123', targetType: ReportTargetType.user), isFalse);
    });

    test('finds duplicate beyond first page via pagination', () async {
      final page1 = List.generate(20, (i) => ReportDto(id: 'r$i', reporterId: 'u1', subjectType: 'content', subjectId: 'other-$i', reasonCode: 'other', createdAt: DateTime.now()));
      final page2 = [ReportDto(id: 'r-target', reporterId: 'u1', subjectType: 'auction', subjectId: 'target-999', reasonCode: 'other', createdAt: DateTime.now())];
      final repo = ReportRepositoryImpl(
        datasource: FakePaginatedDatasource(pages: {1: page1, 2: page2}),
      );
      expect(await repo.hasUserReported(userId: 'u1', targetId: 'target-999', targetType: ReportTargetType.auction), isTrue);
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
  Future<ReportDto> createReport(CreateReportRequestDto request) async {
    if (error != null) {
      throw error!;
    }
    throw UnimplementedError();
  }

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

class FakeReportApiDatasourceWithReports implements ReportApiDatasource {
  final List<ReportDto> reports;
  FakeReportApiDatasourceWithReports({required this.reports});
  @override
  Future<List<ReportDto>> getMyReports({int page = 1}) async => reports;
  @override
  Future<ReportDto> createReport(CreateReportRequestDto request) => throw UnimplementedError();
  @override
  Future<ReportDto> getReport(String reportId) => throw UnimplementedError();
  @override
  Future<AppealDto> createAppeal(CreateAppealRequestDto request) => throw UnimplementedError();
  @override
  Future<AppealDto> getAppeal(String appealId) => throw UnimplementedError();
  @override
  Future<List<AppealDto>> getMyAppeals({String? status, int page = 1}) => throw UnimplementedError();
  @override
  Future<UserWarningDto> getWarning(String warningId) => throw UnimplementedError();
  @override
  Future<List<UserWarningDto>> getUserWarnings(String userId, {String? status, int page = 1}) => throw UnimplementedError();
  @override
  Future<List<UserWarningDto>> getActiveWarnings(String userId) => throw UnimplementedError();
}

class FakePaginatedDatasource implements ReportApiDatasource {
  final Map<int, List<ReportDto>> pages;
  FakePaginatedDatasource({required this.pages});
  @override
  Future<List<ReportDto>> getMyReports({int page = 1}) async => pages[page] ?? [];
  @override
  Future<ReportDto> createReport(CreateReportRequestDto request) => throw UnimplementedError();
  @override
  Future<ReportDto> getReport(String reportId) => throw UnimplementedError();
  @override
  Future<AppealDto> createAppeal(CreateAppealRequestDto request) => throw UnimplementedError();
  @override
  Future<AppealDto> getAppeal(String appealId) => throw UnimplementedError();
  @override
  Future<List<AppealDto>> getMyAppeals({String? status, int page = 1}) => throw UnimplementedError();
  @override
  Future<UserWarningDto> getWarning(String warningId) => throw UnimplementedError();
  @override
  Future<List<UserWarningDto>> getUserWarnings(String userId, {String? status, int page = 1}) => throw UnimplementedError();
  @override
  Future<List<UserWarningDto>> getActiveWarnings(String userId) => throw UnimplementedError();
}
