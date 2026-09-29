import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

// ============================================================================
// THREAD ORDER AUTHORITY — WRITE PATH
//
// The detail thread is `reverse: true` over a **newest-first** list, so index 0
// is rendered at the BOTTOM. The render convention already has its own gate
// (`chat_message_ordering_display_test.dart`: index 0 renders below index 1).
// What was never gated is the WRITE path that has to produce that order — which
// is exactly how a sent message ended up parked at the top of the screen until
// the thread was reopened and re-read from the server page.
//
// These tests drive the REAL notifier: send, live refresh, and pagination must
// all leave the list newest-first without a positional assumption.
// ============================================================================

const _roomId = 'room_1';
const _me = 'me';
const _other = 'other';

Message _message(String id, DateTime createdAt, {String senderId = _other}) =>
    Message(
      id: id,
      chatId: _roomId,
      senderId: senderId,
      senderName: senderId == _me ? 'Me' : 'Other',
      content: 'body-$id',
      createdAt: createdAt,
    );

final _t1 = DateTime.utc(2026, 6, 1, 1, 0);
final _t2 = DateTime.utc(2026, 6, 1, 2, 0);
final _tSent = DateTime.utc(2026, 6, 1, 3, 0);

ChatRoomEventDto _roomEvent() => ChatRoomEventDto(
  eventType: WebSocketEventType.roomUpdated,
  roomId: _roomId,
  roomType: 'direct',
  otherUserId: _other,
  unreadCount: 0,
  createdAt: DateTime.utc(2026, 6, 1),
  updatedAt: DateTime.utc(2026, 6, 2),
  lastMessageAt: DateTime.utc(2026, 6, 2),
);

class _ThreadRepo implements ChatRepository {
  _ThreadRepo(this.events);

  final Stream<ChatRoomEventDto> events;

  /// Newest-first, exactly like the canonical REST page shape.
  List<Message> serverMessages = const [];

  int sendCalls = 0;
  String? lastCursorId;
  DateTime? lastCursorCreatedAt;

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() => events;

  @override
  Future<Result<List<Message>>> getMessages({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  }) async {
    lastCursorId = cursorId;
    lastCursorCreatedAt = cursorCreatedAt;
    return Result.success(serverMessages);
  }

  @override
  Future<Result<bool>> markMessagesAsRead({
    required String chatId,
    required String userId,
    List<String>? messageIds,
  }) async => Result.success(true);

