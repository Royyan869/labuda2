import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show debugPrint;
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/api/models/api_response.dart';
import 'package:labuda/core/api/models/common_api_models.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

/// Base class for all API repositories
///
/// Provides common functionality:
/// - HTTP method wrappers with `Result<T>` return type
/// - Automatic error handling and conversion
/// - Response parsing helpers
/// - Logging
///
/// Usage:
/// ```dart
/// class UserRepositoryApi extends BaseApiRepository {
///   UserRepositoryApi(super.apiClient, {super.logger});
///
///   Future<Result<User>> getProfile() async {
///     return executeRequest(
///       () => apiClient.get('/users/me'),
///       parser: (data) => User.fromJson(data),
///     );
///   }
/// }
/// ```
abstract class BaseApiRepository {
  final ApiClient apiClient;
  final ILoggerService? logger;

  BaseApiRepository(this.apiClient, {this.logger});

  /// Execute a request and return `Result<T>`
  ///
  /// Handles:
  /// - Making the HTTP request
  /// - Parsing response data
  /// - Converting errors to Result.error
  Future<Result<T>> executeRequest<T>(
    Future<Response<dynamic>> Function() request, {
    required T Function(dynamic data) parser,
  }) async {
    try {
      final response = await request();
      final data = response.data;

      // 🌍 Log response status code (2xx = success)
      final statusCode = response.statusCode;
      final isSuccess =
          statusCode != null && statusCode >= 200 && statusCode < 300;

      logger?.log(
        '[API] statusCode=$statusCode, isSuccess=$isSuccess',
        level: LogLevel.debug,
      );
      logger?.log(
        '[API] response.data type=${data.runtimeType}',
        level: LogLevel.debug,
      );

      // 🔒 DEFENSIVE: Handle different response data types safely
      if (data == null) {
        logger?.log('[API] Response data is null', level: LogLevel.warning);
        return Result.error('Empty response from server');
      }

      // Handle standard API response format (Map with success/error/data structure)
      if (data is Map<String, dynamic>) {
        final apiResponse = ApiResponse.fromJson(data, null);

        if (!apiResponse.success && apiResponse.error != null) {
          logger?.error('[API] API error - ${apiResponse.error!.message}');
          return Result.error(
            apiResponse.error!.message,
            code: apiResponse.error!.code,
            statusCode: statusCode,
          );
        }

        // Envelope honesty: success=true with data=null is a contract
        // violation. We must NOT silently fall back to passing the whole
        // envelope into the parser — that hides backend-side errors as
        // generic "Invalid response data format" downstream.
        if (apiResponse.success && apiResponse.data == null) {
          logger?.warning(
            '[API] Envelope violation: success=true but data=null',
          );
          return Result.error(
            'Empty response data',
            code: 'EMPTY_DATA',
            statusCode: statusCode,
          );
        }

        // Legacy raw-map path: backend that does not return an envelope
        // (no `success` key) is treated as success=false / data=null by
        // ApiResponse.fromJson — we still let the parser see the raw map
        // for backward compatibility with non-enveloped endpoints.
        final dataToParse = apiResponse.data ?? data;
        try {
          final parsedData = parser(dataToParse);
          logger?.log(
            '[API] SUCCESS - parsed ${parsedData.runtimeType}',
            level: LogLevel.debug,
          );
          return Result.success(parsedData);
        } catch (parseError, parseStack) {
          // DIAGNOSTIC: always-visible parse error probe (dev only)
          final parseKeys = dataToParse is Map
              ? dataToParse.keys.toList()
              : 'N/A';
          debugPrint(
            '[PARSE_ERROR] type=${parseError.runtimeType} msg=$parseError\n'
            '[PARSE_ERROR] dataType=${dataToParse.runtimeType} keys=$parseKeys\n'
            '[PARSE_ERROR] stack=${parseStack.toString().split('\n').take(5).join(' | ')}',
          );
          logger?.error('[API] Parse error - $parseError');
          await logger?.error(
            'Failed to parse response data',
            extra: {
              'error': parseError.toString(),
              'statusCode': statusCode?.toString() ?? 'null',
              'backendCode': apiResponse.error?.code ?? 'none',
            },
          );
          return Result.error(
            'Invalid response data format',
            code: apiResponse.error?.code,
            statusCode: statusCode,
          );
        }
      }

      // Handle List response (not wrapped in standard format)
      if (data is List) {
        try {
          final parsedData = parser(data);
          logger?.log('[API] SUCCESS - parsed list', level: LogLevel.debug);
          return Result.success(parsedData);
        } catch (parseError) {
          logger?.error('[API] List parse error - $parseError');
          await logger?.error(
            'Failed to parse list response',
            extra: {'error': parseError.toString()},
          );
          return Result.error('Invalid list response format');
        }
      }

      // Handle primitive types (string, number, bool)
      if (data is String || data is num || data is bool) {
        try {
          final parsedData = parser(data);
          logger?.log(
            '[API] SUCCESS - parsed primitive',
            level: LogLevel.debug,
          );
          return Result.success(parsedData);
        } catch (parseError) {
          logger?.error('[API] Primitive parse error - $parseError');
          return Result.error('Invalid response format');
        }
      }

      // Unknown data type - log and return error
      logger?.warning('[API] Unknown data type - ${data.runtimeType}');
      await logger?.error(
        'Unknown response data type',
        extra: {'type': data.runtimeType.toString()},
      );
      return Result.error('Unknown response format: ${data.runtimeType}');
    } on DioException catch (e) {
      logger?.error('[API] DioException - ${e.message}');
      logger?.log('[API] DioException type - ${e.type}', level: LogLevel.debug);
      logger?.log(
        '[API] DioException response - ${e.response?.statusCode}',
        level: LogLevel.debug,
      );

      final exception = apiClient.extractException(e);
      // 🔍 DEBUG: Log actual exception details
      await logger?.error(
        'API Exception: ${exception.message}',
        extra: {
          'code': exception.code,
          'statusCode': exception.statusCode,
          'details': exception.details?.toString(),
          'type': exception.runtimeType.toString(),
        },
      );
      return Result.error(
        exception.message,
        code: exception.code,
        details: exception.details is Map<String, dynamic>
            ? exception.details as Map<String, dynamic>
            : null,
      );
    } catch (e, stackTrace) {
      logger?.error('[API] Unexpected exception - $e');
      await logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      return Result.error('An unexpected error occurred');
    }
  }

