/// Real HTTP-level integration test for COMMERCE_RESTRICTED propagation.
///
/// This test proves the full ErrorInterceptor parsing pipeline by:
/// 1. Constructing real DioException + Response objects with exact backend format
/// 2. Feeding them through the real ErrorInterceptor.onError
/// 3. Verifying the resulting exception carries the correct error code
///
/// Flutter test framework blocks real network IO, so we exercise the
/// parsing path directly using the real `ErrorInterceptor._parseErrorResponse`
/// method (exposed via the interceptor's onError handler). This is the same
/// parsing code that handles real HTTP responses — it is NOT mocked.
///
/// To prove real network behavior, a full integration test on a device/emulator
/// with a running backend is required (see LIMITATIONS section at bottom).

import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_error_codes.dart' as codes;
import 'package:labuda/core/api/commerce_restriction_presenter.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/api/interceptors/error_interceptor.dart';

/// The exact backend JSON response format for COMMERCE_RESTRICTED.
const _commerceRestrictedBody = {
  'success': false,
  'error': {
    'code': 'COMMERCE_RESTRICTED',
    'message': 'Aktivitas commerce Anda saat ini dibatasi karena pelanggaran.',
  },
};

/// A different 403 response (non-commerce-restricted).
const _genericForbiddenBody = {
  'success': false,
  'error': {
    'code': 'FORBIDDEN',
    'message': 'You do not have permission to access this resource.',
  },
};

/// 400 validation error.
const _validationBody = {
  'success': false,
  'error': {
    'code': 'VALIDATION_ERROR',
    'message': 'Field required',
    'field_errors': {'email': ['Required']},
  },
};

/// Handler that captures the ApiException from ErrorInterceptor.onError
/// without triggering the internal Completer that causes async errors.
class _CapturingHandler extends ErrorInterceptorHandler {
  ApiException? capturedApiException;
  DioException? capturedDioException;

  @override
  void next(DioException err) {
    // The ErrorInterceptor wraps the ApiException in DioException.error
    capturedDioException = err;
    capturedApiException = err.error as ApiException?;
    // Do NOT call super.next() — that would complete the internal
    // Completer which causes unhandled async errors in tests.
  }
}

/// Helper: run ErrorInterceptor and return the ApiException.
ApiException _runInterceptor({
  required int statusCode,
  required Map<String, dynamic> body,
  String method = 'POST',
  String path = '/api/v1/commerce/checkout',
}) {
  final requestOptions = RequestOptions(path: path, method: method);
  final response = Response(
    requestOptions: requestOptions,
    statusCode: statusCode,
    data: body,
  );
  final dioException = DioException(
    requestOptions: requestOptions,
    response: response,
    type: DioExceptionType.badResponse,
  );

  final interceptor = ErrorInterceptor();
  final handler = _CapturingHandler();

  interceptor.onError(dioException, handler);

  expect(
    handler.capturedApiException,
    isNotNull,
    reason: 'ErrorInterceptor must produce an ApiException',
  );

  return handler.capturedApiException!;
}

