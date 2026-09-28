// Domain boundary contract: commerce may depend on finance; finance must NOT
// depend on commerce.
//
// The boundary was never enforced — it only ever existed as a doc comment in
// `lib/domains/finance/finance_gateway.dart`, a file with zero implementors,
// zero consumers and zero barrels that claimed "commerce MUST go through
// FinanceGateway". Reality had already overruled it in BOTH directions, and the
// purge of that dead file left the rule with no home at all. Two competing
// authorities for the same money question:
//
//  * backend: payment is upstream. The gateway webhook settles the payment row,
//    and `order_completion_service.go` REFUSES to complete an order until the
//    payment is settlement/capture ("cannot complete order: payment not
//    confirmed"). `OrderCompletionService.MarkPaid` is the commerce transition
//    of a projection — so truth flows payment -> order.
//
//  * mobile, before this gate: `payment_result_notifier.dart` and
//    `payment_result_state.dart` lived in `lib/domains/finance/...` but imported
//    `order_providers.dart` (commerce repository), `Order` and `OrderStatus`
//    (commerce entities), and decided the payment outcome by switching on
//    commerce's order state machine. Finance could not answer a payment
//    question without commerce, and its only consumer was a COMMERCE screen
//    (`lib/domains/commerce/transaction/checkout/presentation/screens/
//    payment_result_screen_impl.dart`).
//
// The reconciliation now lives where its consumer and its authority input live:
// `lib/domains/commerce/transaction/checkout/presentation/providers/`. That
// makes the mobile dependency graph match the backend's direction — commerce
// -> finance, acyclic — and finance becomes free of commerce entirely.
//
// This gate locks four things:
//  1. No file under finance declares a `commerce/` directive. Only an
//     `import`/`export`/`part` creates a compile-time dependency, so the
//     detector is directive-scoped: prose, and data strings that merely name a
//     commerce path (a finance test scanning a commerce file), are legal.
//  2. The reconciliation stays OUT of finance and stays IN commerce.
//  3. The sweep surface is real (file-count floor) and the finance side is not
//     hollowed out to satisfy the lock.
//  4. Negative proof: the detector fires on every real resurrection form and
//     stays quiet on prose.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The allowed direction. Reaching across it from the other side is the
/// coupling this gate forbids.
const _dependentRoots = <String>['lib/domains/finance', 'test/domains/finance'];

/// Files that must survive: proof the finance module is still the money module,
/// not an emptied directory that trivially satisfies the lock.
const _financeMustStillExist = <String>[
  'lib/domains/finance/transaction/payment/domain/entities/payment.dart',
  'lib/domains/finance/transaction/payment/domain/repositories/'
      'payment_repository.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_providers.dart',
  'lib/domains/finance/wallet/coins/coins.dart',
];

/// The reconciliation moved OUT of finance (it used to be here, importing
/// commerce) and INTO commerce checkout, next to the screen that consumes it.
const _movedOutOfFinance = <String>[
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_result_notifier.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_result_state.dart',
];
const _movedIntoCommerce = <String>[
  'lib/domains/commerce/transaction/checkout/presentation/providers/'
      'payment_result_notifier.dart',
  'lib/domains/commerce/transaction/checkout/presentation/providers/'
      'payment_result_state.dart',
];

/// A directive that names a `commerce/` path. This is the resurrection route:
/// `import 'package:labuda/domains/commerce/...'` or a relative
/// `import '../../commerce/...'`. A `//` comment explaining the boundary, or a
/// string that merely names the path, is NOT a dependency.
final _commerceDirective = RegExp(
  r'''^\s*(?:import|export|part)\b[^;]*commerce/''',
  multiLine: true,
);

List<File> _sweptFiles() {
  final files = <File>[];
  for (final root in _dependentRoots) {
    final directory = Directory(root);
    if (!directory.existsSync()) continue;
    files.addAll(
      directory
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart')),
    );
  }
  return files;
}

