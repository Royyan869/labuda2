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
// and `FinanceResult` was a sixth result vocabulary waiting to be revived.
//
// A second purge wave killed the FAILURE-side vocabulary: `core/errors/failure.dart`
// (a `Failure` base plus seven typed subclasses, consumed by nothing but six use
// cases that have since converged onto the authority `Result`),
// `support/domain/entities/support_failure.dart` (a typed family behind a
// duplicate `SupportResult<T>` wrapper whose consumers only ever read
// `.message`), and `share/domain/entities/share_failure.dart` (five kinds
// riding a dartz `Either`). With dartz out of the pubspec, the app has ONE
// result vocabulary.
//
// A third purge wave killed the finance payment zombie — residue kept visible by
// a barrel export and a generated freezed state that nobody read:
//
//  * `lib/domains/finance/transaction/payment/domain/entities/payment_result.dart`
//    — `PaymentResult` + `PaymentResultStatus`, four factories and four
//    predicates, constructed NOWHERE in `lib` or `test`.
//  * `.../payment/presentation/providers/payment_state.dart` (freezed) —
//    `PaymentState` with `paymentCreated`/`paymentLoaded`/`paymentsLoaded`/
//    `paymentSuccess` variants and zero readers. The live flow is
//    `PaymentInitiationNotifier` plus the canonical `PaymentWebviewScreen`.
//  * `.../payment/presentation/providers/payment_notifier.dart` —
//    `PaymentNotifier`, zero references outside itself; it existed only to
//    consume the dead state.
//
// Their `.freezed.dart` / `.g.dart` artefacts died with the sources, and the
// domain barrel stopped exporting the dead entity.
//
// A fourth wave opened the ZERO-CONSUMER SWEEP: files that no `import`/`export`/
// `part` anywhere in `lib`+`test` names, and whose declared types are referenced
// nowhere outside themselves. Slice 1 purged the shared-layer ghost widgets —
// including the whole `ui/atomic/**` tree (every file dead, directories removed)
// and the `widgets/list_item/` island (its three members were reachable only
// through the dead barrel). Names are locked by PATH, not identifier:
// `StatusBadge`, `CardHeader` and friends are generic UI vocabulary, and the
// revival route for a widget is the import path.
//
// This
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
  'lib/core/errors/failure.dart',
  'lib/domains/system/support/domain/entities/support_failure.dart',
  'lib/domains/social/share/domain/entities/share_failure.dart',
  'lib/domains/finance/transaction/payment/domain/entities/payment_result.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_state.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_state.freezed.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_notifier.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_notifier.g.dart',
  // Slice 1 of the zero-consumer sweep: the shared-layer ghost widgets. Every
  // declared type in these files had ZERO references outside its own file, and
  // no `import`/`export`/`part` in `lib` or `test` named the file.
  'lib/shared/ui/atomic/input/description_input_component.dart',
  'lib/shared/ui/atomic/input/price_input_component.dart',
  'lib/shared/ui/atomic/input/title_input_component.dart',
  'lib/shared/ui/atomic/location/location_picker_component.dart',
  'lib/shared/ui/atomic/settings/visibility_settings_component.dart',
  'lib/shared/ui/atomic/tagging/user_tagging_component.dart',
  'lib/shared/ui/src/widgets/text_input_reply_preview.dart',
  'lib/shared/widgets/address_display_section.dart',
  'lib/shared/widgets/base_metric_card.dart',
  'lib/shared/widgets/card_header.dart',
  'lib/shared/widgets/hashtag_input_widget.dart',
  'lib/shared/widgets/map_picker/map_accuracy_indicator.dart',
  'lib/shared/widgets/media_carousel_widget_container.dart',
  'lib/shared/widgets/media_carousel_widget_content.dart',
  'lib/shared/widgets/media_viewer_navigation.dart',
  'lib/shared/widgets/payment_method_card.dart',
  'lib/shared/widgets/polling_status_indicator.dart',
  'lib/shared/widgets/status_badge.dart',
  'lib/shared/widgets/list_item/list_item.dart',
  'lib/shared/widgets/list_item/list_item_types.dart',
  'lib/shared/widgets/list_item/list_item_trailing.dart',
  'lib/shared/widgets/list_item/list_item_decorations.dart',
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
  'SupportResult',
  'ShareFailureType',
  'ValidationFailure',
  'UnknownFailure',
  'NetworkFailure',
  'ServerFailure',
  'CacheFailure',
  'AuthenticationFailure',
  'AuthorizationFailure',
  'FailureFactory',
  // Third wave: the finance payment zombie (zero consumers, zero constructors).
  // `PaymentResult` is locked as a STANDALONE identifier, not as a name family:
  // commerce's live `PaymentResultState` / `PaymentResultNotifier` legitimately
  // keep their names, so a prefix ban would break canonical code.
  'PaymentResult',
  'PaymentResultStatus',
  'PaymentNotifier',
  'PaymentState',
  'PaymentInitial',
  'PaymentLoading',
  'PaymentCreated',
  'PaymentLoaded',
  'PaymentsLoaded',
  'PaymentSuccess',
  'PaymentError',
];

