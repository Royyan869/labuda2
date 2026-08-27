library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/core/utils/notification_navigation_handler.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

class _RecordingNavigationHandler implements NavigationHandler {
  int sellerVerificationCalls = 0;
  int sellerDashboardCalls = 0;
  String? lastExternalProductId;
  String? lastContentId;

  @override
  void navigateToSellerVerification() {
    sellerVerificationCalls += 1;
  }

  @override
  void navigateToSellerDashboard() {
    sellerDashboardCalls += 1;
  }

  @override
  void navigateToExternalProductDetail(String productId) {
    lastExternalProductId = productId;
  }

  @override
  void navigateToContentDetail(String contentId) {
    lastContentId = contentId;
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

NotificationEntity _notification({
  required NotificationType type,
  Map<String, dynamic> data = const {},
}) {
  return NotificationEntity(
    id: 'notif-p2-1',
    userId: 'user-1',
    type: type,
    title: 'Title',
    body: 'Body',
    data: data,
    isRead: false,
    createdAt: DateTime(2026, 6, 18),
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
          path: '/content/:contentId',
          builder: (context, state) {
            final contentId = state.pathParameters['contentId']!;
            return Scaffold(body: Text('content:$contentId'));
          },
        ),
        GoRoute(
          path: '/seller/dashboard',
          builder: (context, state) =>
              const Scaffold(body: Text('seller-dashboard')),
        ),
        GoRoute(
          path: '/verification/seller',
          builder: (context, state) =>
              const Scaffold(body: Text('seller-verification')),
        ),
        GoRoute(
          path: '/seller/promotions/external-products/:productId',
          builder: (context, state) {
            final productId = state.pathParameters['productId']!;
            return Scaffold(body: Text('external-product:$productId'));
          },
        ),
        GoRoute(
          path: '/seller/promotions/:instanceId',
          builder: (context, state) {
            final instanceId = state.pathParameters['instanceId']!;
            return Scaffold(body: Text('promotion:$instanceId'));
          },
        ),
      ],
    ),
  );
}

void main() {
  group('notification deeplink P2 contract', () {
    testWidgets('service routes seller verification and tier notifications', (
      tester,
    ) async {
      final handler = _RecordingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notifications'))),
      );
      final context = tester.element(find.text('notifications'));

      await service.handleNotificationTap(
        context,
        _notification(type: NotificationType.sellerVerificationApproved),
      );
      expect(handler.sellerVerificationCalls, 1);

      await service.handleNotificationTap(
        context,
        _notification(type: NotificationType.sellerTierUpgraded),
      );
      expect(handler.sellerDashboardCalls, 1);
    });

    testWidgets('service routes promotion payloads to useful targets', (
      tester,
    ) async {
      final handler = _RecordingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();
      final context = tester.element(find.text('home'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.promotion,
          data: {'externalProductId': 'ep-123'},
        ),
      );
      expect(handler.lastExternalProductId, 'ep-123');

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.promotion,
          data: {'promotionInstanceId': 'promo-123'},
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('promotion:promo-123'), findsOneWidget);
    });

    testWidgets('handler routes comment targetType=comment to parent content', (
      tester,
    ) async {
      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      final handled = NotificationNavigationHandler.navigate(
        context: tester.element(find.text('home')),
        type: 'comment',
        data: {
          'targetType': 'comment',
          'parent_content_id': 'content-123',
          'comment_id': 'comment-999',
        },
      );

      expect(handled, isTrue);

      await tester.pump(const Duration(milliseconds: 700));
      await tester.pumpAndSettle();

      expect(find.text('content:content-123'), findsOneWidget);
    });
  });
}
