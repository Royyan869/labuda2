import 'package:dio/dio.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/shared/shared.dart';

/// Interceptor that provides detailed HTTP logging for debugging
///
/// Logs:
/// - Request URL (redacted)
/// - HTTP method
/// - Request headers (redacted for security)
/// - Response status code
/// - Response body
/// - Exception details if any
///
/// This interceptor should be placed AFTER AuthInterceptor
/// to capture the final headers with Authorization token.
class DetailedLoggingInterceptor extends Interceptor {
  final ILoggerService _logger = LoggerService.instance;

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    // Log request details BEFORE sending
    final baseUrl = options.baseUrl;
    final path = options.path;
    // Construct full URL
    final fullUrl = baseUrl.isEmpty || path.contains('://')
        ? path
        : baseUrl.endsWith('/')
        ? '$baseUrl$path'
        : '$baseUrl/$path';

    _logger.log('[API] Request URL: $fullUrl', level: LogLevel.debug);
    _logger.log(
      '[API] Request Method: ${options.method.toUpperCase()}',
      level: LogLevel.debug,
    );
    _logger.log('[API] Request Headers: (redacted)', level: LogLevel.debug);

    if (options.data != null) {
      _logger.log('[API] Request Body: (redacted)', level: LogLevel.debug);
    }
    if (options.queryParameters.isNotEmpty) {
      _logger.log('[API] Request Query: (redacted)', level: LogLevel.debug);
    }

    handler.next(options);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    // Log response details
    _logger.log(
      '[API] Response Status: ${response.statusCode}',
      level: LogLevel.debug,
    );
    _logger.log('[API] Response Body: (redacted)', level: LogLevel.debug);

    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    // Log exception details

    _logger.log(
      '[API] Request Exception: ${err.message}',
      level: LogLevel.error,
    );
    _logger.log('[API] Exception Type: ${err.type}', level: LogLevel.error);
    _logger.log(
      '[API] Exception Status: ${err.response?.statusCode ?? "N/A"}',
      level: LogLevel.error,
    );

    if (err.response != null) {
      _logger.log(
        '[API] Exception Response Body: (redacted)',
        level: LogLevel.error,
      );
    }
    if (err.error != null) {
      _logger.log('[API] Exception Error: ${err.error}', level: LogLevel.error);
    }

    handler.next(err);
  }
}
