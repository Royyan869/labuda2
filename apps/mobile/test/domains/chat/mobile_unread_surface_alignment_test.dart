import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

class _FakeApiClient implements ApiClient {
  String? lastGetPath;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastGetPath = path;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data:
          {
                'success': true,
                'data': {'unread_count': 4},
              }
              as T,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('mobile unread surface alignment', () {
    test(
      'totalUnreadCountProvider is computed from room list unread_count',
      () {
        final container = ProviderContainer(
          overrides: [currentUserIdProvider.overrideWith((ref) => 'user_1')],
        );
        addTearDown(container.dispose);

        final chatListNotifier = container.read(chatListProvider.notifier);
        chatListNotifier.state = ChatListState(
          chats: [
            Chat(
              id: 'room_a',
              participantIds: ['user_1', 'user_2'],
              participantNames: {'user_2': 'alice'},
              participantAvatars: {'user_2': null},
              createdAt: DateTime(2026, 6, 2),
              unreadCounts: {'__room_unread__': 3},
            ),
            Chat(
              id: 'room_b',
              participantIds: ['user_1', 'user_3'],
              participantNames: {'user_3': 'bob'},
              participantAvatars: {'user_3': null},
              createdAt: DateTime(2026, 6, 2),
              unreadCounts: {'__room_unread__': 5},
            ),
          ],
        );

        expect(container.read(totalUnreadCountProvider), 8);
      },
    );

    test(
      'UnreadCount notifier syncs from chats without aggregate API call',
      () {
        final container = ProviderContainer();
        addTearDown(container.dispose);

        final unreadNotifier = container.read(unreadCountProvider.notifier);
        unreadNotifier.syncFromChats([
          Chat(
            id: 'room_x',
            participantIds: ['u1', 'u2'],
            participantNames: {'u2': 'alice'},
            participantAvatars: {'u2': null},
            createdAt: DateTime(2026, 6, 2),
            unreadCounts: {'__room_unread__': 2},
          ),
          Chat(
            id: 'room_y',
            participantIds: ['u1', 'u3'],
            participantNames: {'u3': 'bob'},
            participantAvatars: {'u3': null},
            createdAt: DateTime(2026, 6, 2),
            unreadCounts: {'__room_unread__': 1},
          ),
        ], 'u1');

        expect(container.read(unreadCountProvider), {'room_x': 2, 'room_y': 1});
      },
    );

    test('per-room unread endpoint remains mapped', () async {
      final apiClient = _FakeApiClient();
      final datasource = ChatApiDatasource(apiClient);

      final result = await datasource.getUnreadCount('room_123');
      expect(result.isSuccess, isTrue);
      expect(result.data, 4);
      expect(apiClient.lastGetPath, '/chat/rooms/room_123/unread');
    });
  });
}
