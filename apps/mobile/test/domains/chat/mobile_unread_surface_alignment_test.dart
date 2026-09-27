import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

void main() {
  group('mobile unread surface alignment', () {
    // CANONICAL UNREAD AUTHORITY: `unread_count` carried by the room payload
    // (room list / room read / chat.room.updated). There is no separate
    // per-room unread fetch in production — the old datasource mapping was a
    // test-only consumer and was removed with it.
    test('totalUnreadCountProvider is computed from room list unread_count', () {
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
            unreadCount: 3,
          ),
          Chat(
            id: 'room_b',
            participantIds: ['user_1', 'user_3'],
            participantNames: {'user_3': 'bob'},
            participantAvatars: {'user_3': null},
            createdAt: DateTime(2026, 6, 2),
            unreadCount: 5,
          ),
        ],
      );

      expect(container.read(totalUnreadCountProvider), 8);
    });
  });
}
