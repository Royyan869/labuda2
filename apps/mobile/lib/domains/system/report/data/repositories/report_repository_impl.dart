/// Report Repository Implementation
///
/// API-based implementation of ReportRepository.
library;

import 'dart:async';

import '../../domain/entities/entities.dart';
import '../../domain/repositories/report_repository.dart';
import '../mappers/mappers.dart';
import '../remote/report_api_datasource.dart';

/// Report Repository Implementation
class ReportRepositoryImpl implements ReportRepository {
  final ReportApiDatasource _datasource;
  final ImageUploader _imageUploader;

  ReportRepositoryImpl({
    required ReportApiDatasource datasource,
    required ImageUploader imageUploader,
  }) : _datasource = datasource,
       _imageUploader = imageUploader;

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
    } on ReportRepositoryException {
      rethrow;
    } catch (e) {
      throw ReportRepositoryException(
        'Failed to create report: ${e.toString()}',
        type: ReportFailureType.network,
      );
    }
  }

  @override
  Future<Report?> getReportById(String reportId) async {
    try {
      final dto = await _datasource.getReport(reportId);
      return ReportMapper.toEntity(dto);
    } on ReportRepositoryException {
      return null;
    } catch (e) {
      throw ReportRepositoryException(
        'Failed to get report: ${e.toString()}',
        type: ReportFailureType.network,
      );
    }
  }

  @override
  Future<List<Report>> getReportsByUser({
    required String userId,
    int limit = 20,
  }) async {
    try {
      final dtos = await _datasource.getMyReports(page: (limit / 20).ceil());
      return dtos.map((dto) => ReportMapper.toEntity(dto)).toList();
    } catch (e) {
      throw ReportRepositoryException(
        'Failed to get user reports: ${e.toString()}',
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
      final dtos = await _datasource.getMyReports();
      return dtos.any((dto) => dto.subjectId == targetId);
    } catch (e) {
      return false;
    }
  }

  @override
  Future<String> uploadEvidence({
    required String reporterId,
    required String filePath,
  }) async {
    try {
      return await _imageUploader.uploadImage(
        userId: reporterId,
        filePath: filePath,
      );
    } catch (e) {
      throw ReportRepositoryException(
        'Failed to upload evidence: ${e.toString()}',
        type: ReportFailureType.network,
      );
    }
  }

  // =====================
  // REMOVED: All Admin Operations
  // =====================
  // - getReports() - Admin-only endpoint
  // - updateReportStatus() - Admin-only endpoint
  // - getReportStatistics() - Admin-only endpoint
  // - watchPendingReportsCount() - Admin-only endpoint
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
  String toString() => 'ReportRepositoryException: $message';
}

/// Abstract image uploader interface
abstract class ImageUploader {
  Future<String> uploadImage({
    required String userId,
    required String filePath,
  });
}
