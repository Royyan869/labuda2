import 'dart:async';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

/// Chat Repository Implementation
///
/// REST carries every read/write; the WebSocket carries ONE gateway signal
/// type for chat: `chat.room.created` / `chat.room.updated`.
///
/// CANONICAL REALTIME AUTHORITY: room summary events only. The backend emits
/// `chat.room.updated` (user-targeted, viewer-scoped) for each message sent
/// to a room, for moderation hide/restore, for read-state changes and for
/// order linking. Consumers re-fetch message bodies over REST (ADR-005: no
/// message payload in WS frames).
///
/// KILLED DESIGNS (do not reintroduce):
/// - a message-level stream (`watchMessages`) that required a room
///   subscription and had no consumer;
/// - patching thread state from an event body;
/// - typing indicators and read-receipt streams (backend has no such
///   events);
/// - `chat.message.hidden` / `chat.message.restored` / `message.read`
///   handling — those event names are not emitted by the backend.
class ChatRepositoryImpl implements ChatRepository {
  final ChatApiDatasource _apiDatasource;
  final WebSocketService _webSocketService;
  final ILoggerService _logger;

  StreamController<ChatRoomEventDto>? _chatRoomEventStreamController;

  // WebSocket event subscription
  StreamSubscription? _wsEventSubscription;

  ChatRepositoryImpl({
    required ChatApiDatasource apiDatasource,
    required WebSocketService webSocketService,
    required ILoggerService logger,
  }) : _apiDatasource = apiDatasource,
       _webSocketService = webSocketService,
       _logger = logger {
    _initializeWebSocketListener();
  }

  /// Test-only seam to drive [_handleWebSocketEvent] directly with a crafted
  /// payload. Used to verify room-event parsing and that non-gateway chat
  /// signals cannot fabricate thread content.
  void handleWebSocketEventForTest(Map<String, dynamic> eventPayload) =>
      _handleWebSocketEvent(eventPayload);

  // ========================================
  // WebSocket Integration
  // ========================================

  void _initializeWebSocketListener() {
    _wsEventSubscription = _webSocketService.messages.listen(
      _handleWebSocketEvent,
      onError: (Object error, StackTrace stackTrace) {
        // Tier 4 (Runtime Honesty): the WebSocketService now surfaces
        // frame-parse failures via stream errors instead of swallowing
        // them. Log with stack and continue — a single malformed frame
        // must not kill realtime chat for the rest of the session.
        _logger.error(
          'WebSocket message stream error (frame parse / channel error)',
          extra: {'error': error.toString()},
          stackTrace: stackTrace,
        );
      },
    );

    _webSocketService.connectionState.listen(
      (state) {
        if (state == ConnectionState.disconnected) {
          _logger.warning('WebSocket disconnected - real-time updates paused');
        } else if (state == ConnectionState.connected) {
          // Room re-joins after reconnect are owned by WebSocketService
          // itself (it re-issues the subscribe frames it initiated). Chat
          // does not subscribe to rooms: its authority is user-targeted.
          _logger.info('WebSocket connected');
        }
      },
      onError: (Object error, StackTrace stackTrace) {
        _logger.error(
          'WebSocket connectionState stream errored — '
          'realtime state may be stale, treating as disconnected',
          extra: {'error': error.toString()},
          stackTrace: stackTrace,
        );
      },
    );
  }

  void _handleWebSocketEvent(dynamic wsMessage) {
    try {
      final eventData = wsMessage is Map<String, dynamic>
          ? wsMessage
          : {'type': wsMessage.type, 'payload': wsMessage.data};

      final event = WebSocketEventDto.fromJson(eventData);

      switch (event.type) {
        case WebSocketEventType.roomCreated:
        case WebSocketEventType.roomUpdated:
          _handleRoomEvent(event);
          break;
        case WebSocketEventType.messageNew:
          // `chat.message.sent` is a room-broadcast minimal envelope
          // (room_id + message_id) and this connection never subscribes to
          // rooms. Even if it arrived, the open thread refresh is driven by
          // `chat.room.updated` — never by patching state from a signal.
          _logger.debug(
            'Ignored chat message signal — thread refresh is driven by '
            'chat.room.updated',
          );
          break;
        default:
          // Unknown chat websocket events are treated as contract drift.
          // Keep them observable so a new backend shape cannot fail silently.
          _logger.warning('Unhandled WebSocket event type: ${event.type}');
      }
    } catch (e) {
      _logger.error('Error handling WebSocket event: $e');
    }
  }

