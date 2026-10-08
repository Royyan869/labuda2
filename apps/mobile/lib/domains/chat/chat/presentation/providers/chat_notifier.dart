import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'chat_state.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart'
    show WebSocketEventType;
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/chat_usecases.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;

part 'chat_notifier.g.dart';

// ========================================
// Chat List Notifier
// ========================================

/// Chat List Notifier - Manages chat list state
@riverpod
class ChatList extends _$ChatList {
  // Concurrency guards to prevent race conditions
  bool _isLoadingChats = false;
  bool _isGettingOrCreate = false;
  final Map<String, DateTime> _roomLastMessageAtByChatId = {};

  @override
  ChatListState build() {
    ref.listen(chatRoomEventsProvider, (_, next) {
      next.when(
        data: _handleChatRoomEvent,
        loading: () {},
        error: (Object? error, StackTrace? stackTrace) {
          // Room-event transport failures are non-fatal. Keep the
          // chat-list state alive and let REST remain the recovery path.
        },
      );
    });
    ref.onDispose(_roomLastMessageAtByChatId.clear);
    return const ChatListState();
  }

  ChatRepository get _repository => ref.read(chatRepositoryProvider);

  /// Load user's chats.
  ///
  /// LOADING FOUNDATION CONTRACT:
  /// - First load (no chats cached): [ChatListState.isLoading] drives the
  ///   full-content [LoadingIndicator]; failure sets [ChatListState.error]
  ///   (PageErrorState).
  /// - Refresh ([isRefresh] with chats cached): chats are NEVER cleared.
  ///   [ChatListState.isRefreshing] drives the update indicator while
  ///   last-known-good chats stay visible; failure sets
  ///   [ChatListState.refreshError] with an inline indication.
  Future<void> loadChats(String userId, {bool isRefresh = false}) async {
    // Guard against concurrent calls
    if (_isLoadingChats) return;

    final refreshingWithData = isRefresh && state.chats.isNotEmpty;
    final snapshot = List.of(state.chats);

    try {
      _isLoadingChats = true;
      if (refreshingWithData) {
        state = state.copyWith(isRefreshing: true, clearRefreshError: true);
      } else {
        state = state.loading();
      }

      final result = await _repository.getUserChats(userId: userId);

      result.fold(
        (error) {
          if (refreshingWithData) {
            state = state.copyWith(
              chats: snapshot,
              isLoading: false,
              isRefreshing: false,
              refreshError: error,
            );
          } else {
            state = state.failure(error);
          }
        },
        (chats) {
          final activeChats = chats.where((chat) {
            return !chat.isDeletedBy(userId);
          }).toList();

          _replaceChats(activeChats);
          state = ChatListState(chats: _sortChats(activeChats), hasMore: false);
        },
      );
    } finally {
      _isLoadingChats = false;
    }
  }

  /// Get or create chat with another user
  Future<Chat?> getOrCreateChat({
    required String userId,
    required String otherUserId,
  }) async {
    // Guard against concurrent calls
    if (_isGettingOrCreate) return null;

    try {
      _isGettingOrCreate = true;
      state = state.loading();

      final result = await _repository.getOrCreateChat(
        participantIds: [userId, otherUserId],
      );

      return result.fold(
        (error) {
          state = state.failure(error);
          return null;
        },
        (chat) {
          _mergeChat(chat);
          return chat;
        },
      );
    } finally {
      _isGettingOrCreate = false;
    }
  }

  void removeChat(String chatId) {
    _roomLastMessageAtByChatId.remove(chatId);
    final updatedChats = state.chats
        .where((chat) => chat.id != chatId)
        .toList();
    state = state.copyWith(chats: updatedChats);
  }

  void updateChat(Chat updatedChat) {
    _mergeChat(updatedChat);
  }

  /// Marks every room that has unread messages as read, then zeroes the local
  /// badges. Uses the same canonical room-unread value as the list badge, so a
  /// room is only marked when it actually has unread messages.
  Future<void> markAllRead(String userId) async {
    final unreadRooms = state.chats
        .where((chat) => chat.roomUnreadCount > 0)
        .toList();
    for (final chat in unreadRooms) {
      try {
        await ref.read(chatDetailProvider(chat.id).notifier).markAsRead(userId);
      } catch (_) {
        // One failed room must not block the remaining rooms.
      }
    }
    state = state.copyWith(
      chats: [for (final chat in state.chats) chat.copyWith(unreadCount: 0)],
    );
  }

