import 'package:dio/dio.dart';
import 'package:labuda/core/api/config/api_config.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/api/interceptors/auth_interceptor.dart';
import 'package:labuda/core/api/interceptors/detailed_logging_interceptor.dart';
import 'package:labuda/core/api/interceptors/error_interceptor.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

// Conditional import for Platform detection
// Import platform_io by default, use platform_web for web
import 'package:labuda/core/api/platform/platform_io.dart'
    if (dart.library.html) 'package:labuda/core/api/platform/platform_web.dart';

/// Central HTTP client for all API calls to Go backend
///
/// Features:
/// - Automatic Firebase token attachment (via AuthInterceptor)
/// - Error handling and conversion to ApiException
/// - Request/response logging (dev only)
/// - Configurable timeouts
/// - Platform-aware base URL
/// - 401 auto-retry with token refresh
class ApiClient {
  late final Dio _dio;
  final ILoggerService? _logger;

  ApiClient({ILoggerService? logger, String? baseUrl}) : _logger = logger {
    _dio = _createDio(baseUrl);
  }

  /// Create and configure Dio instance
  Dio _createDio(String? baseUrl) {
    final dio = Dio(
      BaseOptions(
        baseUrl: baseUrl ?? _getPlatformBaseUrl(),
        connectTimeout: Duration(milliseconds: ApiConfig.connectTimeout),
        receiveTimeout: Duration(milliseconds: ApiConfig.receiveTimeout),
        sendTimeout: Duration(milliseconds: ApiConfig.sendTimeout),
        headers: ApiConfig.defaultHeaders,
        // 🔥 FIX: Disable connectivity check for emulator
        validateStatus: (status) => status != null && status < 500,
      ),
    );

    // Add interceptors in order
    dio.interceptors.addAll([
      // Auth interceptor - adds Firebase token directly from FirebaseAuth
      AuthInterceptor(logger: _logger),
      // Detailed logging - logs request/response with headers AFTER auth interceptor
      DetailedLoggingInterceptor(),
      // Error interceptor - converts to ApiException
      ErrorInterceptor(logger: _logger),
      // Logging interceptor (dev only)
      if (ApiConfig.enableLogging)
        LogInterceptor(
          requestBody: true,
          responseBody: true,
          error: true,
          logPrint: (obj) => _logger?.debug(obj.toString()),
        ),
    ]);

    return dio;
  }

  /// Get platform-appropriate base URL
  String _getPlatformBaseUrl() {
    return ApiConfig.getBaseUrl(isIOS: platformDetector.isIOS);
  }

  // ============ HTTP Methods ============

  /// GET request
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return _dio.get<T>(
      path,
      queryParameters: queryParameters,
      options: options,
      cancelToken: cancelToken,
    );
  }

  /// POST request
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return _dio.post<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      options: options,
      cancelToken: cancelToken,
    );
  }

  /// PUT request
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return _dio.put<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      options: options,
      cancelToken: cancelToken,
    );
  }

  /// PATCH request
  Future<Response<T>> patch<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return _dio.patch<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      options: options,
      cancelToken: cancelToken,
    );
  }

  /// DELETE request
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return _dio.delete<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      options: options,
      cancelToken: cancelToken,
    );
  }

  /// Upload file with multipart form data
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) async {
    final formData = FormData.fromMap({
      fieldName: await MultipartFile.fromFile(filePath),
      if (additionalFields != null) ...additionalFields,
    });

    return _dio.post<T>(
      path,
      data: formData,
      options: options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
    );
  }

  // ============ Helper Methods ============

  /// Extract ApiException from DioException
  ///
  /// Use this in repositories to get typed exceptions:
  /// ```dart
  /// try {
  ///   final response = await apiClient.get('/users');
  ///   return Result.success(User.fromJson(response.data));
  /// } on DioException catch (e) {
  ///   final apiException = apiClient.extractException(e);
  ///   return Result.error(apiException.message);
  /// }
  /// ```
  ApiException extractException(DioException e) {
    if (e.error is ApiException) {
      return e.error as ApiException;
    }
    return UnknownApiException(
      message: e.message ?? 'Unknown error occurred',
      details: e.error,
    );
  }

  /// Check if exception is a specific type
  bool isUnauthorized(DioException e) {
    return extractException(e) is UnauthorizedException;
  }

  bool isNotFound(DioException e) {
    return extractException(e) is NotFoundException;
  }

  bool isValidationError(DioException e) {
    return extractException(e) is ValidationException;
  }

  /// Get underlying Dio instance (for advanced usage)
  Dio get dio => _dio;
}
