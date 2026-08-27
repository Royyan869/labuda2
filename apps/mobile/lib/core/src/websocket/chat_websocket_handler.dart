import 'dart:async';
import 'package:logging/logging.dart';
import '../../websocket/websocket_service.dart';
import '../../websocket/websocket_message.dart';

/// Handles chat-specific WebSocket messages
class ChatWebSocketHandler {
  final Logger _logger = Logger('ChatWebSocketHandler');
  final WebSocketService _webSocketService;

  // Stream controllers for different chat events
  final _messageController = StreamController<ChatMessage>.broadcast();
  final _typingController = StreamController<TypingIndicator>.broadcast();
  final _readReceiptController = StreamController<ReadReceipt>.broadcast();
  final _messageStatusController = StreamController<MessageStatus>.broadcast();

  // Subscribed rooms (chatIds)
  final Set<String> _subscribedChats = {};

  ChatWebSocketHandler(this._webSocketService) {
    _initializeMessageListener();
  }

  /// Stream for incoming chat messages
  Stream<ChatMessage> get messages => _messageController.stream;

  /// Stream for typing indicators
  Stream<TypingIndicator> get typingIndicators => _typingController.stream;

  /// Stream for read receipts
  Stream<ReadReceipt> get readReceipts => _readReceiptController.stream;

  /// Stream for message status updates (sent, delivered, read)
  Stream<MessageStatus> get messageStatus => _messageStatusController.stream;

  void _initializeMessageListener() {
    _webSocketService.messages.listen((wsMessage) {
      try {
        _handleWebSocketMessage(wsMessage);
      } catch (e) {
        _logger.severe('Error handling WebSocket message: $e');
      }
    });
  }

  void _handleWebSocketMessage(WebSocketMessage wsMessage) {
    switch (wsMessage.type) {
      case 'chat':
        _handleChatMessage(wsMessage);
        break;
      case 'typing':
        _handleTypingIndicator(wsMessage);
        break;
      case 'read_receipt':
        _handleReadReceipt(wsMessage);
        break;
      case 'message_status':
        _handleMessageStatus(wsMessage);
        break;
      default:
        _logger.fine('Unhandled message type: ${wsMessage.type}');
    }
  }

  void _handleChatMessage(WebSocketMessage wsMessage) {
    try {
      final chatMessage = ChatMessage.fromWebSocketData(wsMessage.data);
      _messageController.add(chatMessage);
    } catch (e) {
      _logger.warning('Failed to parse chat message: $e');
    }
  }

  void _handleTypingIndicator(WebSocketMessage wsMessage) {
    try {
      final typing = TypingIndicator.fromWebSocketData(wsMessage.data);
      _typingController.add(typing);
    } catch (e) {
      _logger.warning('Failed to parse typing indicator: $e');
    }
  }

  void _handleReadReceipt(WebSocketMessage wsMessage) {
    try {
      final receipt = ReadReceipt.fromWebSocketData(wsMessage.data);
      _readReceiptController.add(receipt);
    } catch (e) {
      _logger.warning('Failed to parse read receipt: $e');
    }
  }

  void _handleMessageStatus(WebSocketMessage wsMessage) {
    try {
      final status = MessageStatus.fromWebSocketData(wsMessage.data);
      _messageStatusController.add(status);
    } catch (e) {
      _logger.warning('Failed to parse message status: $e');
    }
  }

  /// Subscribe to a chat room for real-time updates
  Future<void> subscribeToChat(String chatId) async {
    if (_subscribedChats.contains(chatId)) {
      _logger.fine('Already subscribed to chat: $chatId');
      return;
    }

    try {
      await _webSocketService.subscribeToRoom(chatId);
      _subscribedChats.add(chatId);
      _logger.info('Subscribed to chat: $chatId');
    } catch (e) {
      _logger.severe('Failed to subscribe to chat $chatId: $e');
      rethrow;
    }
  }

  /// Unsubscribe from a chat room
  Future<void> unsubscribeFromChat(String chatId) async {
    if (!_subscribedChats.contains(chatId)) {
      _logger.fine('Not subscribed to chat: $chatId');
      return;
    }

    try {
      await _webSocketService.unsubscribeFromRoom(chatId);
      _subscribedChats.remove(chatId);
      _logger.info('Unsubscribed from chat: $chatId');
    } catch (e) {
      _logger.warning('Failed to unsubscribe from chat $chatId: $e');
    }
  }

  /// Unsubscribe from all chats
  Future<void> unsubscribeAll() async {
    final chatsToUnsubscribe = List<String>.from(_subscribedChats);
    for (final chatId in chatsToUnsubscribe) {
      await unsubscribeFromChat(chatId);
    }
  }

