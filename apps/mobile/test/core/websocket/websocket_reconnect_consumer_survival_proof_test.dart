// PHASE 4.1 IMPLEMENTATION PROOF — STABLE WEBSOCKET MESSAGE STREAM.
//
// Runtime regression for the fixed transport lifecycle: ONE stable `messages`
// stream per WebSocketService lifetime. Reconnect only swaps the transport
// channel; consumers that subscribed before a reconnect keep receiving on the
// SAME stream without re-subscribing.
//
// Drives PRODUCTION pieces end-to-end:
//   - REAL WebSocketService.connect()/disconnect()/dispose();
//   - REAL ChatRepositoryImpl (production chat consumer);
//   - REAL notificationRealtimeSyncProvider (production notification consumer);
//   - REAL frame-ingestion seam handleMessageForTest.
//
// Proves: stable stream · chat survival · notification survival · duplicate
// protection · multiple reconnect cycles · disconnect→reconnect · dispose.
import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:hishumi/domains/chat/chat/data/dto/message_dto.dart'
    show WebSocketEventType;
import 'package:hishumi/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:hishumi/domains/chat/chat/data/repositories/chat_repository_impl.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_realtime_sync_provider.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _SilentLogger implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> setLogLevel(LogLevel level) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> clearLogs() => Future.value(Result.success(null));
  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) => Future.value(Result.success(const <LogEntry>[]));
  @override
  Future<void> debugSync(String userId) async {}
  @override
  Future<void> debugSyncSuccess(String userId) async {}
  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}
  @override
  Future<void> debugCallingGetCurrentUser() async {}
  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}
  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}
  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}
  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}
  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