  void _handleRoomEvent(WebSocketEventDto event) {
    try {
      final roomEvent = ChatRoomEventDto.fromWebSocketEvent(event);
      final controller = _chatRoomEventStreamController;
      if (controller != null && !controller.isClosed) {
        controller.add(roomEvent);
      }
      _logger.debug(
        'Received chat room websocket event',
        extra: {
          'event_type': roomEvent.eventType.name,
          'room_id': roomEvent.roomId,
        },
      );
    } catch (e, stackTrace) {
      _logger.error('Error handling chat room websocket event: $e', stackTrace: stackTrace);
    }
  }

  // ========================================
  // Chat Operations
  // ========================================

  @override
  Future<Result<Chat>> getOrCreateChat({
    required List<String> participantIds,
  }) async {
    // participantIds should have exactly 2 users: current user and other user
    if (participantIds.length != 2) {
      return Result.error('Direct chat requires exactly 2 participants');
    }

    final otherUserId = participantIds.last;

    // Call the datasource
    final result = await _apiDatasource.getOrCreateDirectRoom(otherUserId);

    return result.fold((error) => Result.error(error), (dto) {
      // Convert ChatDto to domain Chat entity
      final chat = ChatMapper.toDomain(dto);
      // Ensure participant IDs are set correctly from the original request
      return Result.success(chat.copyWith(participantIds: participantIds));
    });
  }

  @override
  Future<Result<Chat>> getChatById(String chatId) async {
    final result = await _apiDatasource.getRoom(chatId);
    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(ChatMapper.toDomain(dto)),
    );
  }

  @override
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async {
    final result = await _apiDatasource.listRooms(limit: limit);
    return result.fold(
      (error) => Result.error(error),
      (dtos) => Result.success(ChatMapper.toDomainList(dtos)),
    );
  }

  @override
  Future<Result<Chat>> linkOrderToChat({
    required String roomId,
    required String orderId,
  }) async {
    final result = await _apiDatasource.linkOrderToChat(roomId, orderId);
    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(ChatMapper.toDomain(dto)),
    );
  }

  // ========================================
  // Message Operations
  // ========================================

  @override
  Future<Result<Message>> sendMessage({
    required String chatId,
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    List<String> mediaAssetIds = const [],
    String? replyToId,
    List<String> mentionedUserIds = const [],
    ChatResourceOccurrenceRequest? resourceOccurrence,
  }) async {
    final request = SendMessageDto(
      body: content,
      messageType: ChatMapper.messageTypeToString(type),
      idempotencyKey:
          '${senderId}_${DateTime.now().microsecondsSinceEpoch}_$chatId',
      mediaAssetIds: mediaAssetIds.isEmpty ? null : mediaAssetIds,
      replyToId: replyToId,
      mentionedUserIds: mentionedUserIds,
      // Declare what the message is about (communication reference only).
      resourceOccurrence: resourceOccurrence,
    );

    final result = await _apiDatasource.sendMessage(chatId, request);
    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(ChatMapper.messageToDomain(dto)),
    );
  }

  @override
  Future<Result<List<Message>>> getMessages({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  }) async {
    final result = await _apiDatasource.listMessages(
      chatId,
      cursorCreatedAt: cursorCreatedAt,
      cursorId: cursorId,
      limit: limit,
    );
    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(ChatMapper.messageListToDomain(dto.messages)),
    );
  }

  // ========================================
  // Read Receipts
  // ========================================

  @override
  Future<Result<bool>> markMessagesAsRead({
    required String chatId,
    required String userId,
    List<String>? messageIds,
  }) async {
    final request = MarkReadDto(timestamp: DateTime.now().toUtc());
    final result = await _apiDatasource.markMessagesAsRead(chatId, request);
    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(true),
    );
  }

  // ========================================
  // Streams (Real-time)
  // ========================================

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() {
    _chatRoomEventStreamController ??=
        StreamController<ChatRoomEventDto>.broadcast();
    return _chatRoomEventStreamController!.stream;
  }

  // ========================================
  // Cleanup
  // ========================================

  void dispose() {
    _wsEventSubscription?.cancel();
    _chatRoomEventStreamController?.close();
    _chatRoomEventStreamController = null;
  }
}
