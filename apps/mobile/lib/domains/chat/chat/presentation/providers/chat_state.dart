import 'package:equatable/equatable.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';

/// Chat List State
class ChatListState extends Equatable {
  final List<Chat> chats;
  final bool hasMore;
  final String? nextCursor;
  final bool isLoading;
  final String? error;

  const ChatListState({
    this.chats = const [],
    this.hasMore = false,
    this.nextCursor,
    this.isLoading = false,
    this.error,
  });

  @override
  List<Object?> get props => [chats, hasMore, nextCursor, isLoading, error];

  ChatListState copyWith({
    List<Chat>? chats,
    bool? hasMore,
    String? nextCursor,
    bool? isLoading,
    String? error,
  }) {
    return ChatListState(
      chats: chats ?? this.chats,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
    );
  }

  ChatListState loading() => copyWith(isLoading: true, error: null);

  ChatListState failure(String error) =>
      copyWith(isLoading: false, error: error);
}

/// Chat Detail State
class ChatDetailState extends Equatable {
  final Chat? chat;
  final List<Message> messages;
  final bool hasMoreMessages;
  final String? nextMessageCursor;
  final Map<String, bool> typingUsers;
  final int unreadCount;
  final bool isLoading;
  final String? error;
  final String? errorCode;

  const ChatDetailState({
    this.chat,
    this.messages = const [],
    this.hasMoreMessages = false,
    this.nextMessageCursor,
    this.typingUsers = const {},
    this.unreadCount = 0,
    this.isLoading = false,
    this.error,
    this.errorCode,
  });

  @override
  List<Object?> get props => [
    chat,
    messages,
    hasMoreMessages,
    nextMessageCursor,
    typingUsers,
    unreadCount,
    isLoading,
    error,
    errorCode,
  ];

  ChatDetailState copyWith({
    Chat? chat,
    List<Message>? messages,
    bool? hasMoreMessages,
    String? nextMessageCursor,
    Map<String, bool>? typingUsers,
    int? unreadCount,
    bool? isLoading,
    String? error,
    String? errorCode,
  }) {
    return ChatDetailState(
      chat: chat ?? this.chat,
      messages: messages ?? this.messages,
      hasMoreMessages: hasMoreMessages ?? this.hasMoreMessages,
      nextMessageCursor: nextMessageCursor ?? this.nextMessageCursor,
      typingUsers: typingUsers ?? this.typingUsers,
      unreadCount: unreadCount ?? this.unreadCount,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      errorCode: errorCode ?? this.errorCode,
    );
  }

  ChatDetailState loading() =>
      copyWith(isLoading: true, error: null, errorCode: null);

  ChatDetailState failure(String error) =>
      copyWith(isLoading: false, error: error);
}

/// Message Send State
class MessageSendState extends Equatable {
  final Message? message;
  final bool isLoading;
  final String? error;
  final bool isSuccess;

  const MessageSendState({
    this.message,
    this.isLoading = false,
    this.error,
    this.isSuccess = false,
  });

  @override
  List<Object?> get props => [message, isLoading, error, isSuccess];

  MessageSendState copyWith({
    Message? message,
    bool? isLoading,
    String? error,
    bool? isSuccess,
  }) {
    return MessageSendState(
      message: message ?? this.message,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      isSuccess: isSuccess ?? this.isSuccess,
    );
  }

  MessageSendState loading() => copyWith(isLoading: true, error: null);

  MessageSendState success(Message message) => copyWith(
    isLoading: false,
    error: null,
    isSuccess: true,
    message: message,
  );

  MessageSendState failure(String error) =>
      copyWith(isLoading: false, error: error, isSuccess: false);
}

/// Typing Indicator State
class TypingState extends Equatable {
  final Map<String, bool> typingUsers; // userId -> isTyping

  const TypingState({this.typingUsers = const {}});

  @override
  List<Object?> get props => [typingUsers];

  bool isUserTyping(String userId) => typingUsers[userId] ?? false;

  TypingState copyWith({Map<String, bool>? typingUsers}) {
    return TypingState(typingUsers: typingUsers ?? this.typingUsers);
  }
}

/// Presence State
class PresenceState extends Equatable {
  final Map<String, bool> onlineUsers; // userId -> isOnline
  final Map<String, DateTime?> lastSeen; // userId -> lastSeen

  const PresenceState({this.onlineUsers = const {}, this.lastSeen = const {}});

  @override
  List<Object?> get props => [onlineUsers, lastSeen];

  bool isUserOnline(String userId) => onlineUsers[userId] ?? false;

  DateTime? getUserLastSeen(String userId) => lastSeen[userId];

  PresenceState copyWith({
    Map<String, bool>? onlineUsers,
    Map<String, DateTime?>? lastSeen,
  }) {
    return PresenceState(
      onlineUsers: onlineUsers ?? this.onlineUsers,
      lastSeen: lastSeen ?? this.lastSeen,
    );
  }
}
