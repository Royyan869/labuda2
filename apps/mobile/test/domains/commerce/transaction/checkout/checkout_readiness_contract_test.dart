import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('checkout preview awaits the canonical future and guards stale data', () {
    final source = File(
      'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart',
    ).readAsStringSync();

    expect(source, contains('orderPreviewProvider(previewParams).future'));
    expect(
      source,
      contains('requestSignature != state._buildCheckoutSignature()'),
    );
    expect(source, contains('state._previewSignature = requestSignature;'));
    expect(source, contains('state._previewRefreshQueued = true;'));
  });

  test('checkout readiness copy distinguishes prerequisites from loading', () {
    final source = File(
      'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
    ).readAsStringSync();

    expect(
      source,
      contains('Pilih opsi pengiriman untuk memuat harga dari server.'),
    );
    expect(
      source,
      contains('Harga perlu diperbarui karena detail checkout berubah.'),
    );
    expect(
      source,
      contains('Lengkapi data checkout untuk memuat harga dari server.'),
    );
  });
}