void main() {
  test('finance declares no dependency on commerce (one-way boundary)', () {
    final offenders = <String>[];
    for (final file in _sweptFiles()) {
      final path = file.path.replaceAll(r'\', '/');
      final source = file.readAsStringSync();
      for (final match in _commerceDirective.allMatches(source)) {
        offenders.add('$path :: ${match.group(0)!.trim()}');
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'finance took a compile-time dependency on commerce. Truth flows '
          'payment -> order (backend refuses to complete an order until the '
          'payment is settled), so commerce may depend on finance — never the '
          'reverse. A payment concern that needs the Order belongs with its '
          'consumer in commerce, not inside the money domain:\n'
          '${offenders.join('\n')}',
    );
  });

  test('the reconciliation lives with its consumer, not in finance', () {
    for (final path in _movedOutOfFinance) {
      expect(
        File(path).existsSync(),
        isFalse,
        reason:
            'the payment-result reconciliation is back in finance: $path. It '
            'reads Order/OrderStatus and the commerce order repository, so '
            'living here re-creates the very coupling this boundary forbids.',
      );
    }
    for (final path in _movedIntoCommerce) {
      expect(
        File(path).existsSync(),
        isTrue,
        reason:
            'the reconciliation is missing from commerce: $path. It belongs '
            'next to payment_result_screen_impl.dart, the only consumer.',
      );
    }
  });

  test('the sweep surface is real and finance is not hollowed out', () {
    expect(
      _sweptFiles().length,
      greaterThan(40),
      reason:
          'the sweep found almost nothing to read, so the boundary lock would '
          'pass vacuously. Finance is a real module; the floor must cover it.',
    );
    for (final path in _financeMustStillExist) {
      expect(
        File(path).existsSync(),
        isTrue,
        reason:
            'finance lost a module file: $path. The boundary fix is a move of '
            'one concern, not a demolition of the money domain.',
      );
    }
  });

  test('the canonical direction is still live (anti-vacuum positive proof)', () {
    final commerceRoot = Directory('lib/domains/commerce');
    final edges = commerceRoot
        .listSync(recursive: true)
        .whereType<File>()
        .where((f) => f.path.endsWith('.dart'))
        .where((f) => f.readAsStringSync().contains('domains/finance/'))
        .length;
    expect(
      edges,
      greaterThan(0),
      reason:
          'commerce no longer reaches finance at all, which cannot be right: '
          'checkout initiates payments and reads coin balance through finance. '
          'Deleting the allowed direction is not a boundary.',
    );
  });

  test('the detector fires on real resurrection, not on prose (negative proof)', () {
    const packageImport =
        "import 'package:labuda/domains/commerce/transaction/order/"
        "domain/entities/order.dart';";
    const relativeImport =
        "import '../../../commerce/transaction/order/data/"
        "order_providers.dart';";
    // An `export` is a dependency too — a barrel would hide the coupling from
    // every file that imports it.
    const relativeExport =
        "export '../../commerce/transaction/order/data/order_providers.dart';";
    expect(_commerceDirective.hasMatch(packageImport), isTrue);
    expect(_commerceDirective.hasMatch(relativeImport), isTrue);
    expect(_commerceDirective.hasMatch(relativeExport), isTrue);

    // Prose and data strings must stay legal: the convergence ledger, and a
    // finance test that scans a commerce file BY PATH, are not dependencies.
    expect(
      _commerceDirective.hasMatch('// commerce owns orders; finance owns money'),
      isFalse,
    );
    expect(
      _commerceDirective.hasMatch(
        "    'lib/domains/commerce/transaction/order/domain/entities/"
        "order.dart';",
      ),
      isFalse,
    );
    // The allowed direction must never be flagged.
    expect(
      _commerceDirective.hasMatch(
        "import 'package:labuda/domains/finance/transaction/payment/"
        "domain/entities/payment.dart';",
      ),
      isFalse,
    );
  });
}
