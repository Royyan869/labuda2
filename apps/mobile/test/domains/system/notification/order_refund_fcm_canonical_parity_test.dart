import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/notification/services/fcm_action_mapper.dart';
import 'package:hishumi/domains/system/notification/services/fcm_message_handler.dart';
import 'package:hishumi/domains/system/notification/services/notification_navigation_service.dart';

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

    testWidgets('order and refund wire types land on order detail directly', (
      tester,
    ) async {
      final service = NotificationNavigationService.canonical();

      for (final caseEntry in [
        ('order.created', 'order-123'),
        ('refund.opened', 'order-456'),
        ('order.refunded', 'order-789'),
      ]) {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        await service.handleNotificationPayload(
          tester.element(find.text('home')),
          type: caseEntry.$1,
          data: {'orderId': caseEntry.$2},
        );

        await tester.pumpAndSettle();
        expect(
          find.text('order:${caseEntry.$2}'),
          findsOneWidget,
          reason: '${caseEntry.$1} must open the order directly',
        );
      }
    });

    test('purged legacy aliases expose no banner action and no catalog entry', () {
      final mapper = FCMActionMapper();

      for (final alias in [
        'order_created',
        'order_confirmed',
        'order_shipped',
        'order_delivered',
        'refund_requested',
        'refund_processed',
      ]) {
        expect(
          mapper.getActionsForType(alias, {'orderId': 'order-123'}),
          isNull,
          reason: '$alias is not a canonical wire type',
        );
        expect(
          NotificationType.tryFromString(alias),
          isNull,
          reason: '$alias must not resolve in the canonical catalog',
        );
      }
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
