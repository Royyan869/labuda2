/// Report Repository Implementation
///
/// API-based implementation of ReportRepository.
library;

import 'dart:async';
import 'package:dio/dio.dart';

import '../../domain/entities/entities.dart';
import '../../domain/repositories/report_repository.dart';
import '../mappers/mappers.dart';
import '../remote/report_api_datasource.dart';

/// Report Repository Implementation
class ReportRepositoryImpl implements ReportRepository {
  final ReportApiDatasource _datasource;

  ReportRepositoryImpl({
    required ReportApiDatasource datasource,
  }) : _datasource = datasource;

  // =====================
  // User Operations
  // =====================

  @override
  Future<Report> createReport({
    required String reporterId,
    required CreateReportRequest request,
  }) async {
    try {
      final dto = await _datasource.createReport(
        ReportMapper.toCreateRequestDto(request),
      );

      return ReportMapper.toEntity(dto);
    } on DioException catch (e) {
      final statusCode = e.response?.statusCode;
      final errorMsg = _extractErrorMessage(e);

      if (statusCode == 409) {
        throw ReportRepositoryException(
          'Anda sudah melaporkan konten ini.',
          type: ReportFailureType.alreadyReported,
        );
      } else if (statusCode == 404) {
        throw ReportRepositoryException(
          'Target laporan tidak ditemukan.',
          type: ReportFailureType.notFound,
        );
      } else if (statusCode == 400) {
        throw ReportRepositoryException(
          errorMsg ?? 'Laporan tidak valid.',
          type: ReportFailureType.validation,
        );
      }
      throw ReportRepositoryException(
        errorMsg ?? 'Gagal membuat laporan.',
        type: ReportFailureType.network,
      );
    } catch (e) {
      if (e is ReportRepositoryException) rethrow;
      throw ReportRepositoryException(
        'Gagal membuat laporan: ${e.toString()}',
        type: ReportFailureType.network,
      );
    }
  }

  @override
  Future<Report?> getReportById(String reportId) async {
    try {
      final dto = await _datasource.getReport(reportId);
      return ReportMapper.toEntity(dto);
    } on DioException catch (e) {
      if (e.response?.statusCode == 404) return null;
      throw ReportRepositoryException(
        _extractErrorMessage(e) ?? 'Gagal mengambil laporan',
        type: ReportFailureType.network,
      );
    } catch (e) {
      throw ReportRepositoryException(
        'Gagal mengambil laporan: ${e.toString()}',
        type: ReportFailureType.network,
      );
    }
  }

  @override
  Future<List<Report>> getReportsByUser({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async {
    try {
      final dtos = await _datasource.getMyReports(page: page);
      return dtos.map((dto) => ReportMapper.toEntity(dto)).toList();
    } catch (e) {
      throw ReportRepositoryException(
        'Gagal mengambil daftar laporan: ${e.toString()}',
        type: ReportFailureType.network,
      );
    }
  }

  @override
  Future<bool> hasUserReported({
    required String userId,
    required String targetId,
    required ReportTargetType targetType,
  }) async {
    try {
      final targetTypeStr = targetType.backendValue;
      // Paginate through all user reports to close the first-page-only hole.
      // Backend unique constraint is final guard, but client check must not
      // falsely return false when duplicate is beyond page 1.
      int page = 1;
      while (true) {
        final dtos = await _datasource.getMyReports(page: page);
        if (dtos.isEmpty) return false;
        if (dtos.any((dto) => dto.subjectId == targetId && dto.subjectType == targetTypeStr)) {
          return true;
        }
        if (dtos.length < 20) return false;
        page++;
        if (page > 50) return false; // safety cap: 1000 reports
      }
    } catch (e) {
      return false;
    }
  }

  String? _extractErrorMessage(DioException e) {
    if (e.response?.data is Map<String, dynamic>) {
      final data = e.response!.data as Map<String, dynamic>;
      if (data.containsKey('message') && data['message'] is String) {
        return data['message'] as String;
      }
      if (data.containsKey('error') && data['error'] is String) {
        return data['error'] as String;
      }
    }
    return e.message;
  }
}

/// Custom exception for repository errors
class ReportRepositoryException implements Exception {
  final String message;
  final ReportFailureType type;

  const ReportRepositoryException(
    this.message, {
    this.type = ReportFailureType.unknown,
  });

  @override
  String toString() => message;
}
