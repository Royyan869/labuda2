// Transport-failure classification contract: ONE table, ONE code identity.
//
// A request that never produced a usable HTTP envelope used to be described
// only by a human message: 'Network error. Please check your connection.',
// 'Connection timed out. Please try again.', 'SSL certificate error…'. Nothing
// machine-readable told them apart, so callers substring-matched the copy —
// `error.contains('network') || error.contains('timeout')` in the auth
// controller, the support mapper, the auction error scaffold, checkout's
// `code: 'NETWORK_ERROR'` fabrication, and a retry helper. Copy changed, and
// the classification changed with it: the connectionError message had to keep
// the word "network" purely so a substring match would keep working.
//
// Convergence: the API layer classifies `DioExceptionType` in exactly one
// table (`ApiExceptionFactory.fromTransport`), that table emits canonical
// codes from `lib/core/api/api_error_codes.dart`, and the code travels out
// through `Result.errorCode`. This gate locks it:
//  1. Every DioExceptionType reaches `Result.errorCode` with its canonical
//     code — proved through the REAL ApiClient + ErrorInterceptor + Dio.
//  2. The HTTP branch (badResponse) stays an HTTP branch: it is never reported
//     as a transport failure.
//  3. `case DioExceptionType.` appears in exactly one file under lib/.
//  4. No transport code literal exists outside the authority file.
//  5. The converged consumers classify by code, not by text.
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/api_error_codes.dart';
import 'package:labuda/core/api/base_api_repository.dart';
import 'package:labuda/core/common/result.dart';

/// The single home of the transport classification table.
const _classificationAuthority = 'lib/core/api/exceptions/api_exception.dart';

/// The single home of mobile error-code identity.
const _codeAuthority = 'lib/core/api/api_error_codes.dart';

/// Every transport code, with the DioExceptionType it must come from.
const _transportTable = <DioExceptionType, String>{
  DioExceptionType.connectionTimeout: requestTimeout,
  DioExceptionType.sendTimeout: requestTimeout,
  DioExceptionType.receiveTimeout: requestTimeout,
  DioExceptionType.transformTimeout: requestTimeout,
  DioExceptionType.connectionError: backendUnreachable,
  DioExceptionType.badCertificate: sslError,
};

List<File> _dartFiles(String root) => Directory(root)
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .toList();

String _rel(File file) => file.path.replaceAll(r'\', '/');

/// Adapter that either fails with a transport error or answers with a body —
/// the same real-Dio-plus-fake-adapter shape used by error_interceptor_test.
class _Adapter implements HttpClientAdapter {
  _Adapter(this.respond);

  final Future<ResponseBody> Function(RequestOptions options) respond;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) =>
      respond(options);

  @override
  void close({bool force = false}) {}
}

class _ProbeRepo extends BaseApiRepository {
  _ProbeRepo(super.apiClient);
}

/// Drives a failure through the REAL production path:
///
///   Dio adapter failure → ErrorInterceptor.onError → ApiException
///   → ApiClient.extractException → BaseApiRepository.executeRequest
///   → Result.errorCode
Future<Result<String>> _classifyThroughRealPipeline(
  Future<ResponseBody> Function(RequestOptions options) adapter,
) {
  final client = ApiClient(baseUrl: 'http://127.0.0.1:9');
  client.dio.httpClientAdapter = _Adapter(adapter);
  final repo = _ProbeRepo(client);
  return repo.executeRequest<String>(
    // skipAuth keeps the auth interceptor out of the way; the classification
    // path under test sits in ErrorInterceptor, further down the chain.
    () => client.get<dynamic>('/probe', options: Options(extra: {'skipAuth': true})),
    parser: (data) => 'parsed',
  );
}

Future<ResponseBody> Function(RequestOptions) _failWith(
  DioException Function(RequestOptions) build,
) {
  return (RequestOptions options) async => throw build(options);
}

