// Payment decision-contract phantom purge (kill once, lock forever).
//
// Satu decision contract untuk seluruh app, dan backend hanya punya satu:
// `backend/internal/commerce/order/delivery/http/dto/decision.go`
// (primary_action / secondary_actions / decision_version / display).
//
// Payment dulu membawa salinan kedua: `DecisionContract` + `DisplayHints` +
// `DecisionContractResponseDto` + field `Payment.decision`, yang mem-parse
// `allowed_actions` — key yang TIDAK PERNAH ada di backend (`grep allowed_actions`
// di backend = nol). DTO-nya sendiri sudah mendokumentasikan bahwa
// `GET /payments/:id` tidak mengirim `decision`, dan nol pembaca di lib/
// maupun pin test. Rantai itu zombie: phantom wire + phantom authority.
//
// Gate ini menjaga tiga hal: rantai phantom tetap mati, decision contract
// tetap punya tepat SATU rumah, dan wire kanonik tetap ter-parse tanpanya.
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/types/payment_types.dart';
import 'package:hishumi/domains/finance/transaction/payment/data/dto/payment_dto.dart';

/// Satu-satunya rumah decision contract (backend parity).
const _canonicalContractHome =
    'lib/domains/commerce/transaction/order/domain/entities/order.dart';

/// File payment yang dulu memelihara salinannya.
const _purgedFiles = <String>[
  'lib/domains/finance/transaction/payment/domain/entities/payment.dart',
  'lib/domains/finance/transaction/payment/data/dto/payment_dto.dart',
];

void main() {
  test('the phantom payment decision chain stays purged', () {
    for (final path in _purgedFiles) {
      final source = File(path).readAsStringSync();
      for (final phantom in [
        'DecisionContract',
        'DisplayHints',
        'allowed_actions',
      ]) {
        expect(
          source.contains(phantom),
          isFalse,
          reason: '$path tidak boleh memuat $phantom lagi',
        );
      }
    }
  });

  test('the decision contract has exactly one home in lib', () {
    final homes = <String, List<String>>{};
    for (final file in Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .where((f) => f.path.endsWith('.dart'))) {
      final path = file.path.replaceAll(r'\', '/');
      for (final line in file.readAsLinesSync()) {
        for (final name in ['class DecisionContract', 'class DisplayHints']) {
          if (line.startsWith(name)) {
            homes.putIfAbsent(name, () => []).add(path);
          }
        }
      }
    }
    expect(homes['class DecisionContract'], [_canonicalContractHome]);
    expect(homes['class DisplayHints'], [_canonicalContractHome]);
  });

  test('the surviving contract still mirrors the backend wire keys', () {
    // Kalau rumah kanoniknya berubah bentuk, gate ini yang berteriak — bukan
    // dua parser yang diam-diam berbeda pendapat.
    final canonical = File(_canonicalContractHome).readAsStringSync();
    expect(canonical.contains("'secondary_actions'"), isTrue);
    expect(canonical.contains("'decision_version'"), isTrue);
    expect(canonical.contains("'time_remaining_seconds'"), isTrue);
  });

  test('the canonical payment wire parses without a decision object', () {
    final fixture =
        jsonDecode(
              File('test/fixtures/payment_wire_contract.json').readAsStringSync(),
            )
            as Map<String, dynamic>;
    final wire = Map<String, dynamic>.from(
      (fixture['get_payment'] as Map<String, dynamic>)['response']
          as Map<String, dynamic>,
    );
    expect(wire.containsKey('decision'), isFalse);

    final parsed = PaymentDto.fromJson(wire).toEntity();
    expect(parsed.status, PaymentStatus.pending);

    // Key asing tidak boleh menghidupkan authority kedua: backend tidak pernah
    // mengirimnya, jadi kehadirannya pun harus diabaikan, bukan diparse.
    final withStray = Map<String, dynamic>.from(wire)
      ..['decision'] = {'state': 'pending', 'allowed_actions': ['pay']};
    expect(PaymentDto.fromJson(withStray).toEntity().status, PaymentStatus.pending);
  });
}
