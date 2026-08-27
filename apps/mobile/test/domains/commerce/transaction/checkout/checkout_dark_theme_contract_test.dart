import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('checkout presentation uses semantic theme surfaces and text colors', () {
    final screenSource = File(
      'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
    ).readAsStringSync();
    final summarySource = File(
      'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart',
    ).readAsStringSync();
    final shippingSource = File(
      'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_shipping_section.dart',
    ).readAsStringSync();

    expect(screenSource, contains('colorScheme.surface'));
    expect(summarySource, contains('colorScheme.surface'));
    expect(summarySource, contains('colorScheme.onSurfaceVariant'));
    expect(shippingSource, contains('colorScheme.surface'));
    expect(shippingSource, contains('colorScheme.onSurfaceVariant'));
  });
}
