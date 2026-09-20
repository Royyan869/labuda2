import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';

class _RecordingRepo implements ChatRepository {
  final List<Chat> chats;
  final List<String> markedRooms = [];

  _RecordingRepo(this.chats);

  @override
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async => Result.success(chats);

  @override
  Future<Result<bool>> markMessagesAsRead({
    required String chatId,
    required String userId,
    List<String>? messageIds,
  }) async {
    markedRooms.add(chatId);
    return Result.success(true);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Chat _chat(String id, int unread) => Chat(
  id: id,
  participantIds: const ['me', 'other'],
  participantNames: const {'other': 'other'},
  participantAvatars: const {},
  createdAt: DateTime.utc(2026, 1, 1),
  unreadCount: unread,
);

void main() {
  // Regression: "Mark all as read" used to read a per-user map key that never
  // matched the wire data, so it silently marked nothing. It now uses the
  // canonical room unread value.
  test('markAllRead marks only rooms with unread and zeroes every badge',
      () async {
    final repo = _RecordingRepo([_chat('room_1', 3), _chat('room_2', 0)]);
    final container = ProviderContainer(
      overrides: [chatRepositoryProvider.overrideWithValue(repo)],
    );
    addTearDown(container.dispose);
    final sub = container.listen(chatListProvider, (_, __) {},
        fireImmediately: true);
    addTearDown(sub.close);

    final notifier = container.read(chatListProvider.notifier);
    await notifier.loadChats('me');

    expect(
      container.read(chatListProvider).chats.map((c) => c.roomUnreadCount),
      [3, 0],
    );

    await notifier.markAllRead('me');

    // Only the room that actually had unread messages is marked...
    expect(repo.markedRooms, ['room_1']);
    // ...and every badge is cleared locally.
    expect(
      container
          .read(chatListProvider)
          .chats
          .every((c) => c.roomUnreadCount == 0),
      isTrue,
    );
  });
}
