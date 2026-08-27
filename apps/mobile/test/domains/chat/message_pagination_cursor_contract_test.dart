import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';

class _CaptureApiClient implements ApiClient {
  String? lastPath;
  Map<String, dynamic>? lastQuery;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPath = path;
    lastQuery = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data:
          {
                'success': true,
                'data': {'data': <Map<String, dynamic>>[], 'has_more': false},
              }
              as T,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeChatRepository implements ChatRepository {
  DateTime? lastCursorCreatedAt;
  String? lastCursorId;
  int callCount = 0;

  @override
  Future<Result<List<Message>>> getMessages({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  }) async {
    callCount++;
    lastCursorCreatedAt = cursorCreatedAt;
    lastCursorId = cursorId;

    if (callCount == 1) {
      final messages = List<Message>.generate(50, (i) {
        final ts = DateTime.utc(
          2026,
          6,
          2,
          10,
          0,
          0,
        ).subtract(Duration(minutes: i));
        return Message(
          id: 'm_$i',
          chatId: chatId,
          senderId: 'u_sender',
          senderName: 'Sender',
          content: 'message $i',
          type: MessageType.text,
          createdAt: ts,
          status: MessageStatus.sent,
          mentionedUserIds: const [],
          deletedBy: const [],
        );
      });
      return Result.success(messages);
    }

    return Result.success(<Message>[
      Message(
        id: 'm_next_1',
        chatId: chatId,
        senderId: 'u_sender',
        senderName: 'Sender',
        content: 'next',
        type: MessageType.text,
        createdAt: DateTime.utc(2026, 6, 2, 9, 9, 0),
        status: MessageStatus.sent,
        mentionedUserIds: const [],
        deletedBy: const [],
      ),
    ]);
  }

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() {
    return const Stream.empty();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('chat message pagination cursor contract', () {
    test('first page request omits cursor fields', () async {
      final apiClient = _CaptureApiClient();
      final datasource = ChatApiDatasource(apiClient);

      final result = await datasource.listMessages('room_1', limit: 20);
      expect(result.isSuccess, isTrue);
      expect(apiClient.lastPath, '/chat/rooms/room_1/messages');
      expect(apiClient.lastQuery?['limit'], 20);
      expect(apiClient.lastQuery?.containsKey('cursor_created_at'), isFalse);
      expect(apiClient.lastQuery?.containsKey('cursor_id'), isFalse);
      expect(apiClient.lastQuery?.containsKey('cursor'), isFalse);
    });

    test(
      'next page request sends cursor_created_at and cursor_id (no cursor key)',
      () async {
        final apiClient = _CaptureApiClient();
        final datasource = ChatApiDatasource(apiClient);
        final ts = DateTime.utc(2026, 6, 2, 9, 10, 11);

        final result = await datasource.listMessages(
          'room_1',
          cursorCreatedAt: ts,
          cursorId: 'msg_123',
          limit: 20,
        );
        expect(result.isSuccess, isTrue);
        expect(apiClient.lastQuery?['cursor_created_at'], ts.toIso8601String());
        expect(apiClient.lastQuery?['cursor_id'], 'msg_123');
        expect(apiClient.lastQuery?.containsKey('cursor'), isFalse);
      },
    );

    test(
      'pagination state uses last message timestamp/id as cursor source',
      () async {
        const chatId = 'chat_abc';
        final fakeRepo = _FakeChatRepository();
        final container = ProviderContainer(
          overrides: [chatRepositoryProvider.overrideWithValue(fakeRepo)],
        );
        addTearDown(container.dispose);

        final notifier = container.read(chatDetailProvider(chatId).notifier);
        await notifier.loadMessages('user_me');

        var state = container.read(chatDetailProvider(chatId));
        expect(state.messages.length, 50);
        expect(state.messages.first.id, 'm_0');
        expect(state.messages.last.id, 'm_49');
        expect(state.hasMoreMessages, isTrue);
        expect(state.nextMessageCursor, contains('m_49'));

        await notifier.loadMoreMessages('user_me');
        state = container.read(chatDetailProvider(chatId));

        expect(fakeRepo.lastCursorId, 'm_49');
        expect(
          fakeRepo.lastCursorCreatedAt?.toUtc().toIso8601String(),
          DateTime.utc(2026, 6, 2, 9, 11, 0).toIso8601String(),
        );
        expect(state.messages.length, 51);
        expect(state.messages.first.id, 'm_0');
        expect(state.messages[49].id, 'm_49');
        expect(state.messages.last.id, 'm_next_1');
      },
    );

    test(
      'malformed cursor in pagination state does not crash loadMore',
      () async {
        const chatId = 'chat_bad_cursor';
        final fakeRepo = _FakeChatRepository();
        final container = ProviderContainer(
          overrides: [chatRepositoryProvider.overrideWithValue(fakeRepo)],
        );
        addTearDown(container.dispose);

        final notifier = container.read(chatDetailProvider(chatId).notifier);
        notifier.state = notifier.state.copyWith(
          hasMoreMessages: true,
          nextMessageCursor: 'malformed_cursor_without_separator',
        );

        await notifier.loadMoreMessages('user_me');
        expect(fakeRepo.callCount, 0);
      },
    );
  });
}
