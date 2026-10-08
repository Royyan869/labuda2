// NAVIGATION CONVERGENCE CONTRACT — PASS 2.
//
// Locks the two behavioural Owner decisions that cannot be exercised through
// the real WebView in a unit test:
//   - payment CANCELLATION returns to the originating surface (order detail),
//     never to the payment-result polling screen;
//   - Coins remain canonical and reachable from Settings.
//
// The GoRouter result-passing mechanism itself is also proven directly.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  group('Payment cancellation returns to origin', () {
    test('payment WebView returns a completion result', () {
      final src = File(
        'lib/domains/finance/transaction/payment/presentation/screens/'
        'payment_webview_screen.dart',
      ).readAsStringSync();

      expect(src, contains('context.pop(true)')); // gateway finish reached
      expect(src, contains('context.pop(false)')); // explicit cancel/close
    });

    test('checkout hands off to Order Detail and never initiates payment', () {
      final src = File(
        'lib/domains/commerce/transaction/checkout/presentation/screens/'
        'checkout_screen_logic.dart',
      ).readAsStringSync();

      // Checkout owns ONLY order creation: it hands the created order to the
      // canonical Order Detail surface and never touches a payment surface.
      expect(src, contains('RoutePaths.orderDetailPath'));
      expect(src, isNot(contains('/payment-result')));
      expect(src, isNot(contains('/payment-webview')));
      expect(src, isNot(contains('initiatePayment')));
    });

    test('order detail reaches payment-result only after completion', () {
      final src = File(
        'lib/domains/commerce/transaction/order/presentation/screens/'
        'order_detail/order_detail_handlers.dart',
      ).readAsStringSync();

      expect(src, contains('push<bool>'));
      expect(src, contains('completed != true'));
    });

    testWidgets(
      'GoRouter push<bool> delivers the pop result; cancel never reaches result',
      (tester) async {
        bool? received;

        final router = GoRouter(
          initialLocation: '/',
          routes: [
            GoRoute(
              path: '/',
              builder: (context, state) => Scaffold(
                body: Builder(
                  builder: (context) => TextButton(
                    onPressed: () async {
                      received = await context.push<bool>('/payment-webview');
                      if (received == true && context.mounted) {
                        context.push('/payment-result/1');
                      }
                    },
                    child: const Text('start'),
                  ),
                ),
              ),
            ),
            GoRoute(
              path: '/payment-webview',
              builder: (context, state) => Scaffold(
                body: TextButton(
                  onPressed: () => context.pop(false),
                  child: const Text('cancel'),
                ),
              ),
            ),
            GoRoute(
              path: '/payment-result/:orderId',
              builder: (context, state) => const Scaffold(body: Text('result')),
            ),
          ],
        );

        await tester.pumpWidget(MaterialApp.router(routerConfig: router));
        await tester.tap(find.text('start'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('cancel'));
        await tester.pumpAndSettle();

        expect(received, isFalse);
        expect(
          find.text('result'),
          findsNothing,
          reason: 'cancellation must not land on the payment-result surface',
        );
      },
    );
  });

  group('Coins canonical + reachable', () {
    test('settings exposes the canonical /coins entry point', () {
      final src = File(
        'lib/domains/user/profile/presentation/screens/settings_screen.dart',
      ).readAsStringSync();

      expect(src, contains('RoutePaths.coins'));
    });
  });
}
