import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/core/utils/notification_navigation_handler.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';
import 'package:labuda/domains/system/notification/domain/services/notification_display_service.dart';
import 'package:labuda/domains/system/notification/services/fcm_action_mapper.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

NotificationEntity _notification({
  required NotificationType type,
  required Map<String, dynamic> data,
}) {
  return NotificationEntity(
    id: 'notif-1',
    userId: 'user-1',
    type: type,
    title: 'Title',
    body: 'Body',
    data: data,
    isRead: false,
    createdAt: DateTime(2026, 6, 4),
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
          path: '/forSale/:fixedPriceSaleId',
          builder: (context, state) {
            final fixedPriceSaleId = state.pathParameters['forSaleId']!;
            return Scaffold(body: Text('forSale:$fixedPriceSaleId'));
          },
        ),
        GoRoute(
          path: '/orders/:orderId',
          builder: (context, state) {
            final orderId = state.pathParameters['orderId']!;
            return Scaffold(body: Text('order:$orderId'));
          },
        ),
        GoRoute(
          path: '/seller/promotions/:contractId/analytics',
          builder: (context, state) {
            final contractId = state.pathParameters['contractId']!;
            return Scaffold(body: Text('analytics:$contractId'));
          },
        ),
        GoRoute(
          path: '/security',
          builder: (context, state) => const Scaffold(body: Text('security')),
        ),
      ],
    ),
  );
}

class _NoopNavigationHandler implements NavigationHandler {
  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('legacy notification cleanup', () {
    test('removed enum strings now fall back to announcement', () {
      expect(
        NotificationType.fromString('payment_success'),
        NotificationType.announcement,
      );
      expect(
        NotificationType.fromString('payment_failed'),
        NotificationType.announcement,
      );
      expect(
        NotificationType.fromString('payment_pending'),
        NotificationType.announcement,
      );
      expect(
        NotificationType.fromString('security_alert'),
        NotificationType.announcement,
      );
      expect(
        NotificationType.fromString('login_from_new_device'),
        NotificationType.announcement,
      );
    });

    test('announcement, promotion, and system maintenance remain intact', () {
      expect(NotificationType.announcement.value, 'announcement');
      expect(NotificationType.promotion.value, 'promotion');
      expect(NotificationType.systemMaintenance.value, 'system_maintenance');

      const display = NotificationDisplayService();
      expect(
        display.getDisplayMetadata(NotificationType.announcement).icon,
        NotificationDisplayIcon.campaign,
      );
      expect(
        display.getDisplayMetadata(NotificationType.promotion).icon,
        NotificationDisplayIcon.campaign,
      );
      expect(
        display.getDisplayMetadata(NotificationType.systemMaintenance).icon,
        NotificationDisplayIcon.build,
      );
    });

    test(
      'notification filter no longer treats security aliases as a support bucket',
      () {
        expect(
          NotificationFilter.support.matches(
            NotificationType.supportTicketCreated,
          ),
          isTrue,
        );
        expect(
          NotificationFilter.support.matches(
            NotificationType.supportTicketResolved,
          ),
          isTrue,
        );
        expect(
          NotificationFilter.support.matches(NotificationType.announcement),
          isFalse,
        );
      },
    );

    testWidgets(
      'navigation handler returns no-op for removed strings and collection_recommendation',
      (tester) async {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final context = tester.element(find.text('home'));

        for (final caseEntry in [
          ('payment_success', {'orderId': 'order-1'}),
          ('payment_failed', {'orderId': 'order-1'}),
          ('payment_pending', {'orderId': 'order-1'}),
          ('security_alert', {'orderId': 'order-1'}),
          ('login_from_new_device', {'orderId': 'order-1'}),
          ('collection_recommendation', {'forSaleId': 'forSale-1'}),
        ]) {
          final handled = NotificationNavigationHandler.navigate(
            context: context,
            type: caseEntry.$1,
            data: caseEntry.$2,
          );

          expect(handled, isFalse, reason: '${caseEntry.$1} should no-op');
        }

        expect(find.text('home'), findsOneWidget);
        expect(find.textContaining('forSale:'), findsNothing);
        expect(find.textContaining('order:'), findsNothing);
      },
    );

    testWidgets(
      'announcement and system maintenance still route as no-op modals',
      (tester) async {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final context = tester.element(find.text('home'));
        final service = NotificationNavigationService(_NoopNavigationHandler());

        await service.handleNotificationTap(
          context,
          _notification(type: NotificationType.announcement, data: const {}),
        );
        await service.handleNotificationTap(
          context,
          _notification(
            type: NotificationType.systemMaintenance,
            data: const {},
          ),
        );

        expect(find.text('home'), findsOneWidget);
      },
    );

    testWidgets(
      'promotion routes to contract analytics when contractId exists',
      (tester) async {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final context = tester.element(find.text('home'));
        final handled = NotificationNavigationHandler.navigate(
          context: context,
          type: 'promotion',
          data: {'contractId': 'contract-9'},
        );

        expect(handled, isTrue);

        await tester.pump(const Duration(milliseconds: 700));
        await tester.pumpAndSettle();

        expect(find.text('analytics:contract-9'), findsOneWidget);
      },
    );

    test('FCM action mapper no longer returns actions for removed strings', () {
      final mapper = FCMActionMapper();

      expect(
        mapper.getActionsForType('payment_success', {'orderId': 'order-1'}),
        isNull,
      );
      expect(
        mapper.getActionsForType('payment_failed', {'orderId': 'order-1'}),
        isNull,
      );
      expect(
        mapper.getActionsForType('security_alert', {'orderId': 'order-1'}),
        isNull,
      );
    });
  });
}
