// Result authority contract: ONE result type for repository operations.
//
// Labuda grew copies of the same concept — "the outcome of a repository
// call" — and every copy drifted: `RepositoryResult<T>` in the order domain
// (`isSuccess => data != null`, success-first `fold`), `ContentRepositoryResult`
// in the content domain (its own `dataOrThrow`), and a typed-failure
// `RepositoryResult<T>` in the payment domain (`PaymentFailure` payload,
// success-first `fold`). Callers had to remember which `fold` order and which
// success rule each one used, and a null-yielding success read as a failure on
// two of them.
//
// `ApiResult` was another copy, in two incompatible shapes: a class in the
// support datasource (success-first `fold`, named error args) plus two separate
// record typedefs in the search domain — the same name declared twice, with NO
// error-code channel at all. Search therefore collapsed every failure into
// `error.toString()`, and support keyed its failure family off HTTP status
// numbers carried as strings plus message-text matching.
//
// Convergence: all of them are dead and every call site is on `Result<T>`
// (`lib/core/common/result.dart`), whose `fold` takes `onError` FIRST. This gate
// locks that so another copy cannot be introduced quietly:
//  1. `Result` is declared exactly once under `lib/`.
//  2. The duplicate names are gone from `lib/` and `test/`.
//  3. The killed files stay deleted, the scan surface is real, and the
//     canonical `fold` order (onError first) cannot silently flip back.
//  4. Payment's failure vocabulary stays dead and the repository forwards the
//     backend's code instead of re-deriving a kind from the error text.
//  5. Support classifies failures from `Result.errorCode`/`statusCode` — never
//     from the human message — and search carries the API failure code across
//     its throw boundary in exactly one place.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The single home of the repository-result authority.
const _authorityFile = 'lib/core/common/result.dart';

/// This gate states the forbidden names in order to detect them, so it is the
/// one file allowed to name them.
const _selfPath = 'test/core/result_authority_contract_test.dart';

/// Convergence killed these; they must not come back.
const _killedFiles = <String>[
  'lib/domains/commerce/transaction/order/domain/repositories/'
      'repository_result.dart',
  'lib/domains/finance/transaction/payment/domain/failures/'
      'payment_failure.dart',
];

const _duplicateNames = <String>[
  'RepositoryResult',
  'ContentRepositoryResult',
  'ApiResult',
];

/// The surfaces that used to declare their own result vocabulary and now
/// return the authority's `Result`.
const _convergedSurfaces = <String>[
  'lib/domains/system/support/data/datasources/support_api_datasource.dart',
  'lib/features/search/search/domain/repositories/search_repository.dart',
  'lib/features/search/search/domain/repositories/'
      'search_history_repository.dart',
];

/// The payment feature: its result type and its failure types are both gone.
const _paymentLib = 'lib/domains/finance/transaction/payment';

/// Every Dart file under [root] — the contract's scan surface.
List<File> _dartFiles(String root) => Directory(root)
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .toList();

