import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';

/// Chat Repository Interface
///
/// Defines contract for chat data operations.
/// Implementation can be API-based, Firestore-based, or hybrid.
abstract class ChatRepository {
  // ========================================
  // Chat Operations
  // ========================================

  /// Get or create chat between two users
  ///
  /// **SOCIAL FIX 1.1:** Context now uses ShareReference for all object references.
  Future<Result<Chat>> getOrCreateChat({
    required List<String> participantIds,
    ShareReference? context,
  });

  /// Get chat by ID
  Future<Result<Chat>> getChatById(String chatId);

  /// Get user's chats with pagination
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
  });

  /// Delete chat for specific user (soft delete)
  Future<Result<bool>> deleteChat({
    required String chatId,
    required String userId,
  });

  /// Link order to chat for commerce continuity (order↔chat alignment)
  /// Used when:
  /// - Order is created from chat (chat-born order)
  /// - User navigates from order detail to chat (direct order → chat continuity)
  Future<Result<Chat>> linkOrderToChat({
    required String roomId,
    required String orderId,
  });

  // ========================================
  // Message Operations
  // ========================================

  /// Send message
  ///
  /// **SOCIAL FIX 1:** Message attachments use ShareReference for object references.
  /// No generic Attachment parameter - construct Message with proper attachment fields.
  Future<Result<Message>> sendMessage({
    required String chatId,
    required String senderId,
    required String senderName,
    required String content,
    MessageType type,
    List<String> mediaUrls,
    String? replyToId,
    List<String> mentionedUserIds,
    // Attachment fields (ShareReference for object references)
    ShareReference? objectReference,
    Map<String, dynamic>?
    workflowAttachment, // For negotiation/shipping/location
  });

  /// Get messages with pagination
  Future<Result<List<Message>>> getMessages({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  });

  // ========================================
  // Read Receipts
  // ========================================

  /// Mark messages as read
  Future<Result<bool>> markMessagesAsRead({
    required String chatId,
    required String userId,
    List<String>? messageIds,
  });

  /// Mark message as delivered
  Future<Result<bool>> markMessageAsDelivered({
    required String chatId,
    required String messageId,
  });

  // ========================================
  // Content Validation
  // ========================================

  /// Validate message content (anti-circumvention)
  Future<Result<bool>> validateMessageContent(String content);

  // ========================================
  // Unread Count
  // ========================================

  /// Get unread count for a specific chat room.
  ///
  /// Canonical backend contract: GET /chat/rooms/:room_id/unread
  Future<Result<int>> getRoomUnreadCount(String roomId);

  // ========================================
  // Streams (Real-time)
  // ========================================

  /// Stream real-time messages
  Stream<List<Message>> watchMessages({
    required String chatId,
    required String userId,
  });

  /// Stream parsed room summary events from realtime transport.
  ///
  /// This is the gateway-only contract for `chat.room.created` and
  /// `chat.room.updated`. It does not mutate chat-list state.
  Stream<ChatRoomEventDto> watchChatRoomEvents();

  /// Stream typing indicators
  Stream<Map<String, bool>> watchTypingIndicators(String chatId);

  // ========================================
  // Presence
  // ========================================

  /// Update user's online status
  Future<Result<bool>> updateUserPresence({
    required String userId,
    required bool isOnline,
    DateTime? lastSeen,
  });

  /// Get user's online status
  Future<Result<bool>> getUserOnlineStatus(String userId);

  /// Get user's last seen time
  Future<Result<DateTime?>> getUserLastSeen(String userId);

  /// Start presence tracking
  Future<Result<bool>> startPresenceTracking(String userId);

  /// Stop presence tracking
  Future<Result<bool>> stopPresenceTracking(String userId);

  // ========================================
  // Support Ticket (Optional)
  // ========================================

  /// Get chat statistics
  Future<Result<Map<String, dynamic>>> getChatStats(String userId);

  /// Clear chat context
  Future<Result<void>> clearChatContext(String chatId);

  // ========================================
  // Commerce Operations
  // ========================================

  /// Create a shipping quote
  ///
  /// Used by sellers to provide manual shipping cost quotes to buyers.
  /// Creates a shipping quote and sends a message to the chat.
  Future<Result<Map<String, dynamic>>> createShippingQuote({
    required String chatId,
    required CreateShippingQuoteRequestDto request,
  });
}

/// Typing event from WebSocket
class TypingEvent {
  final String chatRoomId;
  final String userId;
  final String userName;
  final bool isTyping;

  const TypingEvent({
    required this.chatRoomId,
    required this.userId,
    required this.userName,
    required this.isTyping,
  });
}

/// Read receipt event from WebSocket
class ReadReceiptEvent {
  final String chatRoomId;
  final String messageId;
  final String userId;

  const ReadReceiptEvent({
    required this.chatRoomId,
    required this.messageId,
    required this.userId,
  });
}
