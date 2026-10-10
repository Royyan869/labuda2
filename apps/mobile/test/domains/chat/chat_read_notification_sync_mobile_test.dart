// TASK 3 — Chat Read → Notification Read (server-side), mobile proof.
//
// After the backend completes chat notifications inside POST /chat/rooms/:id/read:
//   1. ChatDetail.markAsRead performs the SINGLE chat-read mutation and NEVER
//      calls POST /notifications/read-by-entity anymore;
//   2. after the successful read, the notification badge/list providers are
//      invalidated so they re-read the canonical backend state immediately.
import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/domains/chat/chat/data/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:hishumi/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

const _roomId = 'room_1';
const _me = 'me';

class _RecordingChatRepo implements ChatRepository {
  final List<String> markedRooms = [];

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() =>
      const Stream<ChatRoomEventDto>.empty();

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
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// Spy notification repository: records the legacy second mutation and
/// counts canonical count/list stream subscriptions so provider
/// invalidation can be proven.
class _SpyNotificationRepository implements INotificationRepository {
  int markAsReadByEntityCalls = 0;
  int countSubscriptions = 0;
  int listCalls = 0;

  @override
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async {
    markAsReadByEntityCalls++;
    return Result.success(null);
  }

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countSubscriptions++;
    return Result.success(3);
  }

  @override
  Future<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) async {
    listCalls++;
    return Result.success(const []);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

void main() {
  late _RecordingChatRepo chatRepo;
  late _SpyNotificationRepository notificationRepo;
  late ProviderContainer container;
  late ProviderSubscription<ChatDetailState> detailSub;

  setUp(() {
    chatRepo = _RecordingChatRepo();
    notificationRepo = _SpyNotificationRepository();
    container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(chatRepo),
        currentUserIdProvider.overrideWith((ref) => _me),
        notificationRepositoryProvider.overrideWithValue(notificationRepo),
      ],
    );
    detailSub = container.listen(
      chatDetailProvider(_roomId),
      (_, _) {},
      fireImmediately: true,
    );
  });

  tearDown(() {
    detailSub.close();
    container.dispose();
  });

  test(
    'chat read is a SINGLE request: no read-by-entity, badge/list re-read backend',
    () async {
      // Activate the notification providers the badge and list watch.
      container.listen(unreadCountProvider(_me), (_, _) {});
      container.listen(notificationListProvider(_me), (_, _) {});
      await pumpEventQueue();
      final countSubsBefore = notificationRepo.countSubscriptions;
      final listCallsBefore = notificationRepo.listCalls;

      await container
          .read(chatDetailProvider(_roomId).notifier)
          .markAsRead(_me);
      await pumpEventQueue();

      // The chat half ran exactly once, for this room.
      expect(chatRepo.markedRooms, [_roomId]);

      // The legacy second client mutation is GONE.
      expect(notificationRepo.markAsReadByEntityCalls, 0,
          reason:
              'chat read must no longer call POST /notifications/read-by-entity');

      // The notification badge and list converged by re-reading the canonical
      // backend state immediately (invalidate), not by waiting for polling.
      expect(notificationRepo.countSubscriptions, countSubsBefore + 1,
          reason: 'unreadCountProvider must be invalidated after chat read');
      expect(notificationRepo.listCalls, listCallsBefore + 1,
          reason: 'notificationListProvider must be invalidated after chat read');
    },
  );
}
