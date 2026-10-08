import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';

/// Chat Repository Interface
///
/// Defines contract for chat data operations.
///
/// CANONICAL REALTIME CONTRACT: the only realtime gateway is
/// [watchChatRoomEvents] (`chat.room.created` / `chat.room.updated`,
/// user-targeted). Consumers re-fetch message bodies over REST — the WS frame
/// intentionally carries no message content (ADR-005). There is no message
/// stream, no typing stream and no read-receipt stream.
abstract class ChatRepository {
  // ========================================
  // Chat Operations
  // ========================================

  /// Get or create chat between two users
  Future<Result<Chat>> getOrCreateChat({required List<String> participantIds});

  /// Get chat by ID
  Future<Result<Chat>> getChatById(String chatId);

  /// Get user's chats with pagination
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
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
  /// A commerce product attachment travels ONLY as a canonical
  /// [ChatResourceOccurrenceRequest] (`direct_commerce_insert_chat`); there is
  /// no object-reference send path and no client attachment snapshot.
  Future<Result<Message>> sendMessage({
    required String chatId,
    required String senderId,
    required String senderName,
    required String content,
    MessageType type,
    /// Canonical chat media: the PENDING asset ids (register step) this message
    /// attaches, in display order. Raw media urls are never sent — the backend
    /// owns asset validation, ownership and the pending window.
    List<String> mediaAssetIds,
    String? replyToId,
    List<String> mentionedUserIds,
    // Explicit resource occurrence (composer direct-commerce attach).
    ChatResourceOccurrenceRequest? resourceOccurrence,
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

  // ========================================
  // Streams (Real-time)
  // ========================================

  /// Stream parsed room summary events from realtime transport.
  ///
  /// This is the gateway-only contract for `chat.room.created` and
  /// `chat.room.updated`. It never carries message bodies; consumers
  /// (chat list merge, open-thread refresh) re-read the canonical REST state.
  Stream<ChatRoomEventDto> watchChatRoomEvents();

  // NOTE: commerce write operations (negotiation, shipping quote) do NOT
  // belong here — they are owned by their commerce domains (Owner rule
  // 2026-10-01: chat never handles shipping; the Shipping domain owns the
  // manual shipping quote end-to-end).
}
