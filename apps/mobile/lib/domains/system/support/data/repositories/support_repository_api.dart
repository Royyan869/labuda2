/// Support API Repository Implementation
///
/// Implementation of SupportRepository using Go API via datasource.
/// This is the target implementation that will replace Firestore-based version.
library;

import 'dart:async';

import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/system/support/data/datasources/support_api_datasource.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';

/// Implementation of SupportRepository using Go API
///
/// This class:
/// - Uses SupportApiDatasource for Go API operations
/// - Converts ApiResult to SupportResult
/// - Returns `SupportResult<T>` for all operations
/// - Knows nothing about Firebase/Firestore
class SupportRepositoryApi implements SupportRepository {
  final SupportApiDatasource _datasource;
  final ILoggerService? _logger;

  SupportRepositoryApi({
    required SupportApiDatasource datasource,
    ILoggerService? logger,
  }) : _datasource = datasource,
       _logger = logger;

  // ============================================================
  // CREATE OPERATIONS
  // ============================================================

  @override
  Future<SupportResult<String>> createSupportChat({
    required String userId,
    required String userName,
    String? userAvatar,
    required SupportCategory category,
    SupportPriority priority = SupportPriority.medium,
    String? description,
    String? linkedOrderId,
  }) async {
    try {
      final result = await _datasource.createTicket(
        userId: userId,
        userName: userName,
        userAvatar: userAvatar,
        category: category.name,
        priority: priority.name,
        description: description,
        linkedOrderId: linkedOrderId,
      );

      return result.fold(
        onError: (error, code) {
          _logger?.error('Failed to create support ticket: $error');
          return SupportResult.failure(_mapApiErrorToFailure(error, code));
        },
        onSuccess: (dto) => SupportResult.success(dto.id),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error creating support chat', stackTrace: stackTrace);
      return SupportResult.failure(
        const SupportFailureNetwork(message: 'Failed to create support ticket'),
      );
    }
  }

  // ============================================================
  // READ OPERATIONS
  // ============================================================

  @override
  Future<SupportResult<SupportTicket>> getTicket(String ticketId) async {
    try {
      final result = await _datasource.getTicket(ticketId);

      return result.fold(
        onError: (error, code) {
          if (code == '404') {
            return SupportResult.failure(
              SupportFailureNotFound(message: error, originalError: code),
            );
          }
          return SupportResult.failure(_mapApiErrorToFailure(error, code));
        },
        onSuccess: (dto) {
          if (dto == null) {
            return SupportResult.failure(
              const SupportFailureNotFound(message: 'Support ticket not found'),
            );
          }
          return SupportResult.success(dto.toEntity());
        },
      );
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket', stackTrace: stackTrace);
      return SupportResult.failure(
        SupportFailureUnknown(message: e.toString(), originalError: e),
      );
    }
  }

  // REMOVED: watchTickets() - Admin-only endpoint
  // REMOVED: _getTicketsSync() - Admin helper method
  // REMOVED: watchUnclaimedTicketsCount() - Admin-only endpoint
  // REMOVED: watchOpenTicketsCount() - Admin-only endpoint
  // REMOVED: watchMyTicketsCount() - Admin-only endpoint
  // REMOVED: getStatistics() - Admin-only endpoint

  // ============================================================
  // UPDATE OPERATIONS
  // ============================================================

  // REMOVED: claimTicket() - Admin-only endpoint
  // REMOVED: resolveTicket() - Admin-only endpoint

  @override
  Future<SupportResult<void>> reopenTicket(ReopenTicketRequest request) async {
    try {
      final result = await _datasource.reopenTicket(
        ticketId: request.ticketId,
        userId: request.userId,
      );

      return result.fold(
        onError: (error, code) =>
            SupportResult.failure(_mapApiErrorToFailure(error, code)),
        onSuccess: (_) => SupportResult.success(null),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error reopening ticket', stackTrace: stackTrace);
      return SupportResult.failure(
        SupportFailureUnknown(message: e.toString(), originalError: e),
      );
    }
  }

  // REMOVED: closeTicket() - Admin-only endpoint
  // REMOVED: updateTicketPriority() - Admin-only endpoint
  // REMOVED: updateTicketCategory() - Admin-only endpoint

  // REMOVED: sendGreetingMessage() - Admin-only endpoint
  // REMOVED: sendSystemMessage() - Admin-only endpoint

  // REMOVED: All ADMIN OPERATIONS
  // - getSupportAdminIds()
  // - notifyAdminsAboutNewTicket()

  // ============================================================
  // EVENT OPERATIONS
  // ============================================================

  @override
  Future<SupportResult<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) async {
    try {
      final result = await _datasource.getMessages(ticketId, limit: limit);

      return result.fold(
        onError: (error, code) =>
            SupportResult.failure(_mapApiErrorToFailure(error, code)),
        onSuccess: (dtos) =>
            SupportResult.success(dtos.map((dto) => dto.toEntity()).toList()),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket messages', stackTrace: stackTrace);
      return SupportResult.failure(
        SupportFailureUnknown(message: e.toString(), originalError: e),
      );
    }
  }

  @override
  Future<SupportResult<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async {
    try {
      final result = await _datasource.getEvents(ticketId, limit: limit);

      return result.fold(
        onError: (error, code) =>
            SupportResult.failure(_mapApiErrorToFailure(error, code)),
        onSuccess: (dtos) =>
            SupportResult.success(dtos.map((dto) => dto.toEntity()).toList()),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket events', stackTrace: stackTrace);
      return SupportResult.failure(
        SupportFailureUnknown(message: e.toString(), originalError: e),
      );
    }
  }

  // ============================================================
  // HELPER METHODS
  // ============================================================

  /// Map API error code to SupportFailure
  SupportFailure _mapApiErrorToFailure(String error, String? code) {
    switch (code) {
      case '403':
      case '401':
        return SupportFailurePermission(message: error);
      case '404':
        return SupportFailureNotFound(message: error);
      case '409':
        return SupportFailureAlreadyAssigned(message: error);
      case '400':
        return SupportFailureValidation(message: error);
      default:
        if (error.contains('already assigned')) {
          return SupportFailureAlreadyAssigned(message: error);
        }
        if (error.contains('already resolved')) {
          return SupportFailureAlreadyResolved(message: error);
        }
        if (error.contains('network') ||
            error.contains('connection') ||
            error.contains('timeout')) {
          return SupportFailureNetwork(message: error);
        }
        return SupportFailureUnknown(message: error, originalError: code);
    }
  }
}