  /// Execute request that returns a list
  Future<Result<List<T>>> executeListRequest<T>(
    Future<Response<dynamic>> Function() request, {
    required T Function(Map<String, dynamic> json) itemParser,
  }) async {
    try {
      final response = await request();
      final data = response.data;
      final statusCode = response.statusCode;

      if (data is Map<String, dynamic>) {
        final apiResponse = ApiResponse.fromJson(data, null);

        if (!apiResponse.success && apiResponse.error != null) {
          return Result.error(
            apiResponse.error!.message,
            code: apiResponse.error!.code,
            statusCode: statusCode,
          );
        }

        // Envelope honesty: success=true with data=null is a contract
        // violation. An empty list (data: []) is a legitimate empty page
        // and is NOT flagged here.
        if (apiResponse.success && apiResponse.data == null) {
          logger?.warning(
            '[API] Envelope violation (list): success=true but data=null',
          );
          return Result.error(
            'Empty response data',
            code: 'EMPTY_DATA',
            statusCode: statusCode,
          );
        }

        final listData = apiResponse.data ?? data['data'];
        if (listData is List) {
          try {
            final items = listData
                .map((e) => itemParser(e as Map<String, dynamic>))
                .toList();
            return Result.success(items);
          } catch (parseError) {
            logger?.error('[API] List parse error - $parseError');
            await logger?.error(
              'Failed to parse list response',
              extra: {
                'error': parseError.toString(),
                'statusCode': statusCode?.toString() ?? 'null',
                'backendCode': apiResponse.error?.code ?? 'none',
              },
            );
            return Result.error(
              'Invalid list response format',
              code: apiResponse.error?.code,
              statusCode: statusCode,
            );
          }
        }
      }

      if (data is List) {
        try {
          final items = data
              .map((e) => itemParser(e as Map<String, dynamic>))
              .toList();
          return Result.success(items);
        } catch (parseError) {
          logger?.error('[API] Raw list parse error - $parseError');
          return Result.error(
            'Invalid list response format',
            statusCode: statusCode,
          );
        }
      }

      return Result.error('Invalid response format', statusCode: statusCode);
    } on DioException catch (e) {
      final exception = apiClient.extractException(e);
      return Result.error(
        exception.message,
        code: exception.code,
        statusCode: exception.statusCode,
      );
    } catch (e, stackTrace) {
      logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      return Result.error('An unexpected error occurred');
    }
  }

