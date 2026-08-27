import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/core/utils/notification_navigation_handler.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/services/fcm_action_mapper.dart';
import 'package:labuda/domains/system/notification/services/fcm_message_handler.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

class _RecordingNavigationHandler implements NavigationHandler {
  String? lastChatConversationId;
  int chatConversationCalls = 0;

  @override
  void navigateToChatConversation(String conversationId) {
    lastChatConversationId = conversationId;
    chatConversationCalls += 1;
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

NotificationEntity _supportTicketNotification(
  NotificationType type, {
  String? ticketId,
  required String chatRoomId,
}) {
  final data = <String, dynamic>{'chatRoomId': chatRoomId};
  if (ticketId != null) {
    data['ticketId'] = ticketId;
  }

  return NotificationEntity(
    id: 'notif-1',
    userId: 'user-1',
    type: type,
    title: 'Support',
    body: 'Ticket update',
    data: data,
    isRead: false,
    createdAt: DateTime(2026, 6, 4),
  );
}

Widget _bannerSuppressionApp(String initialRoute) {
  return MaterialApp(
    initialRoute: initialRoute,
    onGenerateRoute: (settings) => MaterialPageRoute(
      settings: settings,
      builder: (context) => const Scaffold(body: Text('support-chat')),
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
          path: '/chat/:chatId',
          builder: (context, state) {
            final chatId = state.pathParameters['chatId']!;
            return Scaffold(body: Text('chat:$chatId'));
          },
        ),
        GoRoute(
          path: '/support/tickets/:ticketId',
          name: RouteNames.supportTicketThread,
          builder: (context, state) {
            final ticketId = state.pathParameters['ticketId']!;
            return Scaffold(body: Text('support-ticket:$ticketId'));
          },
        ),
      ],
    ),
  );
}

void main() {
  group('support.ticket_waiting_user FCM parity', () {
    testWidgets(
      'foreground suppression treats support tickets as thread/chat',
      (tester) async {
        await tester.pumpWidget(
          _bannerSuppressionApp('/support/tickets/ticket-123'),
        );

        final context = tester.element(find.text('support-chat'));

        expect(
          FCMMessageHandler.shouldSuppressBanner(
            context,
            'support.ticket.created',
            {'ticketId': 'ticket-123'},
          ),
          isTrue,
        );

        await tester.pumpWidget(_bannerSuppressionApp('/chat/chat-123'));
        final chatContext = tester.element(find.text('support-chat'));

        expect(
          FCMMessageHandler.shouldSuppressBanner(
            chatContext,
            'support.ticket_waiting_user',
            {'chatRoomId': 'chat-123'},
          ),
          isTrue,
        );

        expect(
          FCMMessageHandler.shouldSuppressBanner(
            chatContext,
            'support_ticket_created',
            {'chatRoomId': 'chat-123'},
          ),
          isTrue,
        );

        expect(
          FCMMessageHandler.shouldSuppressBanner(
            chatContext,
            'support_ticket_resolved',
            {'chatRoomId': 'chat-123'},
          ),
          isTrue,
        );

        expect(
          FCMMessageHandler.shouldSuppressBanner(
            chatContext,
            'totally.unknown.event',
            {'chatId': 'chat-123'},
          ),
          isFalse,
        );
      },
    );

    testWidgets(
      'push tap routes support ticket notifications to ticket thread',
      (tester) async {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final context = tester.element(find.text('home'));

        final handled = NotificationNavigationHandler.navigate(
          context: context,
          type: 'support.ticket.created',
          data: {'ticket_id': 'ticket-123', 'chatRoomId': 'chat-123'},
        );

        expect(handled, isTrue);

        await tester.pump(const Duration(milliseconds: 700));
        await tester.pumpAndSettle();

        expect(find.text('support-ticket:ticket-123'), findsOneWidget);
      },
    );

    testWidgets(
      'list tap routes ticketId to thread and chatRoomId to chat fallback',
      (tester) async {
        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final context = tester.element(find.text('home'));

        final createdHandled = NotificationNavigationHandler.navigate(
          context: context,
          type: 'support.ticket.resolved',
          data: {'ticketId': 'ticket-456', 'chatRoomId': 'chat-123'},
        );
        expect(createdHandled, isTrue);

        await tester.pump(const Duration(milliseconds: 700));
        await tester.pumpAndSettle();
        expect(find.text('support-ticket:ticket-456'), findsOneWidget);

        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final fallbackHandled = NotificationNavigationHandler.navigate(
          context: tester.element(find.text('home')),
          type: 'support.ticket.closed',
          data: {'chatRoomId': 'chat-456'},
        );
        expect(fallbackHandled, isTrue);

        await tester.pump(const Duration(milliseconds: 700));
        await tester.pumpAndSettle();
        expect(find.text('chat:chat-456'), findsOneWidget);
      },
    );

    testWidgets(
      'service routes support tickets to thread when ticketId is present',
      (tester) async {
        final service = NotificationNavigationService(
          _RecordingNavigationHandler(),
        );

        await tester.pumpWidget(_routerApp());
        await tester.pumpAndSettle();

        final context = tester.element(find.text('home'));

        await service.handleNotificationTap(
          context,
          _supportTicketNotification(
            NotificationType.supportTicketCreated,
            ticketId: 'ticket-1',
            chatRoomId: 'chat-room-1',
          ),
        );

        await tester.pumpAndSettle();

        expect(find.text('support-ticket:ticket-1'), findsOneWidget);
      },
    );

    testWidgets('service falls back to chatRoomId when ticketId is missing', (
      tester,
    ) async {
      final handler = _RecordingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('support-chat'))),
      );
      final context = tester.element(find.text('support-chat'));

      await service.handleNotificationTap(
        context,
        _supportTicketNotification(
          NotificationType.supportTicketWaitingUser,
          chatRoomId: 'chat-room-1',
        ),
      );

      expect(handler.lastChatConversationId, 'chat-room-1');
      expect(handler.chatConversationCalls, 1);
    });

    test('support.ticket_waiting_user expects no dedicated banner action', () {
      final actions = FCMActionMapper().getActionsForType(
        'support.ticket_waiting_user',
        {'chatRoomId': 'chat-123'},
      );

      expect(actions, isNull);
    });

    test('canonical wire string still maps to supportTicketWaitingUser', () {
      expect(
        NotificationType.fromString('support.ticket_waiting_user'),
        NotificationType.supportTicketWaitingUser,
      );
    });

    testWidgets('push handler safely rejects support tickets without ids', (
      tester,
    ) async {
      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      final handled = NotificationNavigationHandler.navigate(
        context: tester.element(find.text('home')),
        type: 'support.ticket.created',
        data: {},
      );

      expect(handled, isFalse);
      expect(find.text('home'), findsOneWidget);
    });
  });
}
