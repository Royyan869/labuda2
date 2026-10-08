// Order creation handoff contract — order creation and payment initiation are
// SEPARATE lifecycles.
//
// CANONICAL:
//   Checkout → POST /orders → Order Detail → pending_payment → Bayar Sekarang
//   → Payment WebView.
//
// Checkout owns ONLY order creation. It NEVER auto-initiates a payment and
// NEVER navigates to a payment surface. Order Detail owns the "Bayar Sekarang"
// action (the backend decision action) and its recovery.
//
// Positive proof: the handler creates the order and hands off to Order Detail.
// Negative proof: no automatic payment initiation / payment navigation / pay
// flags remain in the checkout scope.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _logic =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart';
const _impl =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart';
const _actionBar =
    'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_action_bar.dart';
const _router =
    'lib/domains/commerce/transaction/checkout/presentation/checkout_router_module.dart';
const _orderDetailHandlers =
    'lib/domains/commerce/transaction/order/presentation/screens/order_detail/order_detail_handlers.dart';

String _read(String path) => File(path).readAsStringSync();

void main() {
  final logic = _read(_logic);
  final impl = _read(_impl);
  final actionBar = _read(_actionBar);

  group('Checkout creates the order and hands off to Order Detail', () {
    test('the handler creates the order', () {
      expect(logic, contains('notifier.createOrder(request)'));
    });

    test('after creation it navigates to the canonical Order Detail route', () {
      expect(logic, contains('pushReplacement'));
      expect(
        logic,
        contains('RoutePaths.orderDetailPath(orderResponse.orderId)'),
      );
    });
  });

  group('Checkout never initiates or navigates to payment', () {
    test('no payment initiation in the order handler', () {
      expect(logic, isNot(contains('initiatePayment')));
      expect(logic, isNot(contains('paymentInitiationProvider')));
      expect(logic, isNot(contains('InitiatePaymentRequest')));
    });

    test('no payment navigation from checkout', () {
      expect(logic, isNot(contains('/payment-webview')));
      expect(logic, isNot(contains('/payment-result')));
      expect(logic, isNot(contains('returnToChat')));
    });

    test('checkout no longer observes the payment notifier', () {
      expect(impl, isNot(contains('paymentInitiationProvider')));
      expect(impl, isNot(contains('isInitiatingPayment')));
      expect(actionBar, isNot(contains('isInitiatingPayment')));
    });

    test('no auto-payment configuration flags were introduced', () {
      for (final flag in const [
        'autoPay',
        'skipPayment',
        'shouldInitiatePayment',
        'enableAutoPayment',
      ]) {
        expect(logic, isNot(contains(flag)), reason: 'checkout logic: $flag');
        expect(impl, isNot(contains(flag)), reason: 'checkout screen: $flag');
      }
    });
  });

  group('Order Detail owns the canonical payment entry point', () {
    test('the pending-order Pay action still initiates payment', () {
      expect(_read(_orderDetailHandlers), contains('initiatePayment('));
      expect(_read(_orderDetailHandlers), contains('/payment-webview'));
    });
  });

  group('negative residue', () {
    test('checkout router no longer reads return_to_chat', () {
      expect(_read(_router), isNot(contains('return_to_chat')));
      expect(_read(_router), isNot(contains('returnToChat')));
    });
  });
}
