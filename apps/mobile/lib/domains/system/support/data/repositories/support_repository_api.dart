/// Support API Repository Implementation
///
/// Implementation of SupportRepository using Go API via datasource.
/// This is the target implementation that will replace Firestore-based version.
library;

import 'dart:async';

import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/src/interfaces/services/i_logger_service.dart';
import 'package:hishumi/domains/system/support/data/datasources/support_api_datasource.dart';
import 'package:hishumi/domains/system/support/domain/domain.dart';

/// Implementation of SupportRepository using Go API
///
/// This class:
/// - Uses SupportApiDatasource for Go API operations
/// - Returns the canonical `Result<T>` with all error channels intact
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
  Future<Result<String>> createTicket({
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
        return _forwardError(result);
      }
      return Result.success(result.data!.id);
    } catch (e, stackTrace) {
      _logger?.error('Error creating support ticket', stackTrace: stackTrace);
      return Result.error('Failed to create support ticket');
    }
  }

  // ============================================================
  // READ OPERATIONS
  // ============================================================

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) async {
    try {
      final result = await _datasource.getTicket(ticketId);

      if (result.isError) {
        return _forwardError(result);
      }
      final dto = result.data;
      if (dto == null) {
      return Result.error('Support ticket not found');
      }
      return Result.success(dto.toEntity());
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket', stackTrace: stackTrace);
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<SupportTicket>>> getMyTickets({
    int limit = 50,
  }) async {
    try {
      final result = await _datasource.getMyTickets(limit: limit);

      if (result.isError) {
        return _forwardError(result);
      }
      return Result.success(
        result.data!.map((dto) => dto.toEntity()).toList(),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error listing my tickets', stackTrace: stackTrace);
      return Result.error(e.toString());
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
  Future<Result<void>> reopenTicket(ReopenTicketRequest request) async {
    try {
      final result = await _datasource.reopenTicket(
        ticketId: request.ticketId,
        userId: request.userId,
      );

      if (result.isError) {
        return _forwardError(result);
      }
      return Result.success(null);
    } catch (e, stackTrace) {
      _logger?.error('Error reopening ticket', stackTrace: stackTrace);
      return Result.error(e.toString());
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
  Future<Result<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) async {
    try {
      final result = await _datasource.getMessages(ticketId, limit: limit);

      if (result.isError) {
        return _forwardError(result);
      }
      return Result.success(
        result.data!.map((dto) => dto.toEntity()).toList(),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket messages', stackTrace: stackTrace);
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> sendMessage({
    required String ticketId,
    required String message,
  }) async {
    final trimmed = message.trim();
    if (trimmed.isEmpty) {
      return Result.error('Message cannot be empty');
    }

    try {
      final result = await _datasource.sendMessage(
        ticketId: ticketId,
        message: trimmed,
      );

      if (result.isError) {
        return _forwardError(result);
      }
      return Result.success(null);
    } catch (e, stackTrace) {
      _logger?.error('Error sending ticket message', stackTrace: stackTrace);
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async {
    try {
      final result = await _datasource.getEvents(ticketId, limit: limit);

      if (result.isError) {
        return _forwardError(result);
      }
      return Result.success(
        result.data!.map((dto) => dto.toEntity()).toList(),
      );
    } catch (e, stackTrace) {
      _logger?.error('Error getting ticket events', stackTrace: stackTrace);
      return Result.error(e.toString());
    }
  }

  // ============================================================
  // HELPER METHODS
  // ============================================================

  /// Forward a failed datasource `Result` with ALL canonical error channels
  /// intact — message, machine-readable code, HTTP status, structured details.
  ///  /// The old duplicate result wrapper with its typed-failure family died in
  /// the one-vocabulary convergence: no consumer ever branched on the type,
  /// and every channel it kept (the machine code) is already carried by the
  /// canonical Result — which additionally preserves the HTTP status it dropped.
  Result<T> _forwardError<T>(Result<Object?> result) => Result.error(
    result.error ?? 'Request failed',
    code: result.errorCode,
    statusCode: result.statusCode,
    details: result.errorDetails,
  );
}
