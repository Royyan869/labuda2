import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/data/repositories/chat_repository_impl.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';

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
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
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

Map<String, dynamic> _messagePayload(String chatRoomId) => <String, dynamic>{
  'id': 'msg_${DateTime.now().microsecondsSinceEpoch}',
  'chat_room_id': chatRoomId,
  'sender_id': 'user_sender',
  'sender_name': 'Alice',
  'sender_avatar': null,
  'content': 'hello',
  'type': 'text',
  'media_urls': <String>[],
  'attachment': null,
  'status': 'sent',
  'is_read': false,
  'is_edited': false,
  'edited_at': null,
  'reply_to_id': null,
  'reply_preview': null,
  'mentioned_user_ids': <String>[],
  'created_at': '2026-05-16T00:00:00.000Z',
  'updated_at': '2026-05-16T00:00:00.000Z',
};

Map<String, dynamic> _roomCreatedPayload() => <String, dynamic>{
  'room_id': 'room_created_1',
  'room_type': 'direct',
  'other_user_id': 'user_other',
  'other_user': <String, dynamic>{
    'id': 'user_other',
    'display_name': 'Other User',
    'username': 'other_user',
    'avatar_url': 'https://example.com/avatar.png',
    'lifecycle': 'active',
  },
  'context': <String, dynamic>{'target_type': 'listing', 'target_id': 'lst_1'},
  'context_set_by': 'user_self',
  'linked_order_id': null,
  'last_message': null,
  'unread_count': 2,
  'created_at': '2026-06-02T10:00:00.000Z',
  'updated_at': '2026-06-02T10:01:00.000Z',
  'last_message_at': '2026-06-02T10:01:00.000Z',
};

Map<String, dynamic> _roomUpdatedPayload() => <String, dynamic>{
  'room_id': 'room_updated_1',
  'room_type': 'direct',
  'other_user_id': 'user_other',
  'other_user': <String, dynamic>{
    'id': 'user_other',
    'display_name': 'Other User',
    'username': 'other_user',
    'avatar_url': 'https://example.com/avatar.png',
    'lifecycle': 'active',
  },
  'context': <String, dynamic>{'target_type': 'listing', 'target_id': 'lst_1'},
  'context_set_by': 'user_self',
  'linked_order_id': 'order_1',
  'last_message': <String, dynamic>{
    'id': 'msg_last_1',
    'room_id': 'room_updated_1',
    'sender_id': 'user_other',
    'message_type': 'text',
    'body': 'preview message',
    'is_hidden': false,
    'created_at': '2026-06-02T10:04:00.000Z',
  },
  'unread_count': 1,
  'created_at': '2026-06-02T10:00:00.000Z',
  'updated_at': '2026-06-02T10:05:00.000Z',
  'last_message_at': '2026-06-02T10:04:00.000Z',
};

Map<String, dynamic> _roomMalformedPayload() => <String, dynamic>{
  'room_type': 'direct',
  'other_user_id': 'user_other',
  'created_at': '2026-06-02T10:00:00.000Z',
  'updated_at': '2026-06-02T10:05:00.000Z',
  'last_message_at': '2026-06-02T10:04:00.000Z',
};

Map<String, dynamic> _roomEnvelope({
  required String type,
  required Map<String, dynamic> payload,
}) {
  return <String, dynamic>{'type': type, 'payload': payload};
}

ChatRepositoryImpl _buildRepo() {
  return ChatRepositoryImpl(
    apiDatasource: ChatApiDatasource(_NoopApiClient()),
    webSocketService: WebSocketService(baseUrl: 'ws://example.invalid'),
    logger: _SilentLogger(),
  );
}

void main() {
  group('Chat room event stream gateway', () {
    test('chat.room.created emits one typed room event', () async {
      final repo = _buildRepo();
      final events = <ChatRoomEventDto>[];
      final sub = repo.watchChatRoomEvents().listen(events.add);

      repo.handleWebSocketEventForTest(
        _roomEnvelope(
          type: 'chat.room.created',
          payload: _roomCreatedPayload(),
        ),
      );

      await Future<void>.delayed(Duration.zero);

      expect(events, hasLength(1));
      expect(events.single.eventType, WebSocketEventType.roomCreated);
      expect(events.single.roomId, 'room_created_1');
      expect(events.single.lastMessage, isNull);
      expect(events.single.unreadCount, 2);

      await sub.cancel();
      repo.dispose();
    });

    test('chat.room.updated emits one typed room event', () async {
      final repo = _buildRepo();
      final events = <ChatRoomEventDto>[];
      final sub = repo.watchChatRoomEvents().listen(events.add);

      repo.handleWebSocketEventForTest(
        _roomEnvelope(
          type: 'chat.room.updated',
          payload: _roomUpdatedPayload(),
        ),
      );

      await Future<void>.delayed(Duration.zero);

      expect(events, hasLength(1));
      expect(events.single.eventType, WebSocketEventType.roomUpdated);
      expect(events.single.roomId, 'room_updated_1');
      expect(events.single.lastMessage, isNotNull);
      expect(events.single.lastMessage!.isHidden, isFalse);
      expect(events.single.lastMessage!.body, 'preview message');

      await sub.cancel();
      repo.dispose();
    });

    test('malformed room event does not crash and does not emit', () async {
      final repo = _buildRepo();
      final events = <ChatRoomEventDto>[];
      final sub = repo.watchChatRoomEvents().listen(events.add);

      expect(
        () => repo.handleWebSocketEventForTest(
          _roomEnvelope(
            type: 'chat.room.updated',
            payload: _roomMalformedPayload(),
          ),
        ),
        returnsNormally,
      );

      await Future<void>.delayed(Duration.zero);

      expect(events, isEmpty);

      await sub.cancel();
      repo.dispose();
    });

    test('unknown room event does not emit', () async {
      final repo = _buildRepo();
      final events = <ChatRoomEventDto>[];
      final sub = repo.watchChatRoomEvents().listen(events.add);

      repo.handleWebSocketEventForTest(
        _roomEnvelope(
          type: 'chat.room.removed',
          payload: _roomCreatedPayload(),
        ),
      );

      await Future<void>.delayed(Duration.zero);

      expect(events, isEmpty);

      await sub.cancel();
      repo.dispose();
    });

    test('message events still reach the existing message handler', () async {
      final repo = _buildRepo();
      const chatRoomId = 'room_messages_1';
      final controller = repo.primeMessageControllerForTest(chatRoomId);
      final messages = <Message>[];
      final sub = controller.stream.listen(messages.add);

      repo.handleWebSocketEventForTest(
        _roomEnvelope(
          type: 'chat.message.sent',
          payload: _messagePayload(chatRoomId),
        ),
      );

      await Future<void>.delayed(Duration.zero);

      expect(messages, hasLength(1));
      expect(messages.single.chatId, chatRoomId);
      expect(messages.single.content, 'hello');

      await sub.cancel();
      repo.dispose();
    });

    test('stream closes cleanly on repository dispose', () async {
      final repo = _buildRepo();
      final stream = repo.watchChatRoomEvents();
      final done = expectLater(stream, emitsDone);

      repo.dispose();

      await done;
    });
  });
}