void main() {
  group('DioExceptionType → canonical transport code → Result.errorCode', () {
    for (final entry in _transportTable.entries) {
      test('${entry.key.name} → ${entry.value}', () async {
        final result = await _classifyThroughRealPipeline(
          _failWith(
            (options) => DioException(requestOptions: options, type: entry.key),
          ),
        );

        expect(result.isError, isTrue);
        expect(
          result.errorCode,
          entry.value,
          reason:
              'a ${entry.key.name} must reach callers as its canonical code — '
              'the message is copy and callers must not match it',
        );
        expect(
          isTransportFailureCode(result.errorCode),
          isTrue,
          reason: '${entry.value} must be recognised as a transport failure',
        );
        // Transport failures never carried an HTTP status: inventing one would
        // make them indistinguishable from a real 5xx.
        expect(result.statusCode, isNull);
      });
    }

    test('unknown carrying a SocketException → $networkError', () async {
      final result = await _classifyThroughRealPipeline(
        _failWith(
          (options) => DioException(
            requestOptions: options,
            type: DioExceptionType.unknown,
            error: const SocketException('Failed host lookup'),
          ),
        ),
      );

      expect(result.errorCode, networkError);
      expect(isTransportFailureCode(result.errorCode), isTrue);
    });

    test('cancel → $requestCancelled', () async {
      final result = await _classifyThroughRealPipeline(
        _failWith(
          (options) => DioException(
            requestOptions: options,
            type: DioExceptionType.cancel,
          ),
        ),
      );

      expect(result.errorCode, requestCancelled);
      expect(isTransportFailureCode(result.errorCode), isTrue);
    });

    test('an unclassifiable failure is $unknownError and is NOT claimed as '
        'transport', () async {
      final result = await _classifyThroughRealPipeline(
        _failWith(
          (options) => DioException(
            requestOptions: options,
            type: DioExceptionType.unknown,
            error: 'something odd',
          ),
        ),
      );

      expect(result.errorCode, unknownError);
      expect(
        isTransportFailureCode(result.errorCode),
        isFalse,
        reason:
            '"we could not classify this" is not the same claim as "this was '
            'a transport failure" — callers must not treat it as one',
      );
    });

    test('badResponse stays an HTTP failure — never reported as transport',
        () async {
      final result = await _classifyThroughRealPipeline(
        (options) async => ResponseBody.fromString(
          '{"success":false,"error":{"code":"INTERNAL_SERVER_ERROR",'
          '"message":"Database error"}}',
          500,
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
          },
        ),
      );

      expect(result.errorCode, 'INTERNAL_SERVER_ERROR');
      expect(result.statusCode, 500);
      expect(
        isTransportFailureCode(result.errorCode),
        isFalse,
        reason:
            'a 5xx DID produce an HTTP envelope — it is a backend failure, not '
            'a transport failure, and the two must not be conflated',
      );
    });
  });

  test('isTransportFailureCode is true only for the transport family', () {
    for (final code in _transportTable.values) {
      expect(isTransportFailureCode(code), isTrue, reason: code);
    }
    expect(isTransportFailureCode(requestCancelled), isTrue);

    for (final notTransport in <String?>[
      unknownError,
      null,
      '',
      commerceRestricted,
      invalidPaymentStatus,
      marketAuthorityRequired,
      'INTERNAL_SERVER_ERROR',
    ]) {
      expect(isTransportFailureCode(notTransport), isFalse, reason: '$notTransport');
    }
  });

  test('the DioExceptionType table lives in exactly one file under lib/', () {
    final dioTypeCase = RegExp(r'^\s*case DioExceptionType\.\w+:', multiLine: true);
    final offenders = <String>[];
    for (final file in _dartFiles('lib')) {
      if (dioTypeCase.hasMatch(file.readAsStringSync())) {
        offenders.add(_rel(file));
      }
    }

    expect(
      offenders,
      [_classificationAuthority],
      reason:
          'transport classification was forked. Every DioExceptionType → code '
          'decision must live in $_classificationAuthority, so the API layer '
          'and Result.errorCode can never disagree about the same failure.',
    );
  });

  test('no transport code literal exists outside the authority file', () {
    // Declared here as literals on purpose: this gate is the detector.
    const literals = <String>[
      'BACKEND_UNREACHABLE',
      'TIMEOUT',
      'NETWORK_ERROR',
      'SSL_ERROR',
      'CANCELLED',
      'UNKNOWN_ERROR',
    ];

    final offenders = <String>[];
    for (final file in _dartFiles('lib')) {
      final path = _rel(file);
      if (path == _codeAuthority) continue;
      final source = file.readAsStringSync();
      for (final literal in literals) {
        if (source.contains("'$literal'")) {
          offenders.add('$path hardcodes $literal');
        }
      }
    }

    expect(
      offenders,
      isEmpty,
      reason:
          'error-code identity has one home ($_codeAuthority). A hardcoded '
          'code literal is a second declaration of the same identity and will '
          'drift:\n${offenders.join('\n')}',
    );

    // Positive proof: the authority still declares every transport code.
    final authority = File(_codeAuthority).readAsStringSync();
    for (final literal in literals) {
      expect(
        authority.contains("'$literal'"),
        isTrue,
        reason: '$_codeAuthority stopped declaring $literal',
      );
    }
  });

  test('the converged consumers classify by code, not by message text', () {
    // Both ends of the pipeline must route through the one table.
    for (final path in <String>[
      'lib/core/api/interceptors/error_interceptor.dart',
      'lib/core/api/api_client.dart',
    ]) {
      expect(
        File(path).readAsStringSync().contains('ApiExceptionFactory.fromTransport'),
        isTrue,
        reason: '$path stopped classifying transport failures through the table',
      );
    }

    // ...and the two callers that had to learn a new branch use the predicate.
    for (final path in <String>[
      'lib/domains/user/identity/authentication/presentation/providers/'
          'auth_controller.dart',
      'lib/domains/finance/transaction/payment/presentation/providers/'
          'payment_initiation_notifier.dart',
    ]) {
      expect(
        File(path).readAsStringSync().contains('isTransportFailureCode'),
        isTrue,
        reason:
            '$path stopped branching on the transport code. Matching the error '
            'text for network/connection/timeout is exactly what this '
            'convergence removed.',
      );
    }
  });

  test('the detectors actually fire (negative proof)', () {
    final dioTypeCase = RegExp(
      r'^\s*case DioExceptionType\.\w+:',
      multiLine: true,
    );
    expect(dioTypeCase.hasMatch('        case DioExceptionType.cancel:'), isTrue);
    expect(
      dioTypeCase.hasMatch('  // transport classification (DioExceptionType →)'),
      isFalse,
      reason: 'prose mentioning the type is not a second classification site',
    );
    expect(
      dioTypeCase.hasMatch('  // case DioExceptionType.cancel: was removed'),
      isFalse,
      reason: 'a commented-out branch is not a classification site',
    );

    final resurrected = "const NetworkException(message: 'x', code: 'NETWORK_ERROR');";
    expect(resurrected.contains("'NETWORK_ERROR'"), isTrue);
    expect(
      "const c = networkError;".contains("'NETWORK_ERROR'"),
      isFalse,
      reason: 'referencing the authority constant must stay legal',
    );
  });
}
