import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/utils/notification_navigation_handler.dart';
import 'package:labuda/domains/system/notification/services/fcm_action_mapper.dart';
import 'package:labuda/domains/system/notification/services/fcm_message_handler.dart';

Widget _orderRouteApp() {
  return MaterialApp(
    initialRoute: '/orders/order-123',
    onGenerateRoute: (settings) => MaterialPageRoute(
      settings: settings,
      builder: (context) => const Scaffold(body: Text('order-detail')),
    ),
  );
}

Widget _routerApp() {
  return MaterialApp.router(
    routerConfig: GoRouter(
      navigatorKey: navigatorKey,
      initialLocation: '/home',
      routes: [
        GoRoute(
          path: '/home',
          builder: (context, state) => const Scaffold(body: Text('home')),
        ),
        GoRoute(
          path: '/orders/:orderId',
          builder: (context, state) {
            final orderId = state.pathParameters['orderId']!;
            return Scaffold(body: Text('order:$orderId'));
          },
        ),
      ],
    ),
  );
}

void main() {
  group('Order/refund canonical FCM parity', () {
    testWidgets(
      'foreground suppression recognizes canonical order/refund types',
      (tester) async {
        await tester.pumpWidget(_orderRouteApp());
        final context = tester.element(find.text('order-detail'));

        for (final type in <String>[
          'order.created',
          'order.created.buyer',
          'order.paid',
          'order.paid.buyer',
          'order.shipped',
          'order_delivered',
          'order.completed',
          'order.cancelled',
          'order.cancelled_timeout',
          'order.expired',
          'order.refunded',
          'order.partially_refunded',
          'order.dispute_open',
          'order.confirmation_extended',
          'refund.opened',
          'refund.approved',
          'refund.rejected',
          'refund.escalated',
        ]) {
          expect(
            FCMMessageHandler.shouldSuppressBanner(context, type, {
              'orderId': 'order-123',
            }),
            isTrue,
            reason: '$type should be suppressed on the matching order screen',
          );
        }
      },
    );

    test(
      'canonical action mapping matches legacy UX for order/refund types',
      () {
        final mapper = FCMActionMapper();

        final orderCreated = mapper.getActionsForType('order.created', {
          'orderId': 'order-123',
        });
        final orderCreatedBuyer = mapper.getActionsForType(
          'order.created.buyer',
          {'orderId': 'order-123'},
        );
        final orderPaid = mapper.getActionsForType('order.paid', {
          'orderId': 'order-123',
        });
        final orderPaidBuyer = mapper.getActionsForType('order.paid.buyer', {
          'orderId': 'order-123',
        });
        final orderShipped = mapper.getActionsForType('order.shipped', {
          'orderId': 'order-123',
        });
        final refundOpened = mapper.getActionsForType('refund.opened', {
          'orderId': 'order-123',
        });
        final refundApproved = mapper.getActionsForType('refund.approved', {
          'orderId': 'order-123',
        });
        final refundRejected = mapper.getActionsForType('refund.rejected', {
          'orderId': 'order-123',
        });

        expect(orderCreated, isNotNull);
        expect(orderCreated!.single.label, 'Lihat Order');
        expect(orderCreatedBuyer, isNotNull);
        expect(orderCreatedBuyer!.single.label, 'Lihat Order');
        expect(orderPaid, isNotNull);
        expect(orderPaid!.single.label, 'Lihat Order');
        expect(orderPaidBuyer, isNotNull);
        expect(orderPaidBuyer!.single.label, 'Lihat Order');
        expect(orderShipped, isNotNull);
        expect(orderShipped!.single.label, 'Lacak Paket');
        expect(refundOpened, isNotNull);
        expect(refundOpened!.single.label, 'Lihat Detail');
        expect(refundApproved, isNotNull);
        expect(refundApproved!.single.label, 'Lihat Detail');
        expect(refundRejected, isNotNull);
        expect(refundRejected!.single.label, 'Lihat Detail');
      },
    );

    testWidgets('canonical and legacy tap routing both land on order detail', (
      tester,
    ) async {
      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      final context = tester.element(find.text('home'));

      final canonicalHandled = NotificationNavigationHandler.navigate(
        context: context,
        type: 'order.created',
        data: {'orderId': 'order-123'},
      );
      expect(canonicalHandled, isTrue);

      await tester.pump(const Duration(milliseconds: 700));
      await tester.pumpAndSettle();
      expect(find.text('order:order-123'), findsOneWidget);

      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      final refundHandled = NotificationNavigationHandler.navigate(
        context: tester.element(find.text('home')),
        type: 'refund.opened',
        data: {'orderId': 'order-456'},
      );
      expect(refundHandled, isTrue);

      await tester.pump(const Duration(milliseconds: 700));
      await tester.pumpAndSettle();
      expect(find.text('order:order-456'), findsOneWidget);

      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      final legacyHandled = NotificationNavigationHandler.navigate(
        context: tester.element(find.text('home')),
        type: 'refund_requested',
        data: {'orderId': 'order-789'},
      );
      expect(legacyHandled, isTrue);

      await tester.pump(const Duration(milliseconds: 700));
      await tester.pumpAndSettle();
      expect(find.text('order:order-789'), findsOneWidget);
    });

    test('legacy alias compatibility stays intact for preserved aliases', () {
      final mapper = FCMActionMapper();

      expect(
        mapper.getActionsForType('order_created', {'orderId': 'order-123'}),
        isNotNull,
      );
      expect(
        mapper.getActionsForType('order_confirmed', {'orderId': 'order-123'}),
        isNotNull,
      );
      expect(
        mapper.getActionsForType('refund_requested', {'orderId': 'order-123'}),
        isNotNull,
      );
      expect(
        mapper.getActionsForType('refund_processed', {'orderId': 'order-123'}),
        isNotNull,
      );
    });

    test('canonical type parser resolves order/refund wire strings', () {
      expect(
        NotificationType.fromString('order.created'),
        NotificationType.orderCreated,
      );
      expect(
        NotificationType.fromString('order.paid'),
        NotificationType.orderPaid,
      );
      expect(
        NotificationType.fromString('refund.opened'),
        NotificationType.refundOpened,
      );
      expect(
        NotificationType.fromString('refund.escalated'),
        NotificationType.refundEscalated,
      );
    });
  });
}