/// Counting canonical backend for the notification consumer.
class _CountingNotificationRepository implements INotificationRepository {
  int countSubscriptions = 0;

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countSubscriptions++;
    return Result.success(0);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

// Payload shape mirrors the proven chat_room_event_stream_test fixture so
// ChatRoomEventDto.fromWebSocketEvent accepts it.
Map<String, dynamic> _roomUpdatedData() => <String, dynamic>{
  'room_id': 'room_1',
  'room_type': 'direct',
  'other_user_id': 'user_other',
  'other_user': <String, dynamic>{
    'id': 'user_other',
    'display_name': 'Other User',
    'username': 'other_user',
    'avatar_url': 'https://example.com/avatar.png',
    'lifecycle': 'active',
  },
  'linked_order_id': null,
  'last_message': null,
  'unread_count': 1,
  'created_at': '2026-06-02T10:00:00.000Z',
  'updated_at': '2026-06-02T10:05:00.000Z',
  'last_message_at': '2026-06-02T10:04:00.000Z',
};

String _chatRoomUpdatedFrame() => jsonEncode({
  'id': 'evt-chat',
  'type': 'chat.room.updated',
  'timestamp': DateTime.now().toUtc().toIso8601String(),
  'from': 'server',
  'data': _roomUpdatedData(),
});

String _notificationCreatedFrame() => jsonEncode({
  'id': 'evt-notif',
  'type': 'notification.created',
  'timestamp': DateTime.now().toUtc().toIso8601String(),
  'from': 'server',
  'data': {
    'notification_id': 'n1',
    'type': 'chat_message',
    'unread_count': 1,
  },
});

void main() {
  test(
    'STABLE STREAM PROOF: chat + notification consumers survive multiple '
    'reconnects and disconnect→reconnect; dispose ends delivery',
    () async {
      // Unreachable server: the REAL connect() still executes its synchronous
      // prologue (channel + create-once controller) before failing at the
      // handshake await — the same code path production reconnect takes.
      final service = WebSocketService(baseUrl: 'ws://127.0.0.1:1/unreachable');

      // ── INITIAL CONNECT (production lifecycle entry) ───────────────────
      await service.connect('token-1').catchError((Object _) {});

      // ── PRODUCTION CONSUMERS SUBSCRIBE (never re-subscribed later) ─────
      final chatRepo = ChatRepositoryImpl(
        apiDatasource: ChatApiDatasource(_NoopApiClient()),
        webSocketService: service,
        logger: _SilentLogger(),
      );
      final chatEvents = <ChatRoomEventDto>[];
      final chatSub = chatRepo.watchChatRoomEvents().listen(chatEvents.add);

      final notifRepo = _CountingNotificationRepository();
      final container = ProviderContainer(
        overrides: [
          webSocketServiceProvider.overrideWithValue(service),
          notificationRepositoryProvider.overrideWithValue(notifRepo),
          currentUserIdProvider.overrideWith((ref) => 'u1'),
        ],
      );
      container.listen(notificationRealtimeSyncProvider, (_, _) {});
      container.listen(unreadCountProvider('u1'), (_, _) {});
      await pumpEventQueue();

      // ── SEGMENT 1: pre-reconnect delivery ──────────────────────────────
      service.handleMessageForTest(_chatRoomUpdatedFrame());
      service.handleMessageForTest(_notificationCreatedFrame());
      await pumpEventQueue();
      expect(chatEvents, hasLength(1),
          reason: 'chat consumer must receive the pre-reconnect event');
      // 1 activation subscribe + 1 event-driven refetch.
      expect(notifRepo.countSubscriptions, 2,
          reason:
              'notification consumer must reconcile the pre-reconnect event');

      // ── SEGMENT 2: after FIRST reconnect ───────────────────────────────
      await service.connect('token-2').catchError((Object _) {});
      service.handleMessageForTest(_chatRoomUpdatedFrame());
      service.handleMessageForTest(_notificationCreatedFrame());
      await pumpEventQueue();
      expect(chatEvents, hasLength(2),
          reason:
              'SAME chat consumer must receive after reconnect #1 — no re-subscribe');
      expect(notifRepo.countSubscriptions, 3,
          reason:
              'SAME notification consumer must reconcile after reconnect #1 — no re-subscribe');

      // ── SEGMENT 3: after SECOND reconnect (multiple-cycle proof) ───────
      await service.connect('token-3').catchError((Object _) {});
      service.handleMessageForTest(_chatRoomUpdatedFrame());
      service.handleMessageForTest(_notificationCreatedFrame());
      await pumpEventQueue();
      expect(chatEvents, hasLength(3),
          reason: 'chat survival holds across repeated reconnects');
      expect(notifRepo.countSubscriptions, 4,
          reason: 'notification survival holds across repeated reconnects');

      // ── SEGMENT 4: disconnect (logout transport) then reconnect ────────
      await service.disconnect();
      await service.connect('token-4').catchError((Object _) {});
      service.handleMessageForTest(_chatRoomUpdatedFrame());
      service.handleMessageForTest(_notificationCreatedFrame());
      await pumpEventQueue();
      expect(chatEvents, hasLength(4),
          reason:
              'disconnect() must NOT close the stable stream — consumers keep receiving after re-login connect');
      expect(notifRepo.countSubscriptions, 5,
          reason:
              'disconnect() must NOT close the stable stream — same for the notification consumer');

      // ── DUPLICATE PROTECTION ───────────────────────────────────────────
      // Exact totals prove exactly one live consumption path per event:
      // 4 chat frames → 4 repo events; 4 notification frames → 4 refetches
      // on top of the single activation. No duplicate subscription or event.
      expect(
        chatEvents
            .where((e) => e.eventType == WebSocketEventType.roomUpdated)
            .length,
        4,
      );
      expect(notifRepo.countSubscriptions, 5);

      // ── DISPOSE: streams close; delivery ends ──────────────────────────
      final doneBeforeDispose = <bool>[];
      final chatDoneSub = service.messages.listen(
        (_) {},
        onDone: () => doneBeforeDispose.add(true),
      );
      await service.dispose();
      await pumpEventQueue();
      expect(doneBeforeDispose, hasLength(1),
          reason: 'dispose() is the ONLY owner that closes the stable stream');

      service.handleMessageForTest(_chatRoomUpdatedFrame());
      service.handleMessageForTest(_notificationCreatedFrame());
      await pumpEventQueue();
      expect(chatEvents, hasLength(4),
          reason: 'no chat delivery after dispose');
      expect(notifRepo.countSubscriptions, 5,
          reason: 'no notification reconciliation after dispose');

      await chatSub.cancel();
      await chatDoneSub.cancel();
      chatRepo.dispose();
      container.dispose();
    },
    timeout: const Timeout(Duration(seconds: 120)),
  );
}