  void clearError() {
    if (state.error != null || state.refreshError != null) {
      state = state.copyWith(clearError: true, clearRefreshError: true);
    }
  }

  /// Dismisses the inline refresh-failure indication; cached chats stay.
  void clearRefreshError() {
    if (state.refreshError != null) {
      state = state.copyWith(clearRefreshError: true);
    }
  }

  void _handleChatRoomEvent(ChatRoomEventDto event) {
    try {
      final merged = _chatFromRoomEvent(event);
      _mergeChat(merged, lastMessageAt: event.lastMessageAt);
    } catch (_) {
      // Fail closed: malformed or unexpected room-event payloads are
      // ignored so the list stays on the last known good snapshot.
    }
  }

  void _mergeChat(Chat chat, {DateTime? lastMessageAt}) {
    final existingIndex = state.chats.indexWhere((item) => item.id == chat.id);
    final previousChats = state.chats;

    final derivedLastMessageAt = lastMessageAt ?? _lastMessageAtForChat(chat);
    final previousLastMessageAt = _roomLastMessageAtByChatId[chat.id];

    _roomLastMessageAtByChatId[chat.id] = derivedLastMessageAt;

    final nextChats = <Chat>[...previousChats];
    if (existingIndex >= 0) {
      final existing = nextChats[existingIndex];
      nextChats[existingIndex] = _mergeChatFields(
        existing: existing,
        incoming: chat,
      );
    } else {
      nextChats.add(chat);
    }

    final shouldPreserveOrder =
        existingIndex >= 0 &&
        previousLastMessageAt != null &&
        previousLastMessageAt == derivedLastMessageAt;

    state = state.copyWith(
      chats: shouldPreserveOrder ? nextChats : _sortChats(nextChats),
    );
  }

  Chat _mergeChatFields({required Chat existing, required Chat incoming}) {
    final mergedLastMessage = _mergeLastMessage(
      existing: existing,
      incoming: incoming,
    );

    return existing.copyWith(
      type: incoming.type,
      participantIds: incoming.participantIds.isNotEmpty
          ? incoming.participantIds
          : existing.participantIds,
      participantNames: incoming.participantNames.isNotEmpty
          ? incoming.participantNames
          : existing.participantNames,
      participantAvatars: incoming.participantAvatars.isNotEmpty
          ? incoming.participantAvatars
          : existing.participantAvatars,
      participantLifecycles: incoming.participantLifecycles.isNotEmpty
          ? incoming.participantLifecycles
          : existing.participantLifecycles,
      lastMessage: mergedLastMessage,
      createdAt: existing.createdAt,
      updatedAt: incoming.updatedAt ?? existing.updatedAt,
      // The server is the unread authority: adopt the incoming count when the
      // payload carried one (null = not provided → preserve the known value).
      unreadCount: incoming.unreadCount ?? existing.unreadCount,
      linkedOrderId: incoming.linkedOrderId ?? existing.linkedOrderId,
    );
  }

  Message? _mergeLastMessage({required Chat existing, required Chat incoming}) {
    return incoming.lastMessage ?? existing.lastMessage;
  }

  List<Chat> _sortChats(List<Chat> chats) {
    final sorted = [...chats];
    sorted.sort((a, b) {
      final lastMessageAtA = _lastMessageAtForChat(a);
      final lastMessageAtB = _lastMessageAtForChat(b);
      final lastMessageComparison = lastMessageAtB.compareTo(lastMessageAtA);
      if (lastMessageComparison != 0) {
        return lastMessageComparison;
      }

      final updatedAtA = a.updatedAt ?? a.createdAt;
      final updatedAtB = b.updatedAt ?? b.createdAt;
      final updatedComparison = updatedAtB.compareTo(updatedAtA);
      if (updatedComparison != 0) {
        return updatedComparison;
      }

      return a.id.compareTo(b.id);
    });
    return sorted;
  }

  DateTime _lastMessageAtForChat(Chat chat) {
    return _roomLastMessageAtByChatId[chat.id] ??
        chat.lastMessage?.createdAt ??
        chat.updatedAt ??
        chat.createdAt;
  }

  void _replaceChats(List<Chat> chats) {
    _roomLastMessageAtByChatId.clear();
    for (final chat in chats) {
      _roomLastMessageAtByChatId[chat.id] = _lastMessageAtForChat(chat);
    }
  }

