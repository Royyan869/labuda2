/// Support API Datasource (Go Backend)
///
/// Implements Go API endpoints for Support operations.
/// This datasource communicates with the Go backend via REST API.
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import '../dto/support_ticket_dto.dart';
import '../dto/support_message_dto.dart';

/// API Remote datasource for Support using Go Backend (User-side only)
///
/// This implements user-side support operations using Go API endpoints.
/// Admin endpoints have been removed from mobile app.
///
/// User API Endpoints:
/// - POST   /api/v1/support/tickets (create ticket)
/// - GET    /api/v1/support/tickets/{ticketId} (view own ticket)
/// - PUT    /api/v1/support/tickets/{ticketId}/reopen (reopen own ticket)
/// - GET    /api/v1/support/tickets/{ticketId}/events (view ticket events)
/// - GET    /api/v1/support/tickets/{ticketId}/messages (view ticket messages)
/// - POST   /api/v1/support/tickets/{ticketId}/messages (reply to own ticket)
class SupportApiDatasource {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  static const String _basePath = '/support';

  SupportApiDatasource(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  // ============================================================
  // HELPER METHODS
  // ============================================================

  /// Execute request and return Result with data or error
  Future<ApiResult<T>> _executeRequest<T>({
    required Future<Response<dynamic>> Function() request,
    required T Function(dynamic data) parser,
  }) async {
    try {
      final response = await request();
      final data = response.data;

      if (data is Map<String, dynamic>) {
        // Handle standard API response with success field
        if (data['success'] == false && data['error'] != null) {
          final error = data['error'] as Map<String, dynamic>?;
          return ApiResult.error(
            error?['message']?.toString() ?? 'Request failed',
            code: error?['code']?.toString(),
          );
        }

        // Parse the data field if available, otherwise use entire response
        final parsedData = data['data'] ?? data;
        return ApiResult.success(parser(parsedData));
      }

      // Direct data (not wrapped in standard format)
      return ApiResult.success(parser(data));
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'API request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      return ApiResult.error(exception.message, code: exception.code);
    } catch (e, stackTrace) {
      _logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      return ApiResult.error(e.toString());
    }
  }

  /// Extract a JSON array from a response payload.
  ///
  /// The canonical Support list endpoints return `{success, data: {data: [...]}}`,
  /// so the inner array is nested one level inside the standard envelope's
  /// `data`. A bare array is also accepted (defensive, for non-enveloped
  /// shapes). Any other shape yields an empty list rather than a cast error.
  List<dynamic> _extractList(dynamic payload) {
    if (payload is List) return payload;
    if (payload is Map<String, dynamic> && payload['data'] is List) {
      return payload['data'] as List<dynamic>;
    }
    return const [];
  }

  /// Execute void request (no return data)
  Future<ApiResult<void>> _executeVoidRequest({
    required Future<Response<dynamic>> Function() request,
  }) async {
    try {
      await request();
      return ApiResult.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'API request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      return ApiResult.error(exception.message, code: exception.code);
    } catch (e, stackTrace) {
      _logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      return ApiResult.error(e.toString());
    }
  }

  // ============================================================
  // TICKET OPERATIONS (Go API)
  // ============================================================

  /// Create support ticket via Go API.
  ///
  /// POST /api/v1/support/tickets
  ///
  /// The payload uses the canonical Go contract exactly: `category` carries a
  /// canonical wire value, and the linked order/reference is sent as
  /// `linked_order_id`. Ownership/identity is derived by the backend from the
  /// authenticated session — the client never sends a user id as authority.
  Future<ApiResult<SupportTicketDto>> createTicket({
    required String category,
    required String priority,
    String? subject,
    String? description,
    String? linkedOrderId,
  }) async {
    return _executeRequest<SupportTicketDto>(
      request: () => _apiClient.post(
        '$_basePath/tickets',
        data: {
          'category': category,
          'priority': priority,
          if (subject != null && subject.isNotEmpty) 'subject': subject,
          if (description != null && description.isNotEmpty)
            'description': description,
          if (linkedOrderId != null && linkedOrderId.isNotEmpty)
            'linked_order_id': linkedOrderId,
        },
      ),
      parser: (data) {
        final map = data as Map<String, dynamic>;
        return SupportTicketDto.fromMap(map['id'] as String? ?? '', map);
      },
    );
  }

  /// Get ticket by ID via Go API
  ///
  /// GET /api/v1/support/tickets/{ticketId}
  Future<ApiResult<SupportTicketDto?>> getTicket(String ticketId) async {
    return _executeRequest<SupportTicketDto?>(
      request: () => _apiClient.get('$_basePath/tickets/$ticketId'),
      parser: (data) {
        if (data == null) return null;
        final map = data as Map<String, dynamic>;
        return SupportTicketDto.fromMap(map['id'] as String? ?? '', map);
      },
    );
  }

  /// List the authenticated user's own tickets via the Support API.
  ///
  /// GET /api/v1/support/tickets
  ///
  /// The Support API is the sole identity authority for the ticket list — the
  /// chat room list must NOT be used to discover Support tickets.
  Future<ApiResult<List<SupportTicketDto>>> getMyTickets({
    int limit = 50,
  }) async {
    return _executeRequest<List<SupportTicketDto>>(
      request: () => _apiClient.get(
        '$_basePath/tickets',
        queryParameters: {'limit': limit},
      ),
      parser: (data) {
        return _extractList(data).map((e) {
          final map = e as Map<String, dynamic>;
          return SupportTicketDto.fromMap(map['id'] as String? ?? '', map);
        }).toList();
      },
    );
  }

  // REMOVED: getTickets() - Admin-only endpoint
  // REMOVED: getUnassignedTicketsCount() - Admin-only endpoint
  // REMOVED: getOpenTicketsCount() - Admin-only endpoint
  // REMOVED: getMyTicketsCount() - Admin-only endpoint

  // ============================================================
  // TICKET UPDATE OPERATIONS (Go API)
  // ============================================================

  // REMOVED: claimTicket() - Admin-only endpoint
  // REMOVED: resolveTicket() - Admin-only endpoint

  /// Reopen ticket via Go API (User-only)
  ///
  /// PUT /api/v1/support/tickets/{ticketId}/reopen
  Future<ApiResult<void>> reopenTicket({
    required String ticketId,
    required String userId,
  }) async {
    return _executeVoidRequest(
      request: () => _apiClient.put(
        '$_basePath/tickets/$ticketId/reopen',
        data: {'userId': userId},
      ),
    );
  }

  // REMOVED: closeTicket() - Admin-only endpoint
  // REMOVED: updateTicketPriority() - Admin-only endpoint
  // REMOVED: updateTicketCategory() - Admin-only endpoint

  // REMOVED: sendGreetingMessage() - Admin-only endpoint
  // REMOVED: sendSystemMessage() - Admin-only endpoint

  // REMOVED: All ADMIN OPERATIONS section - Admin-only endpoints
  // - getStatistics()
  // - getSupportAdminIds()
  // - notifyAdminsAboutNewTicket()

  /// Get ticket events via Go API (Read-only for users)
  ///
  /// GET /api/v1/support/tickets/{ticketId}/events
  Future<ApiResult<List<SupportEventDto>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async {
    return _executeRequest<List<SupportEventDto>>(
      request: () => _apiClient.get(
        '$_basePath/tickets/$ticketId/events',
        queryParameters: {'limit': limit},
      ),
      parser: (data) {
        return _extractList(data).map((e) {
          final map = e as Map<String, dynamic>;
          return SupportEventDto.fromMap(map);
        }).toList();
      },
    );
  }

  /// Get ticket messages via Go API (Read-only for users)
  ///
  /// GET /api/v1/support/tickets/{ticketId}/messages
  Future<ApiResult<List<SupportMessageDto>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) async {
    return _executeRequest<List<SupportMessageDto>>(
      request: () => _apiClient.get(
        '$_basePath/tickets/$ticketId/messages',
        queryParameters: {'limit': limit},
      ),
      parser: (data) {
        return _extractList(data).map((e) {
          final map = e as Map<String, dynamic>;
          return SupportMessageDto.fromMap(map);
        }).toList();
      },
    );
  }

