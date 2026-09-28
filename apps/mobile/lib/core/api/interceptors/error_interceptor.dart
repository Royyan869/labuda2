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
  ///
  /// Transport classification (`DioExceptionType` → canonical error code)
  /// lives in ONE place: [ApiExceptionFactory.fromTransport]. It returns null
  /// for `badResponse`, the only type that carries an HTTP envelope — that
  /// branch is parsed here because it needs the raw [Response].
  ApiException _convertToApiException(DioException err) {
    return ApiExceptionFactory.fromTransport(err) ??
        _parseErrorResponse(err.response);
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