  Chat _chatFromRoomEvent(ChatRoomEventDto event) {
    final payload = <String, dynamic>{
      'id': event.roomId,
      'room_type': event.roomType,
      'other_user_id': event.otherUserId,
      if (event.otherUser != null)
        'other_user': <String, dynamic>{
          'id': event.otherUser!.id,
          'username':
              event.otherUser!.username ?? event.otherUser!.displayName ?? '',
          if (event.otherUser!.displayName != null)
            'display_name': event.otherUser!.displayName,
          if (event.otherUser!.avatarUrl != null)
            'avatar_url': event.otherUser!.avatarUrl,
          if (event.otherUser!.lifecycle != null)
            'lifecycle': event.otherUser!.lifecycle,
        },
      if (event.linkedOrderId != null) 'linked_order_id': event.linkedOrderId,
      if (event.lastMessage != null)
        'last_message': <String, dynamic>{
          'id': event.lastMessage!.id,
          'room_id': event.lastMessage!.roomId,
          'sender_id': event.lastMessage!.senderId,
          'sender_name':
              event.otherUser?.displayName ?? event.otherUser?.username ?? '',
          'message_type': event.lastMessage!.messageType,
          if (event.lastMessage!.body != null) 'body': event.lastMessage!.body,
          if (event.lastMessage!.attachmentJson != null)
            'attachment': event.lastMessage!.attachmentJson,
          'is_hidden': event.lastMessage!.isHidden,
          'created_at': event.lastMessage!.createdAt.toIso8601String(),
        },
      'unread_count': event.unreadCount,
      'created_at': event.createdAt.toIso8601String(),
      'updated_at': event.updatedAt.toIso8601String(),
    };

    final dto = ChatDto.fromJson(payload);
    return ChatMapper.toDomain(dto);
  }
}

// ========================================
// Chat Detail Notifier
// ========================================

/// Chat Detail Notifier - Manages single chat state
@riverpod
class ChatDetail extends _$ChatDetail {
  // Concurrency guards to prevent race conditions
  bool _isLoadingMessages = false;
  bool _isLoadingMore = false;
  bool _isSending = false;
  bool _isRefreshingFromEvent = false;

  @override
  ChatDetailState build(String chatId) {
    // CANONICAL REALTIME AUTHORITY FOR AN OPEN THREAD.
    //
    // The backend emits `chat.room.updated` (user-targeted, viewer-scoped)
    // for every message sent to this room, for moderation hide/restore, for
    // read-state changes and for order linking. The WS frame intentionally
    // carries no message body (ADR-005), so the thread re-fetches over REST —
    // one authority, one refresh path. Message-level WS signals
    // (`chat.message.sent` minimal envelope) require a room subscription and
    // are NOT this authority.
    ref.listen(chatRoomEventsProvider, (_, next) {
      next.when(
        data: _handleRoomEvent,
        loading: () {},
        error: (Object? error, StackTrace? stackTrace) {
          // Room-event transport failures are non-fatal: REST reload on
          // next open remains the recovery path.
        },
      );
    });
    return const ChatDetailState();
  }

  void _handleRoomEvent(ChatRoomEventDto event) {
    if (event.roomId != chatId ||
        event.eventType != WebSocketEventType.roomUpdated) {
      return;
    }
    unawaited(_refreshFromRoomEvent(event));
  }

  /// Refreshes the open thread from REST after a `chat.room.updated` signal.
  ///
  /// Always re-fetches (never trusts the event body as message content):
  /// a moderation tombstone for a mid-thread message produces no distinct
  /// payload, so an id-based short-circuit would silently keep hidden
  /// content on screen.
  Future<void> _refreshFromRoomEvent(ChatRoomEventDto event) async {
    if (_isRefreshingFromEvent) return;
    _isRefreshingFromEvent = true;
    try {
      final userId = ref.read(currentUserIdProvider);
      try {
        await _refreshThread(event, userId);
      } catch (_) {
        // A realtime refresh is best-effort: the thread keeps its last known
        // good state and REST reload on next open/event remains the recovery
        // path. Never let a transport/decode failure break the open screen.
      }
    } finally {
      _isRefreshingFromEvent = false;
    }
  }

