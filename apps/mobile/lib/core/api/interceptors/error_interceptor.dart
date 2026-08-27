import 'dart:io';

import 'package:dio/dio.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

/// Interceptor that converts Dio errors to typed ApiExceptions
///
/// Handles:
/// - HTTP error responses (4xx, 5xx)
/// - Network errors (no internet, timeout)
/// - Request cancellation
/// - Parsing API error response format
class ErrorInterceptor extends Interceptor {
  final ILoggerService? _logger;

  ErrorInterceptor({ILoggerService? logger}) : _logger = logger;

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final apiException = _convertToApiException(err);

    _logger?.error(
      'API Error: ${apiException.message}',
      extra: {
        'statusCode': apiException.statusCode,
        'code': apiException.code,
        'path': err.requestOptions.path,
        'method': err.requestOptions.method,
      },
    );

    // Wrap ApiException in DioException to propagate through Dio
    handler.next(
      DioException(
        requestOptions: err.requestOptions,
        response: err.response,
        type: err.type,
        error: apiException,
      ),
    );
  }

  /// Convert DioException to typed ApiException
  ApiException _convertToApiException(DioException err) {
    switch (err.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return const TimeoutException(
          message: 'Connection timed out. Please try again.',
        );

      case DioExceptionType.cancel:
        return const CancelledException();

      case DioExceptionType.connectionError:
        // A connectionError means the socket/handshake to the configured
        // backend host failed (refused, unreachable, wrong IP, backend down)
        // -- this is distinct from the device having no network at all, and
        // reporting it as "no internet" misleads a user whose WiFi/data is
        // fine but whose backend is unreachable. Keep the word "network" in
        // the message so AuthController._isBackendUnavailableError (substring
        // match) still classifies this as AuthState.backendUnavailable.
        return const NetworkException(
          message:
              'Cannot reach Labuda server. Check that the backend is running and the device is on the same network.',
          code: 'BACKEND_UNREACHABLE',
        );

      case DioExceptionType.badCertificate:
        return const NetworkException(
          message: 'SSL certificate error. Please try again later.',
          code: 'SSL_ERROR',
        );

      case DioExceptionType.badResponse:
        return _parseErrorResponse(err.response);

      case DioExceptionType.unknown:
        if (err.error is SocketException) {
          return const NetworkException(
            message: 'Network error. Please check your connection.',
          );
        }
        return UnknownApiException(
          message: err.message ?? 'An unexpected error occurred',
          details: err.error,
        );
    }
  }

  /// Parse error from API response
  ApiException _parseErrorResponse(Response? response) {
    if (response == null) {
      return const UnknownApiException(message: 'No response from server');
    }

    final statusCode = response.statusCode ?? 500;
    final data = response.data;

    // Try to parse structured error response from Go backend
    // Expected format: { "success": false, "error": { "code": "...", "message": "..." } }
    String message = 'An error occurred';
    String? code;
    dynamic details;
    Map<String, List<String>>? fieldErrors;
    int? retryAfterSeconds;

    if (data is Map<String, dynamic>) {
      // Check for error object
      final error = data['error'];
      if (error is Map<String, dynamic>) {
        message = error['message'] as String? ?? message;
        code = error['code'] as String?;
        details = error['details'];

        // Parse field errors for validation
        if (error['field_errors'] is Map) {
          fieldErrors = _parseFieldErrors(error['field_errors']);
        }
      } else if (data['message'] is String) {
        // Simple message format
        message = data['message'] as String;
      }

      // Check for rate limit retry-after
      if (statusCode == 429) {
        retryAfterSeconds = data['retry_after'] as int?;
      }
    } else if (data is String && data.isNotEmpty) {
      message = data;
    }

    return ApiExceptionFactory.fromStatusCode(
      statusCode,
      message,
      code: code,
      details: details,
      fieldErrors: fieldErrors,
      retryAfterSeconds: retryAfterSeconds,
    );
  }

  /// Parse field-level validation errors
  Map<String, List<String>>? _parseFieldErrors(dynamic fieldErrors) {
    if (fieldErrors is! Map) return null;

    final result = <String, List<String>>{};

    fieldErrors.forEach((key, value) {
      if (key is String) {
        if (value is List) {
          result[key] = value.map((e) => e.toString()).toList();
        } else if (value is String) {
          result[key] = [value];
        }
      }
    });

    return result.isEmpty ? null : result;
  }
}