  /// Send a chat message via WebSocket
  Future<void> sendMessage({
    required String chatId,
    required String messageId,
    required String content,
    String? replyToId,
    List<String> mediaUrls = const [],
    Map<String, dynamic>? attachment,
  }) async {
    final message = WebSocketMessage(
      type: 'chat',
      from: '', // Will be set by server from auth token
      data: {
        'chat_id': chatId,
        'message_id': messageId,
        'content': content,
        'reply_to_id': ?replyToId,
        if (mediaUrls.isNotEmpty) 'media_urls': mediaUrls,
        'attachment': ?attachment,
      },
    );

    await _webSocketService.send(message, requireAck: true);
    _logger.fine('Sent message to chat $chatId');
  }

  /// Send typing indicator
  Future<void> sendTypingIndicator({
    required String chatId,
    required bool isTyping,
  }) async {
    final message = WebSocketMessage(
      type: 'typing',
      from: '',
      data: {'chat_id': chatId, 'is_typing': isTyping},
    );

    // Don't require ACK for typing indicators (fire and forget)
    await _webSocketService.send(message, requireAck: false);
  }

  /// Send read receipt
  Future<void> sendReadReceipt({
    required String chatId,
    required List<String> messageIds,
  }) async {
    final message = WebSocketMessage(
      type: 'read_receipt',
      from: '',
      data: {'chat_id': chatId, 'message_ids': messageIds},
    );

    await _webSocketService.send(message, requireAck: false);
  }

  /// Check if subscribed to a chat
  bool isSubscribedTo(String chatId) => _subscribedChats.contains(chatId);

  /// Get list of subscribed chats
  List<String> get subscribedChats => List.unmodifiable(_subscribedChats);

  /// Dispose resources
  void dispose() {
    _messageController.close();
    _typingController.close();
    _readReceiptController.close();
    _messageStatusController.close();
  }
}

/// Chat message received via WebSocket
class ChatMessage {
  final String messageId;
  final String chatId;
  final String senderId;
  final String senderName;
  final String content;
  final String? replyToId;
  final List<String> mediaUrls;
  final Map<String, dynamic>? attachment;
  final DateTime timestamp;

  ChatMessage({
    required this.messageId,
    required this.chatId,
    required this.senderId,
    required this.senderName,
    required this.content,
    this.replyToId,
    this.mediaUrls = const [],
    this.attachment,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory ChatMessage.fromWebSocketData(Map<String, dynamic> data) {
    return ChatMessage(
      messageId: data['message_id'] as String,
      chatId: data['chat_id'] as String,
      senderId: data['sender_id'] as String,
      senderName: data['sender_name'] as String,
      content: data['content'] as String,
      replyToId: data['reply_to_id'] as String?,
      mediaUrls: List<String>.from(data['media_urls'] ?? []),
      attachment: data['attachment'] as Map<String, dynamic>?,
      timestamp: data['timestamp'] != null
          ? DateTime.parse(data['timestamp'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
    'message_id': messageId,
    'chat_id': chatId,
    'sender_id': senderId,
    'sender_name': senderName,
    'content': content,
    'reply_to_id': replyToId,
    'media_urls': mediaUrls,
    'attachment': attachment,
    'timestamp': timestamp.toIso8601String(),
  };
}

/// Typing indicator
class TypingIndicator {
  final String chatId;
  final String userId;
  final String userName;
  final bool isTyping;
  final DateTime timestamp;

  TypingIndicator({
    required this.chatId,
    required this.userId,
    required this.userName,
    required this.isTyping,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory TypingIndicator.fromWebSocketData(Map<String, dynamic> data) {
    return TypingIndicator(
      chatId: data['chat_id'] as String,
      userId: data['user_id'] as String,
      userName: data['user_name'] as String,
      isTyping: data['is_typing'] as bool,
      timestamp: data['timestamp'] != null
          ? DateTime.parse(data['timestamp'] as String)
          : DateTime.now(),
    );
  }
}

/// Read receipt
class ReadReceipt {
  final String chatId;
  final String userId;
  final List<String> messageIds;
  final DateTime timestamp;

  ReadReceipt({
    required this.chatId,
    required this.userId,
    required this.messageIds,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory ReadReceipt.fromWebSocketData(Map<String, dynamic> data) {
    return ReadReceipt(
      chatId: data['chat_id'] as String,
      userId: data['user_id'] as String,
      messageIds: List<String>.from(data['message_ids'] ?? []),
      timestamp: data['timestamp'] != null
          ? DateTime.parse(data['timestamp'] as String)
          : DateTime.now(),
    );
  }
}

/// Message status update (sent, delivered, read)
class MessageStatus {
  final String messageId;
  final String chatId;
  final String status; // 'sent', 'delivered', 'read'
  final DateTime timestamp;

  MessageStatus({
    required this.messageId,
    required this.chatId,
    required this.status,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory MessageStatus.fromWebSocketData(Map<String, dynamic> data) {
    return MessageStatus(
      messageId: data['message_id'] as String,
      chatId: data['chat_id'] as String,
      status: data['status'] as String,
      timestamp: data['timestamp'] != null
          ? DateTime.parse(data['timestamp'] as String)
          : DateTime.now(),
    );
  }

  bool get isSent => status == 'sent';
  bool get isDelivered => status == 'delivered';
  bool get isRead => status == 'read';
}
