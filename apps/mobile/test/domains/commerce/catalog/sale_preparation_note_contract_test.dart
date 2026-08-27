import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('create listing screen keeps Waktu Persiapan and removes Catatan Persiapan', () {
    final source = File(
      'lib/domains/commerce/catalog/listing/presentation/screens/create_listing_screen.dart',
    ).readAsStringSync();

    expect(source, contains('Waktu Persiapan *'));
    expect(source, isNot(contains('Catatan Persiapan')));
    expect(source, isNot(contains('_preparationNoteController')));
    expect(source, isNot(contains('preparationNote:')));
  });

  test('create auction screen keeps Waktu Persiapan and removes Catatan Persiapan', () {
    final source = File(
      'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
    ).readAsStringSync();

    expect(source, contains('Opsi Pengiriman *'));
    expect(source, isNot(contains('Catatan Persiapan')));
    expect(source, isNot(contains('_preparationNoteController')));
    expect(source, isNot(contains('preparationNote:')));
  });
}
