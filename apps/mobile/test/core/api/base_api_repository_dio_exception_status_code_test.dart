// AUTH CLEANUP SLICE 24 — executeRequest DioException statusCode propagation.
//
// Slice 23 proved that `BaseApiRepository.executeRequest`'s DioException branch
// was the ONLY error branch in the file that dropped `statusCode`, while its
// siblings (executeListRequest / executePaginatedRequest / executeVoidRequest)
// all forward `exception.statusCode`. `Result.error`'s own contract states that
// `statusCode` preserves the HTTP status "when available".
//
// These tests cross the real DioException path:
//
//   DioException → ApiClient.extractException → executeRequest → Result.error
//
// The ApiClient stub mirrors production `extractException` (api_client.dart:195)
// and the attached ApiException is built with the production
// `ApiExceptionFactory.fromStatusCode`, so the statusCode under test comes from
// the HTTP status exactly as ErrorInterceptor produces it at runtime.
//
// Scope note: this file covers `executeRequest` only. Sibling methods are NOT
// changed by Slice 24 and are intentionally not covered here.

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/base_api_repository.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/common/result.dart';

/// Mirrors production `ApiClient.extractException`: ErrorInterceptor attaches
/// the typed ApiException to `DioException.error`, and ApiClient hands it back.
class _InterceptorLikeApiClient implements ApiClient {
  @override
  ApiException extractException(DioException e) {
    final error = e.error;
    return error is ApiException
        ? error
        : UnknownApiException(message: e.message ?? 'unknown');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _TestRepo extends BaseApiRepository {
  _TestRepo(super.apiClient);
}

/// Builds the DioException shape that reaches BaseApiRepository in production:
/// a failing HTTP response whose `error` field is the interceptor-produced
/// ApiException.
DioException _badResponse(
  int statusCode, {
  required String code,
  required String message,
}) {
  return DioException(
    requestOptions: RequestOptions(path: '/test'),
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      requestOptions: RequestOptions(path: '/test'),
      statusCode: statusCode,
      data: <String, dynamic>{
        'success': false,
        'error': <String, dynamic>{'code': code, 'message': message},
        'timestamp': '2026-09-14T00:00:00Z',
      },
    ),
    error: ApiExceptionFactory.fromStatusCode(statusCode, message, code: code),
  );
}

void main() {
  final repo = _TestRepo(_InterceptorLikeApiClient());

  Future<Result<String>> callWith(DioException failure) {
    return repo.executeRequest<String>(
      () async => throw failure,
      parser: (data) => 'parsed',
    );
  }

  group('executeRequest DioException statusCode propagation', () {
    test('HTTP 500 → Result.statusCode == 500', () async {
      final result = await callWith(
        _badResponse(500, code: 'INTERNAL_SERVER_ERROR', message: 'Database error'),
      );

      expect(result.isError, isTrue);
      expect(result.error, equals('Database error'));
      expect(result.errorCode, equals('INTERNAL_SERVER_ERROR'));
      expect(result.statusCode, equals(500));
    });

    test('HTTP 503 → Result.statusCode == 503', () async {
      final result = await callWith(
        _badResponse(
          503,
          code: 'FEATURE_DISABLED',
          message: 'Feature is currently disabled',
        ),
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, equals('FEATURE_DISABLED'));
      expect(result.statusCode, equals(503));
    });

    test('HTTP 401 → Result.statusCode == 401', () async {
      final result = await callWith(
        _badResponse(401, code: 'INVALID_TOKEN', message: 'Invalid or expired token'),
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, equals('INVALID_TOKEN'));
      expect(result.statusCode, equals(401));
    });

    test(
      'statusCode is null when the failure has no HTTP response (transport)',
      () async {
        // Timeout path: ErrorInterceptor maps it to TimeoutException(statusCode
        // null), so Result.statusCode must stay null — propagation must not
        // invent a status.
        final result = await callWith(
          DioException(
            requestOptions: RequestOptions(path: '/test'),
            type: DioExceptionType.connectionTimeout,
            error: const TimeoutException(
              message: 'Connection timed out. Please try again.',
            ),
          ),
        );

        expect(result.isError, isTrue);
        expect(result.errorCode, equals('TIMEOUT'));
        expect(result.statusCode, isNull);
      },
    );
  });
}
