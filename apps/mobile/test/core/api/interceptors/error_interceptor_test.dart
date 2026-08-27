// PASS 1C — ErrorInterceptor backend-unreachable message classification.
//
// Verifies that a Dio connectionError (the case that fires when the socket
// to the configured backend host fails — refused, unreachable, wrong IP,
// backend down) is converted to a message that says "Cannot reach Labuda
// server" rather than "No internet connection", while still containing the
// word "network" so AuthController._isBackendUnavailableError (a substring
// match on the error string) keeps classifying it as
// AuthState.backendUnavailable instead of AuthState.backendFailure.
//
// Follows the same real-Dio-plus-fake-adapter pattern as
// auth_interceptor_session_expiry_test.dart rather than hand-constructing
// ErrorInterceptorHandler, since that type isn't meant to be built directly
// in tests.

import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/api/interceptors/error_interceptor.dart';

/// Adapter that fails every request with a given DioException, simulating
/// the socket-level failure Dio itself reports as connectionError.
class _FailingAdapter implements HttpClientAdapter {
  _FailingAdapter(this.exceptionBuilder);

  final DioException Function(RequestOptions options) exceptionBuilder;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    throw exceptionBuilder(options);
  }

  @override
  void close({bool force = false}) {}
}

Dio _buildDio(DioException Function(RequestOptions options) exceptionBuilder) {
  final dio = Dio(BaseOptions(baseUrl: 'http://192.168.1.7:8080'));
  dio.httpClientAdapter = _FailingAdapter(exceptionBuilder);
  dio.interceptors.add(ErrorInterceptor());
  return dio;
}

void main() {
  group('ErrorInterceptor connectionError classification', () {
    test(
      'reports backend-unreachable message, not "No internet connection"',
      () async {
        final dio = _buildDio(
          (options) => DioException(
            requestOptions: options,
            type: DioExceptionType.connectionError,
            error: const SocketException('Connection refused'),
          ),
        );

        late final ApiException apiException;
        try {
          await dio.get('/api/v1/health');
          fail('expected a DioException to be thrown');
        } on DioException catch (e) {
          expect(e.error, isA<ApiException>());
          apiException = e.error as ApiException;
        }

        expect(apiException, isA<NetworkException>());
        expect(apiException.message, contains('Cannot reach Labuda server'));
        expect(apiException.message, isNot(contains('No internet connection')));
        expect(apiException.code, 'BACKEND_UNREACHABLE');
      },
    );

    test(
      'message still contains "network" so AuthController keeps classifying '
      'it as backend-unavailable (substring match), not a hard failure',
      () async {
        final dio = _buildDio(
          (options) => DioException(
            requestOptions: options,
            type: DioExceptionType.connectionError,
            error: const SocketException('Connection refused'),
          ),
        );

        try {
          await dio.get('/api/v1/health');
          fail('expected a DioException to be thrown');
        } on DioException catch (e) {
          final apiException = e.error as ApiException;
          // AuthController._isBackendUnavailableError lowercases and does a
          // plain `.contains('network')` check on the exception's string
          // form. This must keep matching after the copy change.
          expect(
            apiException.message.toLowerCase(),
            contains('network'),
            reason:
                'AuthController._isBackendUnavailableError classifies by '
                'substring match on "network"/"connection" — losing this '
                'word would silently regress backendUnavailable handling',
          );
        }
      },
    );

    test(
      'other error classifications are unchanged (regression guard)',
      () async {
        final dio = _buildDio(
          (options) => DioException(
            requestOptions: options,
            type: DioExceptionType.connectionTimeout,
          ),
        );

        try {
          await dio.get('/api/v1/health');
          fail('expected a DioException to be thrown');
        } on DioException catch (e) {
          final apiException = e.error as ApiException;
          expect(apiException, isA<TimeoutException>());
          expect(apiException.message, contains('Connection timed out'));
        }
      },
    );
  });
}
