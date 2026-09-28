// Dead-file contract: purged dead code stays purged AND stays unreferenced.
//
// Three files shipped as if they were live capability, with ZERO consumers in the
// whole app:
//
//  * `lib/core/dependencies/provider_scope_reader.dart` (27 lines) — a private
//    `_ProviderScopeHolder extends InheritedWidget` never constructed, imported
//    or barrel-exported, plus an unused `core.dart` import and a doc claiming it
//    was the "canonical service-locator authority" while pointing elsewhere.
//
//  * `lib/core/utils/retry_helper.dart` (284 lines) — `RetryConfig`,
//    `RetryHelper`, `RetryFutureExtension`. It advertised exponential backoff
//    with jitter, and its `_isRetryableError` decided retryability by
//    substring-matching the error TEXT ('network'/'connection'/'timeout'/
//    '500'/…). Nothing imported it, `core.dart` never exported it, and no test
//    named it: a retry policy the app never ran, classifying failures the way
//    the transport convergence explicitly outlawed.
//
//  * `lib/domains/finance/finance_gateway.dart` — `abstract class FinanceGateway`
//    plus its own `FinanceResult<T>` (`dataOrThrow`, `isSuccessful`) and
//    `FinanceException`. No implementor, no consumer, no barrel. Its doc
//    declared "commerce MUST go through FinanceGateway"; the app had already
//    overruled that boundary in BOTH directions (commerce imports
//    `domains/finance/...`, finance imports `domains/commerce/...`).
//
// A dead file is not neutral. It is a capability claim a future reader adopts,
// and `FinanceResult` was a sixth result vocabulary waiting to be revived. This
// gate locks four things:
//  1. The purged files stay deleted.
//  2. No CODE names them again — not by identifier, and not by path (an
//     `import`/`export` of the path is how they would actually come back).
//     Docs are deliberately out of scope: the convergence ledger records these
//     purges BY NAME, and prose cannot resurrect compiled code.
//  3. The sweep surface is real (file-count floor) and the purge was scoped —
//     the deleted files' live neighbours are still present, so this lock cannot
//     be satisfied by wiping a directory.
//  4. Negative proof: both detectors fire on a planted resurrection.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// This gate states the forbidden names in order to detect them, so it is the
/// one file allowed to name them.
const _selfPath = 'test/core/dead_file_purge_contract_test.dart';

/// Purged. They must never exist again.
const _purgedFiles = <String>[
  'lib/core/utils/retry_helper.dart',
  'lib/domains/finance/finance_gateway.dart',
  'lib/core/dependencies/provider_scope_reader.dart',
];

/// The public vocabulary each purged file owned. Naming it again in code means
/// the dead abstraction is coming back — or a new result type is being built on
/// its corpse.
const _purgedIdentifiers = <String>[
  'RetryHelper',
  'RetryConfig',
  'RetryFutureExtension',
  'FinanceGateway',
  'FinanceResult',
  'FinanceException',
  '_ProviderScopeHolder',
];

/// Path fragments, because identifier matching alone misses the real
/// resurrection route: `import 'package:labuda/core/utils/retry_helper.dart';`.
const _purgedPathFragments = <String>[
  'utils/retry_helper.dart',
  'finance/finance_gateway.dart',
  'dependencies/provider_scope_reader.dart',
];

/// Live files that must survive the purge — proof that two files died, not two
/// directories.
const _mustSurvive = <String>[
  'lib/core/utils/polling_monitor.dart',
  'lib/core/utils/notification_navigation_handler.dart',
  'lib/domains/finance/transaction/payment/domain/repositories/'
      'payment_repository.dart',
];

/// Every file whose contents are swept: all Dart under `lib`/`test`, plus the
/// pubspec (a dependency or asset entry could name a deleted path).
List<File> _sweptFiles() {
  final files = <File>[File('pubspec.yaml')];
  for (final root in ['lib', 'test']) {
    files.addAll(
      Directory(root)
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart')),
    );
  }
  return files;
}

/// `RetryHelper` as a standalone identifier — `_isRetryableError`-style local
/// names must not be flagged, so the match requires word boundaries.
RegExp _identifier(String name) => RegExp('\\b$name\\b');

void main() {
  test('the purged dead files stay deleted', () {
    for (final path in _purgedFiles) {
      expect(
        File(path).existsSync(),
        isFalse,
        reason:
            'purged dead file resurrected: $path. It had zero consumers and was '
            'a fake capability claim — re-add it only with a real call site and '
            'a replacement for the text-matching rule it carried.',
      );
    }
  });

  test('no code names a purged dead file or its vocabulary', () {
    final offenders = <String>[];
    for (final file in _sweptFiles()) {
      final path = file.path.replaceAll(r'\', '/');
      if (path == _selfPath) continue;
      final source = file.readAsStringSync();
      for (final name in _purgedIdentifiers) {
        if (_identifier(name).hasMatch(source)) {
          offenders.add('$path names $name');
        }
      }
      for (final fragment in _purgedPathFragments) {
        if (source.contains(fragment)) {
          offenders.add('$path imports/exports $fragment');
        }
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'a purged dead file is back in code. These abstractions had no '
          'consumer; reviving the name without a call site re-creates the fake '
          'capability (and, for FinanceResult, a sixth result vocabulary):'
          '\n${offenders.join('\n')}',
    );
  });

  test('the sweep surface is real and the purge was scoped', () {
    // Floors, not exact counts: the lock must cover the real app instead of
    // passing because the sweep found nothing to read.
    expect(_sweptFiles().length, greaterThan(1400));
    for (final path in _mustSurvive) {
      expect(
        File(path).existsSync(),
        isTrue,
        reason:
            'live neighbour of a purged file is gone: $path. The purge must '
            'delete the dead file only — never its directory.',
      );
    }
  });

  test('both detectors actually fire (negative proof)', () {
    expect(_identifier('RetryHelper').hasMatch('class RetryHelper {'), isTrue);
    expect(
      _identifier('FinanceGateway').hasMatch('abstract class FinanceGateway {'),
      isTrue,
    );
    expect(
      _identifier(
        'FinanceResult',
      ).hasMatch('Future<FinanceResult> charge({required String orderId});'),
      isTrue,
    );
    // The path fragment must catch the import/export form that identifiers miss.
    const resurrection =
        "import 'package:labuda/core/utils/retry_helper.dart';";
    expect(
      _purgedPathFragments.any(resurrection.contains),
      isTrue,
      reason: 'an import of the purged path must be detectable',
    );

    expect(
      _identifier('RetryHelper').hasMatch('_isRetryableError(e)'),
      isFalse,
    );
    expect(
      _identifier('FinanceResult').hasMatch('FinanceResultX'),
      isFalse,
      reason: 'a different name sharing the prefix must stay legal',
    );
    // The dead service-locator holder must be caught by name and by import path.
    expect(
      _identifier('_ProviderScopeHolder')
          .hasMatch('class _ProviderScopeHolder extends InheritedWidget {'),
      isTrue,
    );
    const holderImport =
        "import 'package:labuda/core/dependencies/provider_scope_reader.dart';";
    expect(
      _purgedPathFragments.any(holderImport.contains),
      isTrue,
      reason: 'an import of the purged path must be detectable',
    );
  });
}