  @override
  Future<Result<Message>> sendMessage({
    required String chatId,
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    List<String> mediaAssetIds = const [],
    String? replyToId,
    List<String> mentionedUserIds = const [],
    ShareReference? objectReference,
    Map<String, dynamic>? workflowAttachment,
    ChatResourceOccurrenceRequest? resourceOccurrence,
  }) async {
    sendCalls++;
    // The server answers with the created message, which is by definition the
    // newest message in the room.
    return Result.success(
      Message(
        id: 'sent_$sendCalls',
        chatId: chatId,
        senderId: senderId,
        senderName: senderName,
        content: content,
        type: type,
        createdAt: _tSent,
        status: MessageStatus.sent,
        mentionedUserIds: const [],
        deletedBy: const [],
      ),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoOpNotificationRepository implements INotificationRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

void main() {
  late StreamController<ChatRoomEventDto> events;
  late _ThreadRepo repo;
  late ProviderContainer container;

  // The REAL notifier: these tests exercise the production write path.
  ChatDetail notifier() => container.read(chatDetailProvider(_roomId).notifier);

  List<String> ids() =>
      container.read(chatDetailProvider(_roomId)).messages.map((m) => m.id).toList();

  setUp(() {
    events = StreamController<ChatRoomEventDto>.broadcast();
    repo = _ThreadRepo(events.stream);
    container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(repo),
        currentUserIdProvider.overrideWith((ref) => _me),
        notificationRepositoryProvider.overrideWithValue(
          _NoOpNotificationRepository(),
        ),
      ],
    );
    container.listen(chatDetailProvider(_roomId), (_, _) {}, fireImmediately: true);
    addTearDown(() async {
      container.dispose();
      await events.close();
    });
  });

  test(
    'a sent message enters as the newest message, never as the oldest',
    () async {
      repo.serverMessages = [_message('m2', _t2), _message('m1', _t1)];
      await notifier().loadMessages(_me);
      expect(ids(), ['m2', 'm1']);

      final sent = await notifier().sendMessage(
        senderId: _me,
        senderName: 'Me',
        content: 'halo',
      );

      expect(sent, isNotNull);
      // Index 0 is the bottom of a `reverse: true` list. Appending here is what
      // put the newest bubble on top until a reopen re-read the page.
      expect(ids(), ['sent_1', 'm2', 'm1']);
    },
  );

  test(
    'a refresh page that predates a just-sent message never demotes it',
    () async {
      repo.serverMessages = [_message('m2', _t2), _message('m1', _t1)];
      await notifier().loadMessages(_me);

      await notifier().sendMessage(
        senderId: _me,
        senderName: 'Me',
        content: 'halo',
      );
      expect(ids().first, 'sent_1');

      // The live refresh re-reads the canonical page. It predates the message
      // this client just sent, so a positional merge would classify the local
      // copy as "older history" and push it to the top of the thread.
      repo.serverMessages = [_message('m2', _t2), _message('m1', _t1)];
      events.add(_roomEvent());
      await pumpEventQueue();

      expect(ids(), ['sent_1', 'm2', 'm1']);
    },
  );

  test('thread order is normalized even when the page arrives ascending', () async {
    repo.serverMessages = [_message('m1', _t1), _message('m2', _t2)];
    await notifier().loadMessages(_me);

    expect(ids(), ['m2', 'm1']);
  });

  test('pagination cursor names the oldest message, not the last element', () async {
    final base = DateTime.utc(2026, 6, 2, 10, 0);
    // Delivered in the WRONG direction (ascending) on purpose: here the oldest
    // message is FIRST and the newest is LAST, so a cursor taken from the last
    // list position would name the newest message as "older than this".
    repo.serverMessages = List<Message>.generate(
      50,
      (i) => _message('m_${49 - i}', base.subtract(Duration(minutes: 49 - i))),
    );

    await notifier().loadMessages(_me);

    final state = container.read(chatDetailProvider(_roomId));
    expect(state.messages.first.id, 'm_0');
    expect(state.messages.last.id, 'm_49');
    expect(state.hasMoreMessages, isTrue);

    // `m_49` is the oldest message. A cursor taken from the last list position
    // would name `m_0` here and hand the server its NEWEST message as "older
    // than this", re-reading the newest page as history.
    expect(state.nextMessageCursor, contains('m_49'));

    await notifier().loadMoreMessages(_me);
    expect(repo.lastCursorId, 'm_49');
    expect(
      repo.lastCursorCreatedAt?.toUtc().toIso8601String(),
      base.subtract(const Duration(minutes: 49)).toIso8601String(),
    );
  });

  test('the second insert path stays dead', () {
    // `addMessage` was a zero-consumer append of the same shape as the send
    // bug. It is gone and must not come back: the notifier exposes ONE way for
    // a message to enter thread state, and it orders by construction.
    final source = File(
      'lib/domains/chat/chat/presentation/providers/chat_notifier.dart',
    ).readAsStringSync();

    expect(source.contains('void addMessage('), isFalse);
    expect(source.contains('[...state.messages, result.data'), isFalse);

    // The sent message REPLACES its optimistic row through the same order
    // authority — the local id is dropped in the same merge, never left as a
    // duplicate.
    expect(
      source.contains('_mergeNewestFirst([result.data!], dropId: localId)'),
      isTrue,
    );

    // The in-flight states `MessageStatus.sending`/`failed` used to be declared
    // but produced by nobody: the bubble had icons for states nothing ever set.
    expect(source.contains('status: MessageStatus.sending'), isTrue);
    expect(source.contains('status: MessageStatus.failed'), isTrue);
  });
}