void main() {
  group('ErrorInterceptor — COMMERCE_RESTRICTED parsing (real parsing path)', () {
    test(
      '403 COMMERCE_RESTRICTED → ForbiddenException with correct code and message',
      () {
        final apiException = _runInterceptor(
          statusCode: 403,
          body: _commerceRestrictedBody,
        );

        // Verify exact type
        expect(apiException, isA<ForbiddenException>());

        final forbidden = apiException as ForbiddenException;

        // Verify the EXACT backend error code survived parsing
        expect(forbidden.code, equals('COMMERCE_RESTRICTED'));
        expect(forbidden.statusCode, equals(403));
        expect(forbidden.message, contains('Aktivitas commerce'));

        // Verify the canonical presenter recognizes it
        expect(
          CommerceRestrictionPresenter.isCommerceRestricted(forbidden.code),
          isTrue,
        );

        // Verify the canonical constant matches
        expect(forbidden.code, equals(codes.commerceRestricted));
      },
    );

    test(
      '403 generic FORBIDDEN → ForbiddenException with different code — NOT commerce restriction',
      () {
        final apiException = _runInterceptor(
          statusCode: 403,
          body: _genericForbiddenBody,
        );

        expect(apiException, isA<ForbiddenException>());

        final forbidden = apiException as ForbiddenException;

        // Generic FORBIDDEN must NOT be treated as commerce restriction
        expect(forbidden.code, equals('FORBIDDEN'));
        expect(forbidden.code, isNot(equals('COMMERCE_RESTRICTED')));
        expect(
          CommerceRestrictionPresenter.isCommerceRestricted(forbidden.code),
          isFalse,
        );
      },
    );

    test(
      '400 Bad Request preserves error code from backend',
      () {
        final apiException = _runInterceptor(
          statusCode: 400,
          body: _validationBody,
        );

        // Backend 400 with VALIDATION_ERROR → BadRequestException (per factory)
        // The error code is preserved in the exception
        expect(apiException.code, equals('VALIDATION_ERROR'));
        expect(apiException.statusCode, equals(400));
        expect(apiException.message, equals('Field required'));
      },
    );

    test(
      '401 UNAUTHORIZED → UnauthorizedException',
      () {
        final apiException = _runInterceptor(
          statusCode: 401,
          body: {
            'success': false,
            'error': {
              'code': 'UNAUTHORIZED',
              'message': 'Invalid token',
            },
          },
        );

        expect(apiException, isA<UnauthorizedException>());
        expect(
          (apiException as UnauthorizedException).code,
          equals('UNAUTHORIZED'),
        );
      },
    );

    test(
      '500 SERVER_ERROR → ServerException',
      () {
        final apiException = _runInterceptor(
          statusCode: 500,
          body: {
            'success': false,
            'error': {
              'code': 'SERVER_ERROR',
              'message': 'Internal error',
            },
          },
        );

        expect(apiException, isA<ServerException>());
        expect(
          (apiException as ServerException).code,
          equals('SERVER_ERROR'),
        );
      },
    );
  });

  group('End-to-end flow simulation — COMMERCE_RESTRICTED through real parser', () {
    test('Auction bid path', () {
      final apiException = _runInterceptor(
        statusCode: 403,
        body: _commerceRestrictedBody,
        path: '/api/v1/auctions/123/bid',
      );

      expect(apiException, isA<ForbiddenException>());
      expect(
        (apiException as ForbiddenException).code,
        equals(codes.commerceRestricted),
      );
    });

    test('For-sale create path', () {
      final apiException = _runInterceptor(
        statusCode: 403,
        body: _commerceRestrictedBody,
        path: '/api/v1/for-sale',
      );

      expect(apiException, isA<ForbiddenException>());
      expect(
        (apiException as ForbiddenException).code,
        equals(codes.commerceRestricted),
      );
    });

    test('Promotion activate path', () {
      final apiException = _runInterceptor(
        statusCode: 403,
        body: _commerceRestrictedBody,
        path: '/api/v1/promotions/activate',
      );

      expect(apiException, isA<ForbiddenException>());
      expect(
        (apiException as ForbiddenException).code,
        equals(codes.commerceRestricted),
      );
    });

    test('Auction create path', () {
      final apiException = _runInterceptor(
        statusCode: 403,
        body: _commerceRestrictedBody,
        path: '/api/v1/auctions',
      );

      expect(apiException, isA<ForbiddenException>());
      expect(
        (apiException as ForbiddenException).code,
        equals(codes.commerceRestricted),
      );
    });
  });

  group('Full pipeline proof — from backend JSON to presenter', () {
    test(
      'Backend 403 JSON → ErrorInterceptor → ForbiddenException → '
      'CommerceRestrictionPresenter.isCommerceRestricted → true',
      () {
        // Step 1: Backend produces exact JSON (simulate JSON decode as real HTTP would)
        final backendJson = jsonEncode(_commerceRestrictedBody);
        final parsedBody = jsonDecode(backendJson) as Map<String, dynamic>;

        // Step 2: ErrorInterceptor parses the Response (real parsing path)
        final apiException = _runInterceptor(
          statusCode: 403,
          body: parsedBody,
        );

        // Step 3: Verify canonical presenter recognizes it
        expect(apiException, isA<ForbiddenException>());
        expect(
          CommerceRestrictionPresenter.isCommerceRestricted(
            apiException.code,
          ),
          isTrue,
        );

        // Step 4: Verify canonical constant matches
        expect(apiException.code, equals(codes.commerceRestricted));

        // Step 5: Verify message preserves backend content — NOT account suspension
        expect(apiException.message, contains('dibatasi'));
        expect(
          apiException.message.toLowerCase(),
          isNot(contains('diblokir')),
        );
        expect(
          apiException.message.toLowerCase(),
          isNot(contains('ditangguhkan')),
        );
      },
    );
  });
}

/// LIMITATIONS:
///
/// This test exercises the REAL ErrorInterceptor parsing path with REAL
/// JSON parsing and REAL exception construction. It proves that:
/// - The backend JSON envelope format is correctly parsed
/// - The error code survives into the ForbiddenException
/// - CommerceRestrictionPresenter recognizes the code
///
/// What this test CANNOT prove:
/// - Real HTTP transport (Flutter test framework blocks IO)
/// - Real Dio client connecting to a real backend
/// - Real Firebase authentication + restricted user
/// - Real UI rendering of the snackbar/presenter
///
/// For full runtime proof, a device-level integration test with a live backend
/// is required, which is BLOCKED by:
/// 1. No mobile device/emulator available (Windows desktop only)
/// 2. commerce_restrictions migration (000062) not yet applied to the DB
/// 3. No pre-existing restricted test user
