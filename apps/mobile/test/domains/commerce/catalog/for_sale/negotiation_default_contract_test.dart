// ForSale negotiation defaults — owner decision 2026-10-02: listing dibuat
// TANPA NEGO secara default; edit screen meng-hydrate flag nego dari kebenaran
// server, bukan diturunkan dari harga.
//
// KILL ONCE / LOCK FOREVER — designs that must never come back:
//   - create default `_isNegotiable = true` (desain lama);
//   - hydrate `_isNegotiable = forSale.price > 0` (desain lama, mengabaikan
//     `forSale.isNegotiable`).
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const String _createScreenPath =
    'lib/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart';
const String _editScreenPath =
    'lib/domains/commerce/catalog/for_sale/presentation/screens/edit_for_sale_screen.dart';

void main() {
  group('ForSale negotiation default (source contract)', () {
    test('create screen defaults to NO negotiation', () {
      final source = File(_createScreenPath).readAsStringSync();

      expect(source, contains('bool _isNegotiable = false;'));
      expect(source, isNot(contains('bool _isNegotiable = true;')));
    });

    test('edit screen hydrates negotiation from server truth, not price', () {
      final source = File(_editScreenPath).readAsStringSync();

      expect(source, contains('_isNegotiable = forSale.isNegotiable;'));
      expect(source, isNot(contains('forSale.price > 0')));
    });
  });
}
