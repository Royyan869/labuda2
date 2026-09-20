// R4 — Checkout seller-chat canonical contract.
//
// Mirrors `order_chat_canonical_contract_test.dart`. Checkout is PRE-ORDER, so
// the canonical authority is the Chat domain's `openCommerceChat` (room
// resolution + canonical `/chat/<room-id>` navigation + product reference), NOT
// `openOrderCommerceChat` (which links an order into the room).
//
// This is a NEGATIVE contract: it fails if checkout reintroduces a competing
// chat authority — direct repository orchestration, its own room logic, or the
// legacy `/chat?userId=` query.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String relativePath) => File(relativePath).readAsStringSync();

void main() {
  test('checkout opens seller chat through the canonical Chat authority', () {
    final screen = _read(
      'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
    );

    expect(
      screen.contains('openCommerceChat('),
      isTrue,
      reason: 'checkout must delegate room resolution to the canonical helper',
    );
    expect(
      screen.contains('chatRepositoryProvider'),
      isFalse,
      reason: 'checkout must not orchestrate the chat repository directly',
    );
    expect(
      screen.contains('getOrCreateChat('),
      isFalse,
      reason: 'checkout must not resolve chat rooms itself',
    );
    expect(
      screen.contains('linkOrderToChat('),
      isFalse,
      reason: 'checkout is pre-order: it never links an order to a room',
    );
    expect(
      screen.contains('/chat?userId='),
      isFalse,
      reason: 'legacy direct chat query must not return',
    );
  });

  test('no checkout file carries a legacy chat path', () {
    final checkoutLib = Directory('lib/domains/commerce/transaction/checkout')
        .listSync(recursive: true)
        .whereType<File>()
        .where((f) => f.path.endsWith('.dart'));

    for (final file in checkoutLib) {
      final text = file.readAsStringSync();
      expect(
        text.contains('/chat?userId='),
        isFalse,
        reason: 'legacy direct chat query still present in ${file.path}',
      );
      expect(
        text.contains("push('/chat/"),
        isFalse,
        reason: 'checkout must not build chat routes itself (${file.path})',
      );
    }
  });
}
