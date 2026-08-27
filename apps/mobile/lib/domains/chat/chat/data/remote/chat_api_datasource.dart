import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';

/// Chat API Datasource
///
/// Handles HTTP calls to Go backend for chat operations.
class ChatApiDatasource extends BaseApiRepository {
  ChatApiDatasource(super.apiClient, {super.logger});

  // ========================================
  // Room Operations
  // ========================================

  /// Get or create a direct chat room with another user
  ///
  /// If [context] is provided, it will be attached to the room for commerce features.
  /// For new rooms: Creates with the provided context.
  /// For existing rooms without context: Updates with the provided context.
  /// For existing rooms with context: Keeps existing context (not overwritten).
  Future<Result<ChatDto>> getOrCreateDirectRoom(
    String otherUserId, {
    Map<String, dynamic>? context,
  }) async {
    final body = context != null ? {'context': context} : null;

    return executeRequest(
      () => apiClient.post('/chat/direct/$otherUserId', data: body),
      parser: (data) => ChatDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Link an order to a chat room for commerce continuity.
  ///
  /// This enables order↔chat alignment by linking an order to an existing chat room.
  /// Used when:
  /// - Order is created from chat (chat-born order)
  /// - User navigates from order detail to chat (direct order → chat continuity)
  Future<Result<ChatDto>> linkOrderToChat(String roomId, String orderId) async {
    return executeRequest(
      () => apiClient.put(
        '/chat/rooms/$roomId/link-order',
        data: {'order_id': orderId},
      ),
      parser: (data) => ChatDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get chat room by linked order ID for commerce continuity.
  ///
  /// This retrieves the chat room associated with an order, including last 50 messages.
  /// Used for dispute dashboard to show chat context.
  Future<Result<Map<String, dynamic>>> getRoomByOrderId(String orderId) async {
    return executeRequest(
      () => apiClient.get('/chat/rooms/by-order/$orderId'),
      parser: (data) => data as Map<String, dynamic>,
    );
  }

  /// List all chat rooms for the authenticated user.
  ///
  /// Uses cursor-based pagination.
  /// Returns rooms ordered by last_message_at DESC (newest first).
  Future<Result<List<ChatDto>>> listRooms({
    String? cursorLastMessageAt,
    String? cursorId,
    int limit = 50,
  }) async {
    final queryParams = <String, dynamic>{
      'limit': limit,
      if (cursorLastMessageAt != null)
        'cursor_last_message_at': cursorLastMessageAt,
      if (cursorId != null) 'cursor_id': cursorId,
    };

    return executeRequest(
      () => apiClient.get('/chat/rooms', queryParameters: queryParams),
      parser: (data) {
        final list = data['data'] as List<dynamic>;
        return list
            .map((e) => ChatDto.fromJson(e as Map<String, dynamic>))
            .toList();
      },
    );
  }

  /// Get unread count for a specific chat room.
  ///
  /// Returns the number of unread messages for the authenticated user.
  Future<Result<int>> getUnreadCount(String roomId) async {
    return executeRequest(
      () => apiClient.get('/chat/rooms/$roomId/unread'),
      parser: (data) => data['unread_count'] as int,
    );
  }

  // ========================================
  // Message Operations
  // ========================================

  /// Send a message to a chat room
  Future<Result<MessageDto>> sendMessage(
    String chatRoomId,
    SendMessageDto request,
  ) async {
    return executeRequest(
      () => apiClient.post(
        '/chat/rooms/$chatRoomId/messages',
        data: request.toJson(),
      ),
      parser: (data) => MessageDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// List messages in a chat room with pagination
  Future<Result<ListMessagesDto>> listMessages(
    String chatRoomId, {
    DateTime? cursorCreatedAt,
    String? cursorId,
    int limit = 20,
  }) async {
    final queryParams = <String, dynamic>{
      'limit': limit,
      if (cursorCreatedAt != null)
        'cursor_created_at': cursorCreatedAt.toUtc().toIso8601String(),
      if (cursorId != null && cursorId.isNotEmpty) 'cursor_id': cursorId,
    };

    return executeRequest(
      () => apiClient.get(
        '/chat/rooms/$chatRoomId/messages',
        queryParameters: queryParams,
      ),
      parser: (data) => ListMessagesDto.fromJson(data as Map<String, dynamic>),
    );
  }

  // ========================================
  // Read Receipts
  // ========================================

  /// Mark messages as read in a chat room
  Future<Result<void>> markMessagesAsRead(
    String chatRoomId,
    MarkReadDto request,
  ) async {
    return executeRequest(
      () => apiClient.post(
        '/chat/rooms/$chatRoomId/read',
        data: request.toJson(),
      ),
      parser: (_) {},
    );
  }

  // ========================================
  // Commerce Operations
  // ========================================

  /// Create a shipping quote for a chat
  ///
  /// Used by sellers to provide manual shipping cost quotes to buyers.
  /// Creates a shipping quote and sends a message to the chat.
  Future<Result<Map<String, dynamic>>> createShippingQuote(
    String chatRoomId,
    Map<String, dynamic> request,
  ) async {
    return executeRequest(
      () => apiClient.post('/chat/$chatRoomId/shipping-quote', data: request),
      parser: (data) => data as Map<String, dynamic>,
    );
  }
}