String _rel(File file) => file.path.replaceAll(r'\', '/');

/// Matches a top-level declaration of one of the result types.
final _declaration = RegExp(
  r'^(?:abstract |sealed |final |base |mixin )?(?:class|typedef) '
  r'(RepositoryResult|ContentRepositoryResult|ApiResult|Result)\b',
);

/// Result name -> the files that declare it.
Map<String, Set<String>> _declarationHomes(List<File> files) {
  final homes = <String, Set<String>>{};
  for (final file in files) {
    for (final line in file.readAsLinesSync()) {
      final match = _declaration.firstMatch(line);
      if (match == null) continue;
      homes.putIfAbsent(match.group(1)!, () => <String>{}).add(_rel(file));
    }
  }
  return homes;
}

/// `PaymentFailure` as a standalone identifier — `_getPaymentFailureReason`
/// (a *payment status* reason, not the dead failure class) must stay legal, so
/// the match requires word boundaries on both sides.
final _paymentFailureName = RegExp(r'\bPaymentFailure\b');

void main() {
  test('Result is declared in exactly one file under lib/', () {
    final homes = _declarationHomes(_dartFiles('lib'));
    expect(
      homes['Result'],
      {_authorityFile},
      reason:
          'the repository-result authority moved or was copied. '
          'Expected Result to be declared only in $_authorityFile.',
    );
    expect(
      homes['ContentRepositoryResult'],
      isNull,
      reason: 'ContentRepositoryResult was converged away — it is a duplicate.',
    );
    expect(
      homes['RepositoryResult'],
      isNull,
      reason:
          'RepositoryResult was converged away — every repository returns '
          'Result<T> from $_authorityFile.',
    );
    expect(
      homes['ApiResult'],
      isNull,
      reason:
          'ApiResult was converged away — search and support return Result<T> '
          'from $_authorityFile, which carries errorCode/statusCode.',
    );
  });

  test('search and support return the authority Result, not a local copy', () {
    for (final path in _convergedSurfaces) {
      final source = File(path).readAsStringSync();
      expect(
        source.contains('Result<'),
        isTrue,
        reason: '$path stopped returning the authority result type',
      );
    }
  });

  test('support classifies a failure by its code, never by its message', () {
    final source = File(
      'lib/domains/system/support/data/repositories/support_repository_api.dart',
    ).readAsStringSync();

    // Positive proof: the transport family comes from the canonical predicate,
    // and the HTTP family from the status code the API layer preserved.
    expect(
      source.contains('isTransportFailureCode(code)'),
      isTrue,
      reason:
          'support stopped asking the transport authority what failed and is '
          'back to guessing from the message',
    );
    expect(
      source.contains('result.statusCode'),
      isTrue,
      reason: 'support stopped branching on the HTTP status code',
    );

    // Negative proof: text classification and status-code-as-string are the
    // exact shapes that made a drifted message mislabel a failure.
    expect(
      source.contains('error.contains('),
      isFalse,
      reason:
          'support is matching the human message again. A transport failure '
          'carries NETWORK_ERROR/TIMEOUT, never the word "network" — and any '
          'message rewrite silently moves the failure into the wrong family.',
    );
    expect(
      source.contains("case '404'"),
      isFalse,
      reason:
          "the HTTP status is an int on Result.statusCode; a '404' string case "
          'is the dead ApiResult code channel coming back',
    );
  });

  test('search carries the API failure code across its throw boundary', () {
    final service = File(
      'lib/features/search/search/data/remote/search_api_service.dart',
    ).readAsStringSync();

    // One classifier: the API layer's extractException (which delegates the
    // transport table to ApiExceptionFactory), carried through a
    // StructuredApiException so the repository can rebuild the code channel.
    expect(
      service.contains('extractException'),
      isTrue,
      reason: 'the search API service stopped classifying failures',
    );
    expect(
      service.contains('StructuredApiException'),
      isTrue,
      reason:
          'the search API service stopped preserving the failure code across '
          'the throw boundary',
    );
    expect(
      service.contains('DioExceptionType.'),
      isFalse,
      reason:
          'a second DioException -> code table is forbidden; the only one is '
          'ApiExceptionFactory.fromTransport',
    );

    for (final path in <String>[
      'lib/features/search/search/data/search_repository_impl.dart',
      'lib/features/search/search/data/search_history_repository_impl.dart',
    ]) {
      final source = File(path).readAsStringSync();
      expect(
        source.contains('error is StructuredApiException'),
        isTrue,
        reason: '$path stopped reading the preserved API failure code',
      );
      expect(
        source.contains('code: error.code'),
        isTrue,
        reason: '$path stopped forwarding the API failure code',
      );
      expect(
        source.contains('data: null, error:'),
        isFalse,
        reason:
            '$path is back on the code-less ApiResult record shape, which '
            'drops errorCode/statusCode',
      );
    }
  });

  test('no file anywhere names a duplicate result type', () {
    final offenders = <String>[];
    for (final root in ['lib', 'test']) {
      for (final file in _dartFiles(root)) {
        final path = _rel(file);
        if (path == _selfPath) continue;
        final source = file.readAsStringSync();
        for (final name in _duplicateNames) {
          if (source.contains(name)) offenders.add('$path names $name');
        }
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'a duplicate result type is back. Every repository returns Result<T> '
          'from $_authorityFile:\n${offenders.join('\n')}',
    );
  });

  test('the killed result authorities stay deleted and the sweep is real', () {
    // A floor, not an exact count: the lock must cover the real app instead of
    // passing because the sweep found nothing to read.
    expect(_dartFiles('lib').length, greaterThan(1000));
    for (final path in _killedFiles) {
      expect(
        File(path).existsSync(),
        isFalse,
        reason: 'dead duplicate resurrected: $path',
      );
    }
  });

  test('the canonical fold order — onError first — cannot silently flip', () {
    final source = File(_authorityFile).readAsStringSync();
    final onError = source.indexOf('U Function(String error) onError');
    final onSuccess = source.indexOf('U Function(T data) onSuccess');
    expect(onError, greaterThan(-1), reason: 'fold(onError) signature gone');
    expect(
      onSuccess,
      greaterThan(-1),
      reason: 'fold(onSuccess) signature gone',
    );
    expect(
      onError,
      lessThan(onSuccess),
      reason:
          'Result.fold must take onError first. Every call site in the app was '
          'rewritten to this order when RepositoryResult died; flipping it back '
          'would silently swap success and error branches at ~40 call sites.',
    );
  });

  test(
    'payment forwards the backend code instead of inventing a failure kind',
    () {
      final offenders = <String>[];
      for (final file in _dartFiles(_paymentLib)) {
        final source = file.readAsStringSync();
        if (_paymentFailureName.hasMatch(source)) {
          offenders.add('${_rel(file)} names PaymentFailure');
        }
        if (source.contains('_mapApiError')) {
          offenders.add('${_rel(file)} re-classifies the error (_mapApiError)');
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'payment must let the backend code say what failed. The old '
            '_mapApiError grepped the error text for network/expired/not found '
            'and fabricated typed payloads for it:\n${offenders.join('\n')}',
      );

      // Positive proof: the repository keeps the authority's machine-readable
      // fields instead of flattening the failure into a message.
      final repoSource = File(
        '$_paymentLib/data/repositories/payment_repository_impl.dart',
      ).readAsStringSync();
      for (final needle in ['source.errorCode', 'source.statusCode']) {
        expect(
          repoSource.contains(needle),
          isTrue,
          reason: 'payment repository stopped forwarding $needle',
        );
      }
    },
  );

  test(
    'the duplicate-declaration detector actually fires (negative proof)',
    () {
      expect(
        _declaration.hasMatch('class ContentRepositoryResult<T> {'),
        isTrue,
      );
      expect(_declaration.hasMatch('class RepositoryResult<T> {'), isTrue);
      expect(_declaration.hasMatch('class ApiResult<T> {'), isTrue);
      expect(
        _declaration.hasMatch(
          'typedef ApiResult<T> = ({T? data, String? error});',
        ),
        isTrue,
        reason:
            'the search-domain typedef form must be detected too — it is the '
            'shape that carries no error-code channel at all',
      );
      expect(
        _declaration.hasMatch(
          'typedef ContentRepositoryResult<T> = Result<T>;',
        ),
        isTrue,
      );
      expect(
        _declaration.hasMatch('class Result<T> {'),
        isTrue,
        reason: 'a second Result declaration is a duplicate authority too',
      );

      expect(
        _declaration.hasMatch('final result = Result.success(1);'),
        isFalse,
      );
      expect(_declaration.hasMatch('  // RepositoryResult is gone'), isFalse);
      expect(
        _declaration.hasMatch('class ResultBanner extends StatelessWidget {'),
        isFalse,
      );
    },
  );
}
