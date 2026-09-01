/// Report API Datasource
///
/// Handles HTTP communication with Report, Warning, and Appeal API endpoints.
/// This isolates all API calls to a single class.
library;

import 'package:labuda/core/api/api_client.dart';
import 'package:dio/dio.dart';
import '../dto/dto.dart';

/// Abstract interface for Report API
abstract class ReportApiDatasource {
  // =====================
  // Report APIs (POST /reports, GET /reports/mine, GET /reports/:id)
  // =====================

  Future<ReportDto> createReport(CreateReportRequestDto request);
  Future<ReportDto> getReport(String reportId);
  Future<List<ReportDto>> getMyReports({int page = 1});

  // =====================
  // Appeal APIs
  // =====================

  Future<AppealDto> createAppeal(CreateAppealRequestDto request);
  Future<AppealDto> getAppeal(String appealId);
  Future<List<AppealDto>> getMyAppeals({String? status, int page = 1});
  // REMOVED: getAppeals() - Admin-only endpoint
  // REMOVED: getPendingAppeals() - Admin-only endpoint
  // REMOVED: reviewAppeal() - Admin-only endpoint

  // =====================
  // Warning APIs (Read-only for users)
  // =====================

  // REMOVED: issueWarning() - Admin-only endpoint
  Future<UserWarningDto> getWarning(String warningId);
  Future<List<UserWarningDto>> getUserWarnings(
    String userId, {
    String? status,
    int page = 1,
  });
  Future<List<UserWarningDto>> getActiveWarnings(String userId);
  // REMOVED: revokeWarning() - Admin-only endpoint
}

/// Implementation using core ApiClient
class ReportApiDatasourceImpl implements ReportApiDatasource {
  final ApiClient _apiClient;

  ReportApiDatasourceImpl(this._apiClient);

  // Helper method to extract data from response
  Map<String, dynamic> _extractData(Response response) {
    return response.data['data'] as Map<String, dynamic>;
  }

  // Helper method to extract list from response
  List<Map<String, dynamic>> _extractList(
    Response response, {
    String key = 'data',
  }) {
    final data = response.data[key] as List<dynamic>;
    return data.map((e) => e as Map<String, dynamic>).toList();
  }

  // =====================
  // Report APIs
  // =====================

  @override
  Future<ReportDto> createReport(CreateReportRequestDto request) async {
    final response = await _apiClient.post(
      '/reports',
      data: request.toJson(),
    );
    return ReportDto.fromJson(_extractData(response));
  }

  @override
  Future<ReportDto> getReport(String reportId) async {
    final response = await _apiClient.get('/reports/$reportId');
    final data = response.data['data'] as Map<String, dynamic>;
    final report = data['report'] as Map<String, dynamic>? ?? data;
    return ReportDto.fromJson(report);
  }

  @override
  Future<List<ReportDto>> getMyReports({int page = 1}) async {
    final response = await _apiClient.get(
      '/reports/mine',
      queryParameters: {'page': page, 'limit': 20},
    );
    final data = response.data['data'] as Map<String, dynamic>;
    final reports = data['reports'] as List<dynamic>? ?? [];
    return reports
        .map((json) => ReportDto.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  // =====================
  // Appeal APIs
  // =====================

  @override
  Future<AppealDto> createAppeal(CreateAppealRequestDto request) async {
    final response = await _apiClient.post('/appeals', data: request.toJson());
    return AppealDto.fromJson(_extractData(response));
  }

  @override
  Future<AppealDto> getAppeal(String appealId) async {
    final response = await _apiClient.get('/appeals/$appealId');
    final data = _extractData(response);
    final appealData = data['appeal'] as Map<String, dynamic>? ?? data;
    return AppealDto.fromJson(appealData);
  }

  @override
  Future<List<AppealDto>> getMyAppeals({String? status, int page = 1}) async {
    final queryParams = <String, dynamic>{
      'page': page,
      'page_size': 20,
    };
    if (status != null) queryParams['status'] = status;
    final response = await _apiClient.get(
      '/appeals/me',
      queryParameters: queryParams,
    );
    final data = _extractData(response);
    final appeals = data['appeals'] as List<dynamic>? ?? [];
    return appeals
        .map((json) => AppealDto.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  // REMOVED: getAppeals() - Admin-only endpoint
  // REMOVED: getPendingAppeals() - Admin-only endpoint
  // REMOVED: reviewAppeal() - Admin-only endpoint

  // =====================
  // Warning APIs (Read-only for users)
  // =====================

  // REMOVED: issueWarning() - Admin-only endpoint

  @override
  Future<UserWarningDto> getWarning(String warningId) async {
    final response = await _apiClient.get('/warnings/$warningId');
    return UserWarningDto.fromJson(_extractData(response));
  }

  @override
  Future<List<UserWarningDto>> getUserWarnings(
    String userId, {
    String? status,
    int page = 1,
  }) async {
    final response = await _apiClient.get(
      '/users/$userId/warnings',
      queryParameters: {'status': ?status, 'page': page, 'page_size': 20},
    );
    final data = _extractList(response);
    return data.map((json) => UserWarningDto.fromJson(json)).toList();
  }

  @override
  Future<List<UserWarningDto>> getActiveWarnings(String userId) async {
    final response = await _apiClient.get('/users/$userId/warnings/active');
    final data = _extractList(response, key: 'warnings');
    return data.map((json) => UserWarningDto.fromJson(json)).toList();
  }

  // REMOVED: revokeWarning() - Admin-only endpoint
}