  /// Send a reply into the authenticated user's own ticket conversation.
  ///
  /// POST /api/v1/support/tickets/{ticketId}/messages
  ///
  /// The request body carries ONLY the message text. The backend derives the
  /// sender from the authenticated session, so the client never supplies a
  /// sender identity (and one would be ignored server-side anyway).
  Future<ApiResult<void>> sendMessage({
    required String ticketId,
    required String message,
  }) async {
    return _executeVoidRequest(
      request: () => _apiClient.post(
        '$_basePath/tickets/$ticketId/messages',
        data: {'message': message},
      ),
    );
  }
}

// ============================================================
// API RESULT TYPE
// ============================================================

/// Result type for API operations
class ApiResult<T> {
  final T? data;
  final String? error;
  final String? code;

  const ApiResult._({this.data, this.error, this.code});

  factory ApiResult.success(T data) => ApiResult._(data: data);

  factory ApiResult.error(String error, {String? code}) =>
      ApiResult._(error: error, code: code);

  bool get isSuccess => error == null;
  bool get isError => error != null;

  R fold<R>({
    required R Function(T data) onSuccess,
    required R Function(String error, String? code) onError,
  }) {
    if (isSuccess) {
      return onSuccess(data as T);
    }
    return onError(error!, code);
  }
}
