import 'package:equatable/equatable.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';

/// Chat List State
///
/// LOADING SEMANTICS (owner-locked Loading Foundation):
/// - [isLoading] + empty [chats] = first load (LoadingIndicator; failure →
///   PageErrorState via [error]).
/// - [isRefreshing] + non-empty [chats] = refresh. Last-known-good chats stay
///   visible; failure sets [refreshError] with an inline indication, never a
///   full-page error swap.
class ChatListState extends Equatable {
  final List<Chat> chats;
  final bool hasMore;
  final String? nextCursor;
  final bool isLoading;
  final bool isRefreshing;
  final String? error;
  final String? refreshError;

  const ChatListState({
    this.chats = const [],
    this.hasMore = false,
    this.nextCursor,
    this.isLoading = false,
    this.isRefreshing = false,
    this.error,
    this.refreshError,
  });

  @override
  List<Object?> get props => [
    chats,
    hasMore,
    nextCursor,
    isLoading,
    isRefreshing,
    error,
    refreshError,
  ];

  ChatListState copyWith({
    List<Chat>? chats,
    bool? hasMore,
    String? nextCursor,
    bool? isLoading,
    bool? isRefreshing,
    String? error,
    bool clearError = false,
    String? refreshError,
    bool clearRefreshError = false,
  }) {
    return ChatListState(
      chats: chats ?? this.chats,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      isLoading: isLoading ?? this.isLoading,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      error: clearError ? null : error ?? this.error,
      refreshError: clearRefreshError
          ? null
          : refreshError ?? this.refreshError,
    );
  }

  ChatListState loading() =>
      copyWith(isLoading: true, clearError: true, clearRefreshError: true);

  ChatListState failure(String error) =>
      copyWith(isLoading: false, isRefreshing: false, error: error);
}

/// Chat Detail State
class ChatDetailState extends Equatable {
  final Chat? chat;

  /// Thread messages in the canonical order: **newest-first** (descending
  /// `createdAt`), so index 0 is the newest message.
  ///
  /// The detail list renders with `reverse: true`, which puts index 0 at the
  /// BOTTOM of the screen. That makes the order load-bearing: appending a
  /// just-sent message to the end parks the newest bubble at the TOP until the
  /// thread is re-read. Mutate this list only through the notifier's merge.
  final List<Message> messages;
  final bool hasMoreMessages;
  final String? nextMessageCursor;
  final int unreadCount;
  final bool isLoading;
  final String? error;
  final String? errorCode;

  const ChatDetailState({
    this.chat,
    this.messages = const [],
    this.hasMoreMessages = false,
    this.nextMessageCursor,
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