  Future<void> _refreshThread(ChatRoomEventDto event, String userId) async {
    final result = await _getMessagesUseCase(
      chatId: chatId,
      userId: userId,
      limit: 50,
    );
    // Same client-side filter as the canonical initial load.
    final active = <Message>[
      if (result.isSuccess && result.data != null)
        ...result.data!.where((message) => !message.isDeletedBy(userId)),
    ];
    if (active.isNotEmpty) {
      _mergeRefreshedMessages(active);
    }

    // Room summary drift that changes a canonical chat field (order link)
    // needs the room read as well; anything else is already loaded.
    if (event.linkedOrderId != null &&
        event.linkedOrderId != state.chat?.linkedOrderId) {
      await loadChat(userId);
    }

    // A message that arrived while the user is looking at this thread must
    // clear its own unread count through the canonical read authority.
    final incoming = active.isEmpty ? null : active.first;
    if (event.unreadCount > 0 &&
        userId.isNotEmpty &&
        incoming != null &&
        incoming.senderId != userId) {
      await markAsRead(userId);
    }

    // CTA LIVENESS: a message can also move the negotiation session — a
    // counter arrives, or accept / reject / counter emits the proposal
    // message. The proposal card reads the SESSION for its CTA authority,
    // not the message list, so a message-bearing signal re-pulls the
    // per-room session (exact-set contract: it also clears a session the
    // server no longer returns). Body-less signals (read state, order link)
    // leave the session read alone. Best-effort with an isolated catch: a
    // failing session read must never break the message refresh or the
    // read-marking above — the card keeps its last known session.
    if (event.lastMessage != null) {
      try {
        await ref
            .read(negotiationNotifierProvider.notifier)
            .getNegotiation(chatRoomId: chatId);
      } catch (_) {
        // Session liveness is display refresh, not message truth.
      }
    }
  }

  /// Merges the canonical latest page into local state.
  ///
  /// [fetched] wins every id collision (the server is the authority on a
  /// message) and local messages the page does not carry are kept. Where they
  /// end up is decided by [_mergeNewestFirst] from their timestamps — never by
  /// where a local copy happened to sit in the list. A live refresh therefore
  /// never collapses read history and never demotes a just-sent message to the
  /// top of the screen when the server page predates it.
  void _mergeRefreshedMessages(List<Message> fetched) {
    final fetchedIds = fetched.map((message) => message.id).toSet();
    final keepOlder = state.messages.any(
      (message) => !fetchedIds.contains(message.id),
    );

    state = state.copyWith(
      messages: _mergeNewestFirst(fetched),
      hasMoreMessages: keepOlder ? state.hasMoreMessages : fetched.length >= 50,
      nextMessageCursor: keepOlder
          ? state.nextMessageCursor
          : _encodeMessageCursorFromMessages(fetched),
      error: null,
    );
  }

  /// The ONE way messages enter thread state.
  ///
  /// [incoming] wins every id collision and the result is ordered newest-first
  /// by construction, so the canonical list order cannot depend on which side a
  /// caller spliced a new message in from. A `reverse: true` list renders index
  /// 0 at the BOTTOM: appending a just-sent message to the end is what parked
  /// the newest bubble at the top of the screen until the thread was reopened.
  List<Message> _mergeNewestFirst(List<Message> incoming, {String? dropId}) {
    final seen = <String>{};
    // An optimistic id being replaced by its server twin must not survive the
    // merge, or the thread would show the same message twice.
    if (dropId != null) seen.add(dropId);
    final merged = <Message>[];
    for (final message in [...incoming, ...state.messages]) {
      if (!seen.add(message.id)) continue;
      merged.add(message);
    }
    merged.sort(_byNewestFirst);
    return merged;
  }

  /// Canonical thread order: newest first. Ties break on id descending, which is
  /// the same tiebreak the backend applies (`ORDER BY created_at DESC, id DESC`)
  /// and therefore the same order the keyset cursor is derived from.
  static int _byNewestFirst(Message a, Message b) {
    final byTime = b.createdAt.compareTo(a.createdAt);
    return byTime != 0 ? byTime : b.id.compareTo(a.id);
  }

  // UseCases injected via providers
  GetChatUseCase get _getChatUseCase => ref.read(getChatUseCaseProvider);
  GetMessagesUseCase get _getMessagesUseCase =>
      ref.read(getMessagesUseCaseProvider);
  SendMessageUseCase get _sendMessageUseCase =>
      ref.read(sendMessageUseCaseProvider);
  MarkMessagesAsReadUseCase get _markMessagesReadUseCase =>
      ref.read(markMessagesReadUseCaseProvider);

  Future<void> loadChat(String userId) async {
    state = state.copyWith(isLoading: true);

    final result = await _getChatUseCase(chatId);

    if (result.isSuccess && result.data != null) {
      state = state.copyWith(chat: result.data, isLoading: false);
    } else {
      state = state.copyWith(error: result.error, isLoading: false);
    }
  }