  /// Execute paginated request returning PaginatedApiResponse
  ///
  /// Backend returns:
  /// ```json
  /// {
  ///   "success": true,
  ///   "data": [...],
  ///   "meta": {
  ///     "page": 1,
  ///     "per_page": 20,
  ///     "total": 100,
  ///     "total_pages": 5
  ///   }
  /// }
  /// ```
  Future<Result<PaginatedApiResponse<T>>> executePaginatedRequest<T>(
    Future<Response<dynamic>> Function() request, {
    required T Function(dynamic) itemParser,
  }) async {
    try {
      final response = await request();
      final data = response.data;
      final statusCode = response.statusCode;

      if (data is Map<String, dynamic>) {
        final apiResponse = ApiResponse.fromJson(data, null);

        if (!apiResponse.success && apiResponse.error != null) {
          return Result.error(
            apiResponse.error!.message,
            code: apiResponse.error!.code,
            statusCode: statusCode,
          );
        }

        // Envelope honesty: success=true with data=null is a contract
        // violation for paginated responses too. An empty page (data: [])
        // is legitimate and is NOT flagged here.
        if (apiResponse.success && apiResponse.data == null) {
          logger?.warning(
            '[API] Envelope violation (paginated): success=true but data=null',
          );
          return Result.error(
            'Empty response data',
            code: 'EMPTY_DATA',
            statusCode: statusCode,
          );
        }

        try {
          final paginatedData = PaginatedApiResponse.fromJson(data, itemParser);
          return Result.success(paginatedData);
        } catch (parseError) {
          logger?.error('[API] Paginated parse error - $parseError');
          await logger?.error(
            'Failed to parse paginated response',
            extra: {
              'error': parseError.toString(),
              'statusCode': statusCode?.toString() ?? 'null',
              'backendCode': apiResponse.error?.code ?? 'none',
            },
          );
          return Result.error(
            'Invalid response data format',
            code: apiResponse.error?.code,
            statusCode: statusCode,
          );
        }
      }

      return Result.error('Invalid response format', statusCode: statusCode);
    } on DioException catch (e) {
      final exception = apiClient.extractException(e);
      logger?.error(
        'Paginated request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      return Result.error(
        exception.message,
        code: exception.code,
        statusCode: exception.statusCode,
      );
    } catch (e, stackTrace) {
      logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      return Result.error('An unexpected error occurred');
    }
  }

  /// Execute request that returns void (no data expected)
  ///
  /// Note: void requests do NOT trigger the EMPTY_DATA envelope check —
  /// `data: null` is the canonical shape for a void success.
  Future<Result<void>> executeVoidRequest(
    Future<Response<dynamic>> Function() request,
  ) async {
    try {
      final response = await request();
      final data = response.data;
      final statusCode = response.statusCode;

      // Check for error in response
      if (data is Map<String, dynamic>) {
        final success = data['success'] as bool? ?? true;
        if (!success && data['error'] != null) {
          final error = ApiError.fromJson(data['error']);
          return Result.error(
            error.message,
            code: error.code,
            statusCode: statusCode,
          );
        }
      }

      return Result.success(null);
    } on DioException catch (e) {
      final exception = apiClient.extractException(e);
      return Result.error(
        exception.message,
        code: exception.code,
        statusCode: exception.statusCode,
      );
    } catch (e, stackTrace) {
      logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      return Result.error('An unexpected error occurred');
    }
  }

  /// Get typed exception from DioException
  ApiException getException(DioException e) => apiClient.extractException(e);

  /// Helper to build query parameters for pagination
  ///
  /// Uses `per_page` to match backend convention (not `limit`)
  Map<String, dynamic> paginationParams({
    int page = 1,
    int perPage = 20,
    String? sortBy,
    String? sortOrder,
  }) {
    return {
      'page': page,
      'per_page': perPage,
      'sort_by': ?sortBy,
      'sort_order': ?sortOrder,
    };
  }
}
