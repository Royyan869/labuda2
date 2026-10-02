// CTA LIVENESS — negotiation session re-read on incoming message (2026-09-30).
//
// The open thread refreshes its messages on `chat.room.updated` (ADR-005: the
// WS frame carries no bodies → REST re-fetch). But a fresh message can also
// CHANGE the negotiation session itself — a counter arrives, or accept /
// reject / counter emits the message — and the proposal card reads the
// SESSION for its CTA authority, not the message list. These tests pin the
// contract: a message-bearing room signal re-pulls the per-room session;
// body-less signals (read state, order link) and other-room signals do not.

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/domain/repositories/negotiation_repository.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

const _roomId = 'room_1';
const _me = 'buyer-1';
const _other = 'seller-1';

class _ThreadRepo implements ChatRepository {
  _ThreadRepo(this.events);

  final Stream<ChatRoomEventDto> events;

  /// Newest-first, exactly like the canonical REST page shape.
  List<Message> serverMessages = const [];
  int getMessagesCalls = 0;

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
    getMessagesCalls++;
    return Result.success(serverMessages);
  }

  @override
  Future<Result<bool>> markMessagesAsRead({
    required String chatId,
    required String userId,
    List<String>? messageIds,
  }) async => Result.success(true);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StubNegotiationRepository implements NegotiationRepository {
  Negotiation? session;
  int getNegotiationCalls = 0;

  @override
  Future<Result<Negotiation?>> getNegotiation({
    required String chatRoomId,
  }) async {
    getNegotiationCalls++;
    return Result.success(session);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoOpNotificationRepository implements INotificationRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

Message _message(String id, {String senderId = _other}) => Message(
  id: id,
  chatId: _roomId,
  senderId: senderId,
  senderName: senderId == _me ? 'Me' : 'Other',
  content: 'body-$id',
  // Deterministic, ascending with the m<number> suffix.
  createdAt: DateTime.utc(2026, 6, 1, int.parse(id.substring(1))),
);

ChatRoomLastMessageDto _lastMessage(Message message) => ChatRoomLastMessageDto(
  id: message.id,
  roomId: message.chatId,
  senderId: message.senderId,
  messageType: 'text',
  body: message.content,
  isHidden: false,
  createdAt: message.createdAt,
);

ChatRoomEventDto _roomEvent({
  required String roomId,
  required int unreadCount,
  Message? lastMessage,
  String? linkedOrderId,
}) => ChatRoomEventDto(
  eventType: WebSocketEventType.roomUpdated,
  roomId: roomId,
  roomType: 'direct',
  otherUserId: _other,
  linkedOrderId: linkedOrderId,
  lastMessage: lastMessage == null ? null : _lastMessage(lastMessage),
  unreadCount: unreadCount,
  createdAt: DateTime.utc(2026, 6, 1),
  updatedAt: DateTime.utc(2026, 6, 2),
  lastMessageAt: DateTime.utc(2026, 6, 2),
);

Negotiation _session({
  required int round,
  required String lastOfferBy,
  required double price,
}) => Negotiation(
  id: 'session-1',
  chatId: _roomId,
  fixedPriceSaleId: 'for-sale-1',
  forSaleName: '',
  originalPrice: 300000.0,
  buyerId: _me,
  buyerName: '',
  sellerId: _other,
  status: NegotiationStatus.active,
  currentOfferPrice: price,
  lastOfferBy: lastOfferBy,
  round: round,
  createdAt: DateTime.utc(2026, 9, 30),
  updatedAt: DateTime.utc(2026, 9, 30),
);

void main() {
  late StreamController<ChatRoomEventDto> events;
  late _ThreadRepo repo;
  late _StubNegotiationRepository negoRepo;
  late ProviderContainer container;
  late ProviderSubscription<ChatDetailState> sub;

  setUp(() {
    events = StreamController<ChatRoomEventDto>.broadcast();
    repo = _ThreadRepo(events.stream);
    negoRepo = _StubNegotiationRepository();
    container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(repo),
        negotiationRepositoryProvider.overrideWithValue(negoRepo),
        currentUserIdProvider.overrideWith((ref) => _me),
        notificationRepositoryProvider.overrideWithValue(
          _NoOpNotificationRepository(),
        ),
      ],
    );
    sub = container.listen(
      chatDetailProvider(_roomId),
      (_, _) {},
      fireImmediately: true,
    );
  });

  tearDown(() async {
    sub.close();
    container.dispose();
    await events.close();
  });

  test(
    'a message-bearing room signal re-pulls the negotiation session',
    () async {
      negoRepo.session = _session(
        round: 1,
        lastOfferBy: _me,
        price: 250000,
      );
      repo.serverMessages = [_message('m1')];
      await container
          .read(chatDetailProvider(_roomId).notifier)
          .loadMessages(_me);

      final callsBeforeMessages = repo.getMessagesCalls;
      final callsBeforeNego = negoRepo.getNegotiationCalls;

      // The seller countered: the REST page now carries m2 AND the session
      // moved to round 2. One room signal must refresh BOTH — the card reads
      // the session, so a message-only refresh would leave a stale CTA row.
      repo.serverMessages = [_message('m2'), _message('m1')];
      negoRepo.session = _session(round: 2, lastOfferBy: _other, price: 200000);
      events.add(
        _roomEvent(roomId: _roomId, unreadCount: 0, lastMessage: _message('m2')),
      );
      await pumpEventQueue();

      expect(repo.getMessagesCalls, callsBeforeMessages + 1);
      expect(negoRepo.getNegotiationCalls, callsBeforeNego + 1);
      final session = container
          .read(negotiationNotifierProvider)
          .currentNegotiation;
      expect(session?.round, 2);
      expect(session?.currentOfferPrice, 200000);
    },
  );

  test(
    'body-less room signals (read state / order link) never touch the session read',
    () async {
      negoRepo.session = _session(round: 1, lastOfferBy: _me, price: 250000);
      repo.serverMessages = [_message('m1')];
      await container
          .read(chatDetailProvider(_roomId).notifier)
          .loadMessages(_me);

      final callsBefore = negoRepo.getNegotiationCalls;
      events.add(_roomEvent(roomId: _roomId, unreadCount: 0));
      events.add(
        _roomEvent(roomId: _roomId, unreadCount: 0, linkedOrderId: 'order_1'),
      );
      await pumpEventQueue();

      expect(negoRepo.getNegotiationCalls, callsBefore);
    },
  );

  test(
    'signals for other rooms never touch this room session read',
    () async {
      negoRepo.session = _session(round: 1, lastOfferBy: _me, price: 250000);
      repo.serverMessages = [_message('m1')];
      await container
          .read(chatDetailProvider(_roomId).notifier)
          .loadMessages(_me);

      final callsBefore = negoRepo.getNegotiationCalls;
      events.add(
        _roomEvent(
          roomId: 'room_other',
          unreadCount: 5,
          lastMessage: _message('m9'),
        ),
      );
      await pumpEventQueue();

      expect(negoRepo.getNegotiationCalls, callsBefore);
    },
  );
}
