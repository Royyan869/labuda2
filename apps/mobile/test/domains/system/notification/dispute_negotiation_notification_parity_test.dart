import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/data/mappers/notification_api_mapper.dart';
import 'package:labuda/domains/system/notification/data/models/api/notification_api_models.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/services/notification_display_service.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

class _RecordingNavigationHandler implements NavigationHandler {
  String? lastOrderId;
  String? lastChatConversationId;

  @override
  void navigateToOrderDetail(String orderId) {
    lastOrderId = orderId;
  }

  @override
  void navigateToChatConversation(String conversationId) {
    lastChatConversationId = conversationId;
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
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
        GoRoute(
          path: '/chat/:chatId',
          builder: (context, state) {
            final chatId = state.pathParameters['chatId']!;
            return Scaffold(body: Text('chat:$chatId'));
          },
        ),
      ],
    ),
  );
}

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

void main() {
  group('dispute.opened parity', () {
    test('parser and API mapper resolve dispute.opened canonically', () {
      expect(
        NotificationType.fromString('dispute.opened'),
        NotificationType.disputeOpened,
      );

      final response = NotificationResponse(
        id: 'notif-1',
        userId: 'user-1',
        type: 'dispute.opened',
        title: 'Dispute opened',
        body: 'A dispute was opened',
        data: {'orderId': 'order-123'},
        isRead: false,
        createdAt: DateTime(2026, 6, 4),
      );

      final entity = NotificationApiMapper.toEntity(response);

      expect(entity.type, NotificationType.disputeOpened);
    });

    test('display metadata uses dispute-specific icon and color', () {
      final metadata = const NotificationDisplayService().getDisplayMetadata(
        NotificationType.disputeOpened,
      );

      expect(metadata.icon, NotificationDisplayIcon.gavel);
      expect(metadata.color, NotificationDisplayColor.deepOrange);
    });

    testWidgets('list tap and push/open both route dispute.opened to order', (
      tester,
    ) async {
      final handler = _RecordingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notification-list'))),
      );
      final context = tester.element(find.text('notification-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.disputeOpened,
          data: {'orderId': 'order-123'},
        ),
      );

      expect(handler.lastOrderId, 'order-123');

      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      await NotificationNavigationService.canonical()
          .handleNotificationPayload(
            tester.element(find.text('home')),
            type: 'dispute.opened',
            data: {'orderId': 'order-456'},
          );

      await tester.pumpAndSettle();

      expect(find.text('order:order-456'), findsOneWidget);
    });
  });

  group('negotiation.cancelled parity', () {
    testWidgets('list tap and push/open route to chatRoomId', (tester) async {
      final handler = _RecordingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notification-list'))),
      );
      final context = tester.element(find.text('notification-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.negotiationCancelled,
          data: {'chatRoomId': 'chat-room-123'},
        ),
      );

      expect(handler.lastChatConversationId, 'chat-room-123');

      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      await NotificationNavigationService.canonical()
          .handleNotificationPayload(
            tester.element(find.text('home')),
            type: 'negotiation.cancelled',
            data: {'chatRoomId': 'chat-room-456'},
          );

      await tester.pumpAndSettle();

      expect(find.text('chat:chat-room-456'), findsOneWidget);
    });
  });

  // PASS_8A / F1: negotiation.started, negotiation.message_sent,
  // negotiation.accepted, and negotiation.expired are canonical types with one
  // destination decision (NotificationNavigationService). This group locks the
  // push/open path to the same chat destination as the in-app list — and to the
  // same "stay put" behaviour when the payload carries no chat room id.
  group('negotiation push/open parity (PASS_8A / F1)', () {
    for (final type in [
      'negotiation.started',
      'negotiation.message_sent',
      'negotiation.accepted',
      'negotiation.expired',
    ]) {
      testWidgets('$type routes to chatRoomId via the canonical service', (
        tester,
      ) async {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        await NotificationNavigationService.canonical()
            .handleNotificationPayload(
              tester.element(find.text('home')),
              type: type,
              data: {'chatRoomId': 'chat-room-789'},
            );

        await tester.pumpAndSettle();

        expect(
          find.text('chat:chat-room-789'),
          findsOneWidget,
          reason: '$type must open the negotiation chat',
        );
      });
    }

    testWidgets('missing chatRoomId stays put instead of crashing', (
      tester,
    ) async {
      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      await NotificationNavigationService.canonical()
          .handleNotificationPayload(
            tester.element(find.text('home')),
            type: 'negotiation.started',
            data: const {},
          );

      await tester.pumpAndSettle();
      expect(find.text('home'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  group('negotiation.started/message_sent/accepted/expired list parity', () {
    for (final entry in {
      NotificationType.negotiationStarted: 'negotiation.started',
      NotificationType.negotiationMessageSent: 'negotiation.message_sent',
      NotificationType.negotiationAccepted: 'negotiation.accepted',
      NotificationType.negotiationExpired: 'negotiation.expired',
    }.entries) {
      testWidgets('${entry.value} list tap routes to chatRoomId', (
        tester,
      ) async {
        final handler = _RecordingNavigationHandler();
        final service = NotificationNavigationService(handler);

        await tester.pumpWidget(
          const MaterialApp(home: Scaffold(body: Text('notification-list'))),
        );
        final context = tester.element(find.text('notification-list'));

        await service.handleNotificationTap(
          context,
          _notification(
            type: entry.key,
            data: {'chatRoomId': 'chat-room-list-${entry.value}'},
          ),
        );

        expect(handler.lastChatConversationId, 'chat-room-list-${entry.value}');
      });
    }
  });
}
