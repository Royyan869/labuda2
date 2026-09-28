/// Support API Repository Implementation
///
/// Implementation of SupportRepository using Go API via datasource.
/// This is the target implementation that will replace Firestore-based version.
library;

import 'dart:async';

import 'package:labuda/core/api/api_error_codes.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/system/support/data/datasources/support_api_datasource.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';

/// Implementation of SupportRepository using Go API
///
/// This class:
/// - Uses SupportApiDatasource for Go API operations
/// - Converts `Result<T>` to SupportResult
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
  Future<SupportResult<String>> createTicket({
    required String userId,
    required String userName,
    String? userAvatar,
    required SupportCategory category,
    SupportPriority priority = SupportPriority.medium,
    String? subject,
    String? description,
    String? linkedOrderId,
  }) async {
    // Identity is NOT sent: the backend derives the owner from the session.
    // `userId`/`userName`/`userAvatar` remain local display context only.
    assert(
      userId.isNotEmpty && userName.isNotEmpty,
      'support ticket creation requires a local owner context',
    );
    try {
      final result = await _datasource.createTicket(
        category: category.wireValue,
        priority: priority.name,
        subject: subject,
        description: description,
        linkedOrderId: linkedOrderId,
      );

      if (result.isError) {
        _logger?.error('Failed to create support ticket: ${result.error}');
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      return SupportResult.success(result.data!.id);
    } catch (e, stackTrace) {
      _logger?.error('Error creating support ticket', stackTrace: stackTrace);
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

      if (result.isError) {
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      final dto = result.data;
      if (dto == null) {
        return SupportResult.failure(
          const SupportFailureNotFound(message: 'Support ticket not found'),
        );
      }
      return SupportResult.success(dto.toEntity());
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket', stackTrace: stackTrace);
      return SupportResult.failure(
        SupportFailureUnknown(message: e.toString(), originalError: e),
      );
    }
  }

  @override
  Future<SupportResult<List<SupportTicket>>> getMyTickets({
    int limit = 50,
  }) async {
    try {
      final result = await _datasource.getMyTickets(limit: limit);

      if (result.isError) {
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      return SupportResult.success(
        result.data!.map((dto) => dto.toEntity()).toList(),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error listing my tickets', stackTrace: stackTrace);
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

      if (result.isError) {
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      return SupportResult.success(null);
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

      if (result.isError) {
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      return SupportResult.success(
        result.data!.map((dto) => dto.toEntity()).toList(),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket messages', stackTrace: stackTrace);
      return SupportResult.failure(
        SupportFailureUnknown(message: e.toString(), originalError: e),
      );
    }
  }

  @override
  Future<SupportResult<void>> sendMessage({
    required String ticketId,
    required String message,
  }) async {
    final trimmed = message.trim();
    if (trimmed.isEmpty) {
      return SupportResult.failure(
        const SupportFailureValidation(message: 'Message cannot be empty'),
      );
    }

    try {
      final result = await _datasource.sendMessage(
        ticketId: ticketId,
        message: trimmed,
      );

      if (result.isError) {
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      return SupportResult.success(null);
    } catch (e, stackTrace) {
      _logger?.error('Error sending ticket message', stackTrace: stackTrace);
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

      if (result.isError) {
        return SupportResult.failure(_mapApiErrorToFailure(result));
      }
      return SupportResult.success(
        result.data!.map((dto) => dto.toEntity()).toList(),
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

  /// Classify a failed `Result` into a [SupportFailure].
  ///
  /// The classification branches on the two machine-readable channels the API
  /// layer preserved — `Result.statusCode` (HTTP envelope) and
  /// `Result.errorCode` (backend/transport code) — never on the human message.
  /// Message-text matching (`'already assigned'`, `'network'`, …) silently
  /// mislabels every failure whose wording drifts, and it can never see a
  /// transport failure at all: those carry a transport code from
  /// `api_error_codes.dart`, never a connectivity word in the message.
  SupportFailure _mapApiErrorToFailure(Result<Object?> result) {
    final error = result.error ?? 'Request failed';
    final code = result.errorCode;

    // Transport failures are decided by the canonical transport predicate,
    // NOT by whether the message happens to contain a connectivity word.
    if (isTransportFailureCode(code)) {
      return SupportFailureNetwork(message: error, originalError: code);
    }

    switch (result.statusCode) {
      case 401:
      case 403:
        return SupportFailurePermission(message: error, originalError: code);
      case 404:
        return SupportFailureNotFound(message: error, originalError: code);
      case 409:
        return SupportFailureAlreadyAssigned(
          message: error,
          originalError: code,
        );
      case 400:
      case 422:
        return SupportFailureValidation(message: error, originalError: code);
      default:
        return SupportFailureUnknown(message: error, originalError: code);
    }
  }
}
