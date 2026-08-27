/// Warning Repository Implementation
///
/// API-based implementation of WarningRepository.
/// V1: Passive warning records only. No acknowledge, no escalation.
library;

import 'dart:async';

import '../../domain/entities/user_warning.dart';
import '../../domain/repositories/warning_repository.dart';
import '../mappers/mappers.dart';
import '../remote/report_api_datasource.dart';

/// Warning Repository Implementation
class WarningRepositoryImpl implements WarningRepository {
  final ReportApiDatasource _datasource;
  final UserNameProvider _nameProvider;

  WarningRepositoryImpl({
    required ReportApiDatasource datasource,
    required UserNameProvider nameProvider,
  }) : _datasource = datasource,
       _nameProvider = nameProvider;

  // =====================
  // User Operations
  // =====================

  @override
  Future<List<UserWarning>> getUserWarnings(
    String userId, {
    WarningStatus? status,
    int limit = 20,
  }) async {
    try {
      final dtos = await _datasource.getUserWarnings(
        userId,
        status: status?.value,
        page: (limit / 20).ceil(),
      );

      // Get admin names for each warning
      final warnings = <UserWarning>[];
      for (final dto in dtos) {
        final adminName = await _nameProvider.getUserName(dto.issuedBy);
        warnings.add(WarningMapper.toEntity(dto, adminName: adminName));
      }
      return warnings;
    } catch (e) {
      throw WarningRepositoryException(
        'Failed to get user warnings: ${e.toString()}',
        type: WarningFailureType.network,
      );
    }
  }

  @override
  Future<List<UserWarning>> getActiveWarnings(String userId) async {
    try {
      final dtos = await _datasource.getActiveWarnings(userId);

      final warnings = <UserWarning>[];
      for (final dto in dtos) {
        final adminName = await _nameProvider.getUserName(dto.issuedBy);
        warnings.add(WarningMapper.toEntity(dto, adminName: adminName));
      }
      return warnings;
    } catch (e) {
      throw WarningRepositoryException(
        'Failed to get active warnings: ${e.toString()}',
        type: WarningFailureType.network,
      );
    }
  }

  @override
  Future<UserWarning?> getWarningById(String warningId) async {
    try {
      final dto = await _datasource.getWarning(warningId);
      final adminName = await _nameProvider.getUserName(dto.issuedBy);
      return WarningMapper.toEntity(dto, adminName: adminName);
    } on WarningRepositoryException {
      return null;
    } catch (e) {
      throw WarningRepositoryException(
        'Failed to get warning: ${e.toString()}',
        type: WarningFailureType.network,
      );
    }
  }

  @override
  Future<bool> hasActiveWarnings(String userId) async {
    try {
      final warnings = await getActiveWarnings(userId);
      return warnings.any((w) => w.isCurrentlyActive);
    } catch (e) {
      return false;
    }
  }

  @override
  Stream<int> watchActiveWarningsCount(String userId) {
    // Poll for active warnings count
    return Stream.periodic(const Duration(seconds: 30), (_) => 0).asyncMap((
      _,
    ) async {
      try {
        final warnings = await getActiveWarnings(userId);
        return warnings.where((w) => w.isCurrentlyActive).length;
      } catch (_) {
        return 0;
      }
    });
  }

  // =====================
  // REMOVED: All Admin Operations (Pure Admin)
  // =====================
  // - issueWarning() - Admin-only endpoint
  // - revokeWarning() - Admin-only endpoint
  //
  // Users can only READ warnings, not create or revoke them
}

/// Custom exception for repository errors
class WarningRepositoryException implements Exception {
  final String message;
  final WarningFailureType type;

  const WarningRepositoryException(
    this.message, {
    this.type = WarningFailureType.unknown,
  });

  @override
  String toString() => 'WarningRepositoryException: $message';
}

/// Abstract user name provider interface
abstract class UserNameProvider {
  Future<String> getUserName(String userId);
}