  Future<void> loadMessages(String userId) async {
    // Guard against concurrent calls
    if (_isLoadingMessages) return;

    try {
      _isLoadingMessages = true;
      final result = await _getMessagesUseCase(
        chatId: chatId,
        userId: userId,
        limit: 50,
      );

      if (result.isSuccess && result.data != null) {
        final activeMessages = result.data!.where((msg) {
          return !msg.isDeletedBy(userId);
        }).toList();

        state = state.copyWith(
          messages: _mergeNewestFirst(activeMessages),
          hasMoreMessages: activeMessages.length == 50,
          nextMessageCursor: _encodeMessageCursorFromMessages(activeMessages),
        );
      } else {
        state = state.copyWith(error: result.error);
      }
    } finally {
      _isLoadingMessages = false;
    }
  }

  Future<void> loadMoreMessages(String userId) async {
    // Guard against concurrent pagination
    if (_isLoadingMore) return;
    if (!state.hasMoreMessages || state.nextMessageCursor == null) return;
    final cursor = _decodeMessageCursor(state.nextMessageCursor!);
    if (cursor == null) return;

    try {
      _isLoadingMore = true;
      final result = await _getMessagesUseCase(
        chatId: chatId,
        userId: userId,
        page: 2,
        limit: 50,
        cursorCreatedAt: cursor.createdAt,
        cursorId: cursor.messageId,
      );

      if (result.isSuccess && result.data != null) {
        state = state.copyWith(
          messages: _mergeNewestFirst(result.data!),
          hasMoreMessages: result.data!.length == 50,
          nextMessageCursor: _encodeMessageCursorFromMessages(result.data!),
        );
      } else {
        state = state.copyWith(error: result.error);
      }
    } finally {
      _isLoadingMore = false;
    }
  }

  /// Payloads of in-flight/failed sends, keyed by the optimistic message id, so
  /// "Coba lagi" can replay the exact request (media asset ids included) instead
  /// of asking the user to pick files again.
  final Map<String, _PendingSend> _pendingSends = {};

  Future<Message?> sendMessage({
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,

    /// Canonical chat media: PENDING asset ids from the room media register
    /// step, in display order. The message attaches them atomically — raw media
    /// urls are never part of the send contract.
    List<String> mediaAssetIds = const [],
    String? replyToId,
    List<String> mentionedUserIds = const [],
    ChatResourceOccurrenceRequest? resourceOccurrence,
  }) async {
    // Guard against concurrent sends
    if (_isSending) return null;

    // The bubble exists BEFORE the request does. A composer that only reacted
    // after the round trip (plus every media upload ahead of it) is what made a
    // tap on Send feel like nothing happened.
    final localId = _beginOptimisticSend(
      senderId: senderId,
      senderName: senderName,
      content: content,
      type: type,
      hasMedia: mediaAssetIds.isNotEmpty,
      pending: _PendingSend(
        senderId: senderId,
        senderName: senderName,
        content: content,
        type: type,
        mediaAssetIds: mediaAssetIds,
        resourceOccurrence: resourceOccurrence,
      ),
    );

    try {
      _isSending = true;
      final result = await _sendMessageUseCase(
        chatId: chatId,
        senderId: senderId,
        senderName: senderName,
        content: content,
        type: type,
        mediaAssetIds: mediaAssetIds,
        resourceOccurrence: resourceOccurrence,
      );

      if (result.isSuccess && result.data != null) {
        // The server message REPLACES the optimistic row: one row, one order
        // authority, and the local id never survives a successful send.
        _pendingSends.remove(localId);
        state = state.copyWith(
          messages: _mergeNewestFirst([result.data!], dropId: localId),
        );
        return result.data;
      } else {
        _markSendFailed(localId);
        state = state.copyWith(
          error: result.error,
          errorCode: result.errorCode,
        );
        return null;
      }
    } catch (_) {
      // A throw is a failed send too: the row stays, marked, with its retry.
      _markSendFailed(localId);
      rethrow;
    } finally {
      _isSending = false;
    }
  }

  /// Retries a failed send with its exact original payload.
  ///
  /// The optimistic row is dropped here and re-created by the canonical send
  /// path, so the retry never leaves a duplicate behind.
  Future<Message?> retrySend(String localMessageId) async {
    final pending = _pendingSends.remove(localMessageId);
    if (pending == null) return null;

    state = state.copyWith(
      messages: state.messages
          .where((message) => message.id != localMessageId)
          .toList(),
    );

    return sendMessage(
      senderId: pending.senderId,
      senderName: pending.senderName,
      content: pending.content,
      type: pending.type,
      mediaAssetIds: pending.mediaAssetIds,
      resourceOccurrence: pending.resourceOccurrence,
    );
  }