/// Vocabulary families whose suffixed variants must stay dead too —
/// word-boundary prefix, so `SupportFailureNetwork` / `ShareFailureType` are
/// caught while names that merely start the same are not.
const _purgedNameFamilies = <String>['SupportFailure', 'ShareFailure'];

RegExp _family(String prefix) => RegExp('\\b$prefix\\w*');

/// Path fragments, because identifier matching alone misses the real
/// resurrection route: `import 'package:labuda/core/utils/retry_helper.dart';`.
const _purgedPathFragments = <String>[
  'utils/retry_helper.dart',
  'finance/finance_gateway.dart',
  'dependencies/provider_scope_reader.dart',
  'core/errors/failure.dart',
  'entities/support_failure.dart',
  'entities/share_failure.dart',
  'transaction/payment/domain/entities/payment_result.dart',
  'transaction/payment/presentation/providers/payment_state.dart',
  'transaction/payment/presentation/providers/payment_notifier.dart',
  'ui/atomic/input/description_input_component.dart',
  'ui/atomic/input/price_input_component.dart',
  'ui/atomic/input/title_input_component.dart',
  'ui/atomic/location/location_picker_component.dart',
  'ui/atomic/settings/visibility_settings_component.dart',
  'ui/atomic/tagging/user_tagging_component.dart',
  'ui/src/widgets/text_input_reply_preview.dart',
  'widgets/address_display_section.dart',
  'widgets/base_metric_card.dart',
  'widgets/card_header.dart',
  'widgets/hashtag_input_widget.dart',
  'widgets/map_picker/map_accuracy_indicator.dart',
  'widgets/media_carousel_widget_container.dart',
  'widgets/media_carousel_widget_content.dart',
  'widgets/media_viewer_navigation.dart',
  'widgets/payment_method_card.dart',
  'widgets/polling_status_indicator.dart',
  'widgets/status_badge.dart',
  'widgets/list_item/list_item.dart',
  'widgets/list_item/list_item_types.dart',
  'widgets/list_item/list_item_trailing.dart',
  'widgets/list_item/list_item_decorations.dart',
];

/// Sibling gates that detect these very names — they must state what they
/// detect (prose about a purge cannot resurrect compiled code, and a scanner
/// naming its target is the detector, not a use site). Only non-gate code
/// counts against this lock.
const _gatePeers = <String>[
  'test/core/result_authority_contract_test.dart',
  'test/core/domain_boundary_contract_test.dart',
];

