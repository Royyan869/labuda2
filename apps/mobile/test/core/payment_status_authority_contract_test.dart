// Payment-status authority: ONE vocabulary, and `pending` means ONE thing.
//
// The wire carried two competing truths for the same money question:
//
//   * the canonical type declares that gateway vocabulary is forbidden on this
//     wire and that an unrecognised value is rejected loudly;
//
//   * commerce's order mapper kept a private translation table
//     ('settlement' / 'capture' / 'challenge' — the last one the backend does
//     not even have) and silently degraded ANY unknown value to `pending`.
//
// So `pending` carried three meanings at once: "no payment row exists", "the
// status is unrecognised", and "there is a bill waiting to be paid". On a money
// surface that is not a nuance — it tells a buyer who never received a bill to
// go and pay one.
//
// The backend now normalises at its own edge (the package that owns the payment
// state: settlement|capture -> paid, deny -> failed, expire -> expired, cancel
// and anything unrecognised -> NO VERDICT), and the client translates nothing.
//
// This gate locks four things:
//  1. No Dart file under lib/ may contain a QUOTED gateway literal. A quoted
//     one is the resurrection signature of the table that died. Prose may name
//     the word; code may not hold it as a value.
//  2. The canonical mapper still maps the canonical names, so the sweep cannot
//     be satisfied by deleting the translation altogether.
//  3. `pending` is reachable ONLY from the wire value `pending` — never from an
//     absent, empty or unrecognised value.
//  4. Negative proof: the detector fires on the deleted table's shape.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/data/mappers/order_mapper.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    show PaymentStatus;

/// The gateway's own vocabulary for a payment row. None of these is a canonical
/// buyer-facing verdict, so no Dart file may hold one as a value.
const _gatewayLiterals = <String>[
  "'settlement'",
  "'capture'",
  "'challenge'",
  "'deny'",
];

/// Every Dart file swept, including untracked ones on disk.
List<File> _sweptFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .toList();

bool _quotesGatewayVocabulary(String source) =>
    _gatewayLiterals.any(source.contains);

void main() {
  test('no code quotes gateway vocabulary — the table stays dead', () {
    final offenders = <String>[];
    for (final file in _sweptFiles()) {
      final path = file.path.replaceAll(r'\', '/');
      final source = file.readAsStringSync();
      for (final literal in _gatewayLiterals) {
        if (source.contains(literal)) {
          offenders.add('$path holds $literal');
        }
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'gateway vocabulary is being translated in the client again. The '
          'payments-table words belong to finance and are normalised by the '
          'backend before the wire; a client-side table is a second truth for '
          'the same money question:\n${offenders.join('\n')}',
    );
  });

  test('the canonical names still map (anti-vacuum floor)', () {
    // A floor > 600 files: the sweep must cover the real app instead of passing
    // because it found nothing to read.
    expect(_sweptFiles().length, greaterThan(600));
    expect(OrderMapper.mapPaymentStatus('pending'), PaymentStatus.pending);
    expect(OrderMapper.mapPaymentStatus('paid'), PaymentStatus.paid);
    expect(OrderMapper.mapPaymentStatus('failed'), PaymentStatus.failed);
    expect(OrderMapper.mapPaymentStatus('expired'), PaymentStatus.expired);
    expect(OrderMapper.mapPaymentStatus('refunded'), PaymentStatus.refunded);
  });

  test('pending is reachable ONLY from the wire value pending', () {
    // No verdict: absent or blank wire value says nothing about payment.
    expect(OrderMapper.mapPaymentStatus(''), isNull);
    expect(OrderMapper.mapPaymentStatus('   '), isNull);
    // Unrecognised vocabulary is a contract violation, not a money state.
    expect(
      () => OrderMapper.mapPaymentStatus('future_gateway_status'),
      throwsFormatException,
    );
    // And the gateway words the dead table used to translate are rejected.
    for (final raw in ['settlement', 'capture', 'challenge', 'deny']) {
      expect(
        () => OrderMapper.mapPaymentStatus(raw),
        throwsFormatException,
        reason: 'raw gateway status must never reach a buyer-facing verdict: $raw',
      );
    }
  });

  test('the detector fires on the deleted table, not on prose (negative proof)', () {
    expect(
      _quotesGatewayVocabulary("case 'settlement': return PaymentStatus.paid;"),
      isTrue,
    );
    expect(
      _quotesGatewayVocabulary("    if (status == 'capture') return true;"),
      isTrue,
    );
    // Prose may name the concept; only a held value is a resurrection.
    expect(
      _quotesGatewayVocabulary(
        '// settlement is settled in the payments table, not here',
      ),
      isFalse,
    );
    // Auction's waiting_settlement is a different concept and must stay legal.
    expect(_quotesGatewayVocabulary("'waiting_settlement'"), isFalse);
  });
}
