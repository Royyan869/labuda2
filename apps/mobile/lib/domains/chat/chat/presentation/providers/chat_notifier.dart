import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'chat_state.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/chat_usecases.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';

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

  /// Load user's chats
  Future<void> loadChats(String userId) async {
    // Guard against concurrent calls
    if (_isLoadingChats) return;

    try {
      _isLoadingChats = true;
      state = state.loading();

      final result = await _repository.getUserChats(userId: userId);

      result.fold((error) => state = state.failure(error), (chats) {
        final activeChats = chats.where((chat) {
          return !chat.isDeletedBy(userId);
        }).toList();

        _replaceChats(activeChats);
        state = ChatListState(chats: _sortChats(activeChats), hasMore: false);
      });
    } finally {
      _isLoadingChats = false;
    }
  }

  /// Get or create chat with another user
  ///
  /// **SOCIAL FIX 1.1:** Context now uses ShareReference for all object references.
  Future<Chat?> getOrCreateChat({
    required String userId,
    required String otherUserId,
    ShareReference? context,
  }) async {
    // Guard against concurrent calls
    if (_isGettingOrCreate) return null;

    try {
      _isGettingOrCreate = true;
      state = state.loading();

      final result = await _repository.getOrCreateChat(
        participantIds: [userId, otherUserId],
        context: context,
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
    state = ChatListState(chats: updatedChats);
  }

  void updateChat(Chat updatedChat) {
    _mergeChat(updatedChat);
  }

  void updateUnreadCount(String chatId, String userId, int newCount) {
    final updatedChats = state.chats.map((chat) {
      if (chat.id == chatId) {
        final updatedUnreadCounts = Map<String, int>.from(chat.unreadCounts);
        updatedUnreadCounts[userId] = newCount;
        return chat.copyWith(unreadCounts: updatedUnreadCounts);
      }
      return chat;
    }).toList();

    state = ChatListState(chats: updatedChats);
  }

  void clearError() {
    if (state.error != null) {
      state = state.copyWith(error: null);
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

    state = ChatListState(
      chats: shouldPreserveOrder ? nextChats : _sortChats(nextChats),
      hasMore: state.hasMore,
      nextCursor: state.nextCursor,
      isLoading: state.isLoading,
      error: state.error,
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
      context: incoming.context ?? existing.context,
      contextSetBy: incoming.contextSetBy ?? existing.contextSetBy,
      lastMessage: mergedLastMessage,
      createdAt: existing.createdAt,
      updatedAt: incoming.updatedAt ?? existing.updatedAt,
      unreadCounts: incoming.unreadCounts.isNotEmpty
          ? incoming.unreadCounts
          : existing.unreadCounts,
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
      if (event.context != null) 'context': event.context,
      if (event.contextSetBy != null) 'context_set_by': event.contextSetBy,
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
  @override
  ChatDetailState build(String chatId) {
    return const ChatDetailState();
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
          messages: activeMessages,
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
        final updatedMessages = [...state.messages, ...result.data!];
        state = state.copyWith(
          messages: updatedMessages,
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

  Future<Message?> sendMessage({
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    List<String> mediaUrls = const [],
    String? replyToId,
    List<String> mentionedUserIds = const [],
    ShareReference? objectReference,
    Map<String, dynamic>? workflowAttachment,
  }) async {
    // Guard against concurrent sends
    if (_isSending) return null;

    try {
      _isSending = true;
      final result = await _sendMessageUseCase(
        chatId: chatId,
        senderId: senderId,
        senderName: senderName,
        content: content,
        type: type,
        objectReference: objectReference,
        workflowAttachment: workflowAttachment,
      );

      if (result.isSuccess && result.data != null) {
        final updatedMessages = [...state.messages, result.data!];
        state = state.copyWith(messages: updatedMessages);
        return result.data;
      } else {
        state = state.copyWith(
          error: result.error,
          errorCode: result.errorCode,
        );
        return null;
      }
    } finally {
      _isSending = false;
    }
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

  void updateTypingUsers(Map<String, bool> typingUsers) {
    state = state.copyWith(typingUsers: typingUsers);
  }

  void addMessage(Message message) {
    if (state.messages.any((msg) => msg.id == message.id)) return;

    final updatedMessages = [...state.messages, message];
    state = state.copyWith(messages: updatedMessages);
  }

  void updateMessage(Message updatedMessage) {
    final updatedMessages = state.messages.map((msg) {
      return msg.id == updatedMessage.id ? updatedMessage : msg;
    }).toList();

    state = state.copyWith(messages: updatedMessages);
  }

  /// Create a shipping quote
  ///
  /// Used by sellers to provide manual shipping cost quotes to buyers.
  /// Creates a shipping quote and sends a message to the chat.
  Future<void> createShippingQuote(
    CreateShippingQuoteRequestDto request,
  ) async {
    final repository = ref.read(chatRepositoryProvider);
    final result = await repository.createShippingQuote(
      chatId: chatId,
      request: request,
    );

    if (result.isFailure) {
      state = state.copyWith(error: result.error);
      throw Exception(result.error ?? 'Failed to create shipping quote');
    }
  }

  void clearError() {
    if (state.error != null) {
      state = state.copyWith(error: null);
    }
  }

  String? _encodeMessageCursorFromMessages(List<Message> messages) {
    if (messages.isEmpty) return null;
    final lastMessage = messages.last;
    return '${lastMessage.createdAt.toUtc().toIso8601String()}|${lastMessage.id}';
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

class _MessageCursor {
  final DateTime createdAt;
  final String messageId;

  const _MessageCursor({required this.createdAt, required this.messageId});
}

// ========================================
// Unread Count Notifier
// ========================================

/// Unread Count Notifier - Manages unread message counts
@riverpod
class UnreadCount extends _$UnreadCount {
  @override
  Map<String, int> build() {
    return {};
  }

  void syncFromChats(List<Chat> chats, String currentUserId) {
    final next = <String, int>{};
    for (final chat in chats) {
      next[chat.id] = chat.getUnreadCount(currentUserId);
    }
    state = next;
  }

  void updateChatUnread(String chatId, int count) {
    state = {...state, chatId: count};
  }

  int get totalUnreadCount => state.values.fold(0, (sum, count) => sum + count);

  int getChatUnread(String chatId) => state[chatId] ?? 0;
}

// ========================================
// Presence Notifier
// ========================================

/// Presence Notifier - Manages user online status
@riverpod
class Presence extends _$Presence {
  @override
  PresenceState build() {
    return const PresenceState();
  }

  ManagePresenceUseCase get _managePresenceUseCase =>
      ref.read(managePresenceUseCaseProvider);

  Future<void> startTracking(String userId, List<String> userIds) async {
    await _managePresenceUseCase.startTracking(userId);
  }

  Future<void> stopTracking(String userId) async {
    await _managePresenceUseCase.stopTracking(userId);
  }

  void updatePresence(String userId, bool isOnline, DateTime? lastSeen) {
    final updatedOnline = Map<String, bool>.from(state.onlineUsers);
    final updatedLastSeen = Map<String, DateTime?>.from(state.lastSeen);

    updatedOnline[userId] = isOnline;
    updatedLastSeen[userId] = lastSeen;

    state = PresenceState(
      onlineUsers: updatedOnline,
      lastSeen: updatedLastSeen,
    );
  }

  bool isUserOnline(String userId) => state.isUserOnline(userId);

  DateTime? getUserLastSeen(String userId) => state.getUserLastSeen(userId);
}