/// Live files that must survive the purge — proof that two files died, not two
/// directories.
const _mustSurvive = <String>[
  'lib/core/utils/polling_monitor.dart',
  'lib/core/utils/notification_navigation_handler.dart',
  'lib/core/core.dart',
  'lib/domains/finance/transaction/payment/domain/repositories/'
      'payment_repository.dart',
  'lib/domains/system/support/domain/entities/support_ticket.dart',
  'lib/domains/social/share/domain/entities/share_result.dart',
  'lib/domains/finance/transaction/payment/domain/entities/payment.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_providers.dart',
  'lib/domains/finance/transaction/payment/presentation/providers/'
      'payment_initiation_notifier.dart',
  'lib/shared/ui/base/base_component.dart',
  'lib/shared/widgets/hybrid_avatar.dart',
  'lib/shared/widgets/base_card.dart',
  'lib/shared/widgets/media_carousel_widget.dart',
  'lib/shared/widgets/media_viewer_indicators.dart',
  'lib/shared/widgets/map_picker/map_picker_widgets.dart',
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
      if (path == _selfPath || _gatePeers.contains(path)) continue;
      final source = file.readAsStringSync();
      for (final name in _purgedIdentifiers) {
        if (_identifier(name).hasMatch(source)) {
          offenders.add('$path names $name');
        }
      }
      for (final prefix in _purgedNameFamilies) {
        if (_family(prefix).hasMatch(source)) {
          offenders.add('$path names the $prefix family');
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

    // The second-wave corpses: names, subclass families, and import paths.
    expect(_identifier('SupportResult').hasMatch('class SupportResult<T> {'),
        isTrue);
    expect(
      _identifier('SupportResult').hasMatch('SupportResultBanner'),
      isFalse,
      reason: 'a different name sharing the prefix must stay legal',
    );
    expect(_family('SupportFailure').hasMatch('const SupportFailureNetwork()'),
        isTrue);
    expect(_family('ShareFailure').hasMatch('enum ShareFailureType {'), isTrue);
    expect(_family('SupportFailure').hasMatch('NotSupportFailure'), isFalse);
    const supportImport =
        "import 'package:labuda/domains/system/support/domain/entities/"
        "support_failure.dart';";
    expect(
      _purgedPathFragments.any(supportImport.contains),
      isTrue,
      reason: 'an import of the purged support failure path must be detectable',
    );

    // Third wave: the finance payment zombie.
    expect(_identifier('PaymentState').hasMatch('class PaymentState {'), isTrue);
    expect(
      _identifier(
        'PaymentResult',
      ).hasMatch('Future<PaymentResult> charge({required String id});'),
      isTrue,
    );
    const zombieImport =
        "import 'package:labuda/domains/finance/transaction/payment/domain/"
        "entities/payment_result.dart';";
    expect(
      _purgedPathFragments.any(zombieImport.contains),
      isTrue,
      reason: 'an import of the purged payment_result path must be detectable',
    );
    // The LIVE commerce recon names must stay legal: this lock is a
    // word-boundary identifier lock, not a prefix ban.
    expect(
      _identifier('PaymentResult').hasMatch('class PaymentResultState {'),
      isFalse,
      reason:
          'commerce PaymentResultState/Notifier are canonical and must not be '
          'flagged by the finance payment zombie lock',
    );
    expect(
      _identifier('PaymentState').hasMatch('class PaymentStateX {'),
      isFalse,
      reason: 'a different name sharing the prefix must stay legal',
    );

    // Slice 1: path-only lock. The widget names themselves stay legal — their
    // FILES must never come back.
    const ghostWidgetImport =
        "import 'package:labuda/shared/widgets/status_badge.dart';";
    expect(
      _purgedPathFragments.any(ghostWidgetImport.contains),
      isTrue,
      reason: 'an import of a purged shared widget must be detectable',
    );
    expect(
      _purgedPathFragments.any(
        "import 'package:labuda/shared/widgets/media_carousel_widget.dart';"
            .contains,
      ),
      isFalse,
      reason: 'the LIVE carousel widget must not match a purged fragment',
    );
    expect(
      _purgedPathFragments.any("export 'list_item_types.dart';".contains),
      isFalse,
      reason: 'the lock keys on the real import path, not on a bare basename',
    );
  });
}
