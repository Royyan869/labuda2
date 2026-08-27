/// Appeal Repository Implementation
///
/// API-based implementation of AppealRepository.
library;

import 'dart:async';

import '../../domain/entities/entities.dart';
import '../../domain/repositories/appeal_repository.dart';
import '../mappers/mappers.dart';
import '../remote/report_api_datasource.dart';

/// Appeal Repository Implementation
class AppealRepositoryImpl implements AppealRepository {
  final ReportApiDatasource _datasource;

  AppealRepositoryImpl({required ReportApiDatasource datasource})
    : _datasource = datasource;

  // =====================
  // User Operations
  // =====================

  @override
  Future<Appeal> submitAppeal(CreateAppealRequest request) async {
    try {
      final dto = await _datasource.createAppeal(
        AppealMapper.toCreateRequestDto(request),
      );
      return AppealMapper.toEntity(dto);
    } on AppealRepositoryException {
      rethrow;
    } catch (e) {
      throw AppealRepositoryException(
        'Failed to submit appeal: ${e.toString()}',
        type: AppealFailureType.network,
      );
    }
  }

  @override
  Future<List<Appeal>> getUserAppeals(
    String userId, {
    AppealStatus? status,
    int limit = 20,
  }) async {
    try {
      final dtos = await _datasource.getMyAppeals(
        status: status?.value,
        page: (limit / 20).ceil(),
      );
      return dtos.map((dto) => AppealMapper.toEntity(dto)).toList();
    } catch (e) {
      throw AppealRepositoryException(
        'Failed to get user appeals: ${e.toString()}',
        type: AppealFailureType.network,
      );
    }
  }

  @override
  Future<Appeal?> getAppealById(String appealId) async {
    try {
      final dto = await _datasource.getAppeal(appealId);
      return AppealMapper.toEntity(dto);
    } on AppealRepositoryException {
      return null;
    } catch (e) {
      throw AppealRepositoryException(
        'Failed to get appeal: ${e.toString()}',
        type: AppealFailureType.network,
      );
    }
  }

  @override
  Future<void> cancelAppeal(String appealId) async {
    // NOTE: Backend does not have a cancel appeal endpoint.
    // Appeals can only be resolved by admins, not cancelled by users.
    throw AppealRepositoryException(
      'Cancel appeal not available - appeals can only be resolved by admin',
      type: AppealFailureType.cannotCancel,
    );
  }

  @override
  Future<bool> hasPendingAppeal({
    required String userId,
    required AppealType type,
    String? sourceId,
  }) async {
    try {
      final appeals = await getUserAppeals(
        userId,
        status: AppealStatus.pending,
      );
      return appeals.any(
        (a) =>
            a.userId == userId &&
            a.appealType == type &&
            (sourceId == null || a.sourceId == sourceId),
      );
    } catch (e) {
      return false;
    }
  }

  // =====================
  // REMOVED: All Admin Operations
  // =====================
  // - getAppeals() - Admin-only endpoint
  // - reviewAppeal() - Admin-only endpoint
  // - watchPendingAppealsCount() - Admin-only endpoint
}

/// Custom exception for repository errors
class AppealRepositoryException implements Exception {
  final String message;
  final AppealFailureType type;

  const AppealRepositoryException(
    this.message, {
    this.type = AppealFailureType.unknown,
  });

  @override
  String toString() => 'AppealRepositoryException: $message';
}
