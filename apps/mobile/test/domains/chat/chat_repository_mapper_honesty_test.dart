// Runtime Honesty Tier 4 — ChatRepositoryImpl.
//
// Verifies that a mapper failure on an incoming chat message is
// surfaced as a stream error on the affected chat-room stream
// (instead of being silently logged and dropped), AND that the
// stream remains open for subsequent valid messages.
//
// Test seams used (added in Tier 4):
//   * `messageMapperForTest`         — injects a throwing mapper so we
//                                      can exercise the mapper-error
//                                      branch deterministically without
//                                      constructing a payload that
//                                      breaks ChatMapper internals.
//   * `primeMessageControllerForTest` — registers a stream controller
//                                      for a chat-room id without
//                                      going through the public
//                                      `watchMessages` path (which
//                                      triggers backend side effects).
//   * `handleMessageEventForTest`    — drives `_handleMessageEvent`
//                                      directly with a crafted payload.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/data/repositories/chat_repository_impl.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

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

Map<String, dynamic> _validMessagePayload(String chatRoomId) =>
    <String, dynamic>{
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

Map<String, dynamic> _validMessagePayloadWithAttachment(String chatRoomId) =>
    <String, dynamic>{
      ..._validMessagePayload(chatRoomId),
      'attachment': {
        'type': 'reference',
        'data': {
          'target_type': 'for_sale',
          'target_id': 'listing_hidden_1',
          'preview': {'title': 'Produk Rahasia'},
        },
      },
    };

void main() {
  ChatRepositoryImpl buildRepo({
    Message Function(MessageDto)? mapper,
    Future<void> Function(String chatRoomId)? refreshHook,
  }) {
    return ChatRepositoryImpl(
      apiDatasource: ChatApiDatasource(_NoopApiClient()),
      webSocketService: WebSocketService(baseUrl: 'ws://example.invalid'),
      logger: _SilentLogger(),
      messageMapperForTest: mapper,
      roomRefreshHookForTest: refreshHook,
    );
  }

  group('ChatRepositoryImpl — mapper honesty (C2)', () {
    test('mapper failure surfaces as a stream error on the affected '
        'chat-room stream (not a silent drop) and the controller stays '
        'open for subsequent valid messages', () async {
      var shouldThrow = true;
      final repo = buildRepo(
        mapper: (dto) {
          if (shouldThrow) {
            throw StateError('simulated mapper failure on $dto');
          }
          // Build a minimal valid Message for the recovery branch.
          return Message(
            id: dto.id,
            chatId: dto.chatRoomId,
            senderId: dto.senderId,
            senderName: dto.senderName,
            content: dto.content,
            type: MessageType.text,
            mediaUrls: const [],
            createdAt: dto.createdAt,
            status: MessageStatus.sent,
            isEdited: false,
            mentionedUserIds: const [],
            deletedBy: const [],
          );
        },
      );

      const chatRoomId = 'chat_room_alpha';
      final controller = repo.primeMessageControllerForTest(chatRoomId);

      final errors = <Object>[];
      final messages = <Message>[];
      final sub = controller.stream.listen(messages.add, onError: errors.add);

      // 1) Drive a payload that succeeds at MessageDto.fromJson but
      // fails at the mapper. The previous behavior was a silent
      // _logger.error — Tier 4 must surface this as a stream error.
      repo.handleMessageEventForTest(_validMessagePayload(chatRoomId));

      await Future<void>.delayed(Duration.zero);

      expect(
        errors,
        hasLength(1),
        reason:
            'mapper failure must surface as a stream error rather than '
            'silently dropping the message',
      );
      expect(messages, isEmpty);

      // 2) Flip the mapper to success and drive another valid
      // payload. The controller MUST still be open and route the
      // recovered message.
      shouldThrow = false;
      repo.handleMessageEventForTest(_validMessagePayload(chatRoomId));

      await Future<void>.delayed(Duration.zero);

      expect(
        messages,
        hasLength(1),
        reason:
            'controller must remain open after a mapper error so '
            'subsequent valid messages still reach the listener',
      );

      await sub.cancel();
    });

    test('DTO-level parse failure logs but does NOT crash the service '
        '(no controller to route to — service stays alive)', () async {
      final repo = buildRepo();

      // Payload with `chat_room_id` missing — MessageDto.fromJson
      // will throw because the cast is non-nullable.
      final badPayload = <String, dynamic>{
        'id': 'msg_1',
        // 'chat_room_id': intentionally omitted
        'sender_id': 'user_sender',
        'sender_name': 'Alice',
        'content': 'oops',
        'type': 'text',
        'status': 'sent',
        'is_read': false,
        'is_edited': false,
        'created_at': '2026-05-16T00:00:00.000Z',
        'updated_at': '2026-05-16T00:00:00.000Z',
      };

      // Must not throw out of the handler — DTO failure is logged
      // and swallowed at the service boundary (no chat-room id to
      // route the error to).
      repo.handleMessageEventForTest(badPayload);

      // Service should remain usable for subsequent valid payloads
      // on a fresh chat room.
      const chatRoomId = 'chat_room_beta';
      final controller = repo.primeMessageControllerForTest(chatRoomId);
      final received = <Message>[];
      final sub = controller.stream.listen(received.add);

      repo.handleMessageEventForTest(_validMessagePayload(chatRoomId));
      await Future<void>.delayed(Duration.zero);

      expect(received, hasLength(1));
      await sub.cancel();
    });
    test(
      'hidden signal marks cached local message as hidden in same room',
      () async {
        final repo = buildRepo();
        const chatRoomId = 'chat_room_hidden';
        const messageId = 'msg_hidden_1';
        final controller = repo.primeMessageControllerForTest(chatRoomId);
        final messages = <Message>[];
        final sub = controller.stream.listen(messages.add);

        repo.handleMessageEventForTest(
          _validMessagePayloadWithAttachment(chatRoomId)..['id'] = messageId,
        );
        await Future<void>.delayed(Duration.zero);
        messages.clear();

        repo.handleWebSocketEventForTest({
          'type': 'chat.message.hidden',
          'payload': {'room_id': chatRoomId, 'message_id': messageId},
        });
        await Future<void>.delayed(Duration.zero);

        expect(messages, hasLength(1));
        expect(messages.first.id, messageId);
        expect(messages.first.isHidden, isTrue);
        expect(messages.first.content, isEmpty);
        expect(messages.first.hasAttachment, isFalse);
        expect(messages.first.objectReference, isNull);
        await sub.cancel();
        repo.dispose();
      },
    );

    test(
      'restored signal triggers room-scoped refresh hook only for related room',
      () async {
        final refreshed = <String>[];
        final repo = buildRepo(
          refreshHook: (roomId) async {
            refreshed.add(roomId);
          },
        );

        const activeRoom = 'chat_room_active';
        repo.primeMessageControllerForTest(activeRoom);

        repo.handleWebSocketEventForTest({
          'type': 'chat.message.restored',
          'payload': {'room_id': activeRoom, 'message_id': 'msg_1'},
        });
        repo.handleWebSocketEventForTest({
          'type': 'chat.message.restored',
          'payload': {'room_id': 'chat_room_other', 'message_id': 'msg_2'},
        });
        await Future<void>.delayed(Duration.zero);

        expect(refreshed, [activeRoom]);
        repo.dispose();
      },
    );

    test(
      'chat.message.sent minimal signal triggers room-scoped refresh only for active room',
      () async {
        final refreshed = <String>[];
        final repo = buildRepo(
          refreshHook: (roomId) async {
            refreshed.add(roomId);
          },
        );

        const activeRoom = 'chat_room_active';
        repo.primeMessageControllerForTest(activeRoom);

        repo.handleWebSocketEventForTest({
          'type': 'chat.message.sent',
          'payload': {'room_id': activeRoom, 'message_id': 'msg_sent_1'},
        });
        repo.handleWebSocketEventForTest({
          'type': 'chat.message.sent',
          'payload': {'room_id': 'chat_room_other', 'message_id': 'msg_sent_2'},
        });
        await Future<void>.delayed(Duration.zero);

        expect(
          refreshed,
          [activeRoom],
          reason:
              'signal for unrelated room must be ignored when there is no active controller',
        );
        repo.dispose();
      },
    );

    test(
      'chat.message.sent refresh path relies on room_id/message_id signal only',
      () async {
        final refreshed = <String>[];
        final repo = buildRepo(
          refreshHook: (roomId) async {
            refreshed.add(roomId);
          },
        );

        const activeRoom = 'chat_room_active_minimal';
        repo.primeMessageControllerForTest(activeRoom);

        repo.handleWebSocketEventForTest({
          'type': 'chat.message.sent',
          'payload': {'room_id': activeRoom, 'message_id': 'msg_minimal_1'},
        });
        await Future<void>.delayed(Duration.zero);

        expect(
          refreshed,
          [activeRoom],
          reason: 'consumer must not require full message payload over WS',
        );
        repo.dispose();
      },
    );

    test(
      'message.deleted is treated as unknown and does not mutate room stream',
      () async {
        final repo = buildRepo();
        const activeRoom = 'chat_room_deleted_dead_event';
        final controller = repo.primeMessageControllerForTest(activeRoom);

        final messages = <Message>[];
        final sub = controller.stream.listen(messages.add);

        repo.handleWebSocketEventForTest({
          'type': 'message.deleted',
          'payload': {'room_id': activeRoom, 'message_id': 'msg_del_1'},
        });
        await Future<void>.delayed(Duration.zero);

        expect(messages, isEmpty);
        await sub.cancel();
        repo.dispose();
      },
    );
  });
}