  /// Inserts the message the user just sent, marked [MessageStatus.sending].
  ///
  /// It rides the SAME list and the same order authority as server messages, so
  /// an in-flight message lands at the bottom like any newest message instead of
  /// needing a second rendering path.
  String _beginOptimisticSend({
    required String senderId,
    required String senderName,
    required String content,
    required MessageType type,
    required bool hasMedia,
    required _PendingSend pending,
  }) {
    final localId = 'local_${DateTime.now().microsecondsSinceEpoch}';
    _pendingSends[localId] = pending;

    state = state.copyWith(
      messages: _mergeNewestFirst([
        Message(
          id: localId,
          chatId: chatId,
          senderId: senderId,
          senderName: senderName,
          // A media-only send still says something while it is in flight: the
          // real media arrives with the server message that replaces this row.
          content: content.isEmpty && hasMedia ? 'Mengirim media…' : content,
          type: type,
          createdAt: DateTime.now().toUtc(),
          status: MessageStatus.sending,
        ),
      ]),
    );

    return localId;
  }

  /// Keeps a failed send in the thread as an actionable row (retry), instead of
  /// discarding it behind a transient snackbar.
  void _markSendFailed(String localId) {
    if (!_pendingSends.containsKey(localId)) return;

    state = state.copyWith(
      messages: state.messages
          .map(
            (message) => message.id == localId
                ? message.copyWith(status: MessageStatus.failed)
                : message,
          )
          .toList(),
    );
  }

  Future<void> markAsRead(String userId) async {
    await _markMessagesReadUseCase(chatId: chatId, userId: userId);

    // CHAT-NOTIFICATION SYNC: Mark chat notifications as read when chat is read
    // This syncs notification read state without merging ownership - notification
    // system remains owner of notification read truth, chat remains owner of message read truth
    try {
      final notificationRepo = ref.read(notificationRepositoryProvider);
      await notificationRepo.markAsReadByEntity(
        userId: userId,
        entityType: 'chat',
        entityId: chatId,
      );
    } catch (e) {
      // Non-fatal error - chat read sync should not break chat functionality
      // If notification sync fails, the chat read action still succeeds
    }
  }

  void updateMessage(Message updatedMessage) {
    final updatedMessages = state.messages.map((msg) {
      return msg.id == updatedMessage.id ? updatedMessage : msg;
    }).toList();

    state = state.copyWith(messages: updatedMessages);
  }

  void clearError() {
    if (state.error != null) {
      state = state.copyWith(error: null);
    }
  }

  /// Cursor for the NEXT OLDER page.
  ///
  /// Derived from the order itself (the oldest message), not from a list
  /// position: a thread list that was mutated from the wrong side would
  /// otherwise hand the server its NEWEST message as "older than this",
  /// silently re-reading the newest page as history.
  String? _encodeMessageCursorFromMessages(List<Message> messages) {
    if (messages.isEmpty) return null;
    final oldest = messages.reduce((a, b) => _byNewestFirst(a, b) <= 0 ? b : a);
    return '${oldest.createdAt.toUtc().toIso8601String()}|${oldest.id}';
  }

  _MessageCursor? _decodeMessageCursor(String raw) {
    final separator = raw.indexOf('|');
    if (separator <= 0 || separator >= raw.length - 1) return null;
    final ts = raw.substring(0, separator);
    final messageId = raw.substring(separator + 1);
    final createdAt = DateTime.tryParse(ts);
    if (createdAt == null || messageId.isEmpty) return null;
    return _MessageCursor(createdAt: createdAt, messageId: messageId);
  }
}

/// The exact payload of a send, kept so a failed message can be retried from
/// its own bubble. Media asset ids are replayed as-is: a retry never re-uploads
/// a file that already reached the room's pending window.
class _PendingSend {
  final String senderId;
  final String senderName;
  final String content;
  final MessageType type;
  final List<String> mediaAssetIds;
  final ChatResourceOccurrenceRequest? resourceOccurrence;

  const _PendingSend({
    required this.senderId,
    required this.senderName,
    required this.content,
    required this.type,
    this.mediaAssetIds = const [],
    this.resourceOccurrence,
  });
}

class _MessageCursor {
  final DateTime createdAt;
  final String messageId;

  const _MessageCursor({required this.createdAt, required this.messageId});
}
