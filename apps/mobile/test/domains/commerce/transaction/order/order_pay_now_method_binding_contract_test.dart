// Post-order payment method convergence (Phase 2 final).
//
// A BOUND order (order.paymentMethodCode != null) must Pay Now with that exact
// method — never offer alternatives the backend will reject. Only an UNBOUND
// order (auction-claim) may select a method for the first time.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String path) => File(path).readAsStringSync();

const _handlers =
    'lib/domains/commerce/transaction/order/presentation/screens/order_detail/order_detail_handlers.dart';
const _mapper =
    'lib/domains/commerce/transaction/order/data/mappers/order_mapper.dart';
const _card =
    'lib/domains/commerce/transaction/order/presentation/widgets/order_payment_info_card.dart';

void main() {
  test('bound order Pay Now uses the bound method; picker only for unbound', () {
    final handlers = _read(_handlers);

    expect(
      handlers,
      contains('String? selectedMethodCode = order.paymentMethodCode;'),
      reason: 'the bound method must be the default selection',
    );

    // The order-scoped method list is fetched ONLY after the bound check, i.e.
    // only when the order has no bound method (auction-claim first selection).
    final boundIdx = handlers.indexOf(
      'String? selectedMethodCode = order.paymentMethodCode;',
    );
    final fetchIdx = handlers.indexOf('getPaymentMethodOptions(');
    expect(fetchIdx, greaterThan(boundIdx),
        reason: 'a bound order must never fetch/offer alternative methods');

    // The selected (bound) method is what is sent to payment creation.
    expect(handlers, contains('paymentMethodCode: selectedMethodCode'));
  });

  test('order mapper carries the canonical bound code (no hardcoded method)', () {
    final mapper = _read(_mapper);
    expect(mapper, contains('paymentMethodCode: dto.paymentMethodCode'));
    expect(
      mapper,
      isNot(contains('PaymentMethodType.bankTransfer')),
      reason: 'the hardcoded payment method is a lie and must stay purged',
    );
  });

  test('order payment info card reads the canonical bound code', () {
    final card = _read(_card);
    expect(card, contains('order.paymentMethodCode'));
    expect(card, isNot(contains('order.paymentMethod)')));
  });
}
