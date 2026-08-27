import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String relativePath) {
  return File(relativePath).readAsStringSync();
}

void main() {
  test('order chat entry points use the canonical helper and no legacy query', () {
    final orderDetail = _read(
      'lib/domains/commerce/transaction/order/presentation/screens/order_detail_screen.dart',
    );
    final shippingCard = _read(
      'lib/domains/commerce/transaction/order/presentation/widgets/order_shipping_info_card.dart',
    );
    final widgetImpl = _read(
      'lib/domains/commerce/transaction/order/presentation/widgets/order_widgets_impl.dart',
    );

    expect(orderDetail.contains('openOrderCommerceChat('), isTrue);
    expect(shippingCard.contains('openOrderCommerceChat('), isTrue);
    expect(
      widgetImpl.contains(r"context.go('/chat/${roomResult.data!.id}')"),
      isTrue,
    );

    final orderLib = Directory(
      'lib/domains/commerce/transaction/order',
    ).listSync(recursive: true).whereType<File>();
    for (final file in orderLib) {
      final text = file.readAsStringSync();
      expect(
        text.contains('/chat?userId='),
        isFalse,
        reason: 'legacy direct chat query still present in ${file.path}',
      );
    }
  });
}
