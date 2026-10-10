// TASK_6A — widget-level E2E wiring proof: the PRODUCTION
// NotificationInitializer (mounted once in the app builder chain) keeps the
// realtime sync consumer alive, so a `notification.created` frame on the
// existing WebSocket session converges the canonical badge/list providers.
//
// Kept separate from the plain-test suite so a flaky widget-runner hang can
// never mask the core behavior proofs.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/core/websocket/websocket_message.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:hishumi/domains/system/notification/presentation/widgets/notification_initializer.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

const _userId = 'u1';

class _ScriptedNotificationRepository implements INotificationRepository {
  _ScriptedNotificationRepository({this.unread = 0});

  int unread;
  int countSubscriptions = 0;

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countSubscriptions++;
    return Result.success(unread);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeWebSocketService extends WebSocketService {
  _FakeWebSocketService() : super(baseUrl: 'ws://example.invalid');

  final _controller = StreamController<WebSocketMessage>.broadcast();

  @override
  Stream<WebSocketMessage> get messages => _controller.stream;

  void emit(WebSocketMessage message) => _controller.add(message);

  void disposeController() => _controller.close();
}

class _GuestAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();

  @override
  Future<void> forceRefreshAuthState() async {}
}

void main() {
  testWidgets(
    'mounted initializer: notification.created converges the badge provider',
    (tester) async {
      final repo = _ScriptedNotificationRepository(unread: 0);
      final socket = _FakeWebSocketService();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(socket),
            notificationRepositoryProvider.overrideWithValue(repo),
            authControllerProvider.overrideWith(() => _GuestAuthController()),
            currentUserIdProvider.overrideWith((ref) => _userId),
          ],
          child: const MaterialApp(
            home: NotificationInitializer(child: SizedBox.shrink()),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 100));

      final container = ProviderScope.containerOf(
        tester.element(find.byType(NotificationInitializer)),
      );
      // Home badge source — watches the same canonical provider.
      container.listen(unreadCountProvider(_userId), (_, _) {});
      for (var i = 0; i < 10; i++) {
        await tester.pump(const Duration(milliseconds: 10));
      }
      expect(container.read(unreadCountProvider(_userId)).value, 0);

      // Canonical backend now has one unread; the WS event arrives on the
      // existing session the mounted initializer keeps consumed.
      repo.unread = 1;
      socket.emit(
        WebSocketMessage(
          id: 'evt-1',
          type: 'notification.created',
          from: 'server',
          data: {'notification_id': 'n1', 'type': 'chat_message', 'unread_count': 99},
        ),
      );
      for (var i = 0; i < 10; i++) {
        await tester.pump(const Duration(milliseconds: 10));
      }

      expect(repo.countSubscriptions, 2,
          reason: 'the mounted initializer must reconcile via the provider');
      expect(container.read(unreadCountProvider(_userId)).value, 1,
          reason: 'canonical 1 wins over the stale event hint 99');

      socket.disposeController();
    },
  );
}
