import 'package:equatable/equatable.dart';
import 'attachment_dto.dart';

/// Message DTO from API
class MessageDto extends Equatable {
  final String id;
  final String chatRoomId;
  final String senderId;
  final String senderName;
  final String senderUsername;
  final String? senderAvatar;
  final String content;
  final bool isHidden;
  final String type;
  final List<String>? mediaUrls;
  final AttachmentDto? attachment;
  final String status;
  final bool isRead;
  final bool isEdited;
  final DateTime? editedAt;
  final String? replyToId;
  final ReplyPreviewDto? replyPreview;
  final List<String>? mentionedUserIds;
  final DateTime createdAt;
  final DateTime updatedAt;

  /// E4.1 — Sender lifecycle wire string, parsed from the nested canonical
  /// `sender` UserCard (publiccard.UserCard.Lifecycle on the message-sender
  /// seam in backend/internal/interaction/chat/delivery/http/chat_handler.go
  /// messageToResponse). Always tolerant: null / missing / unknown is
  /// converted to `ContentLifecycle.active` at the mapper layer.
  ///
  /// Backend emits `null` today (publiccard.New is still in use on the
  /// chat sender hydrator at chat_handler.go:1255-1291); this field exists
  /// so the mobile ingestion seam is in place before any backend activation.
  /// No build_runner regeneration is needed — MessageDto.fromJson is hand-
  /// written, so the new field is read inline.
  final String? senderLifecycle;

  /// B1/B2 — Seller trust lifecycle for the item referenced in this message's
  /// attachment. Emitted by the backend at the top level of attachment_json.
  /// Null when absent (legacy-safe; mapper defaults to active).
  final String? attachmentSellerTrustLifecycle;

  const MessageDto({
    required this.id,
    required this.chatRoomId,
    required this.senderId,
    required this.senderName,
    this.senderUsername = '',
    this.senderAvatar,
    required this.content,
    this.isHidden = false,
    required this.type,
    this.mediaUrls,
    this.attachment,
    required this.status,
    required this.isRead,
    required this.isEdited,
    this.editedAt,
    this.replyToId,
    this.replyPreview,
    this.mentionedUserIds,
    required this.createdAt,
    required this.updatedAt,
    this.senderLifecycle,
    this.attachmentSellerTrustLifecycle,
  });

  factory MessageDto.fromJson(Map<String, dynamic> json) {
    final senderName =
        (json['sender_name'] as String?) ??
        (json['sender'] is Map<String, dynamic>
            ? (json['sender']['username'] as String?)
            : null) ??
        '';
    final senderUsername =
        (json['sender_username'] as String?) ??
        (json['sender'] is Map<String, dynamic>
            ? (json['sender']['username'] as String?)
            : null) ??
        '';
    final type =
        (json['type'] as String?) ??
        (json['message_type'] as String?) ??
        'text';
    final content =
        (json['content'] as String?) ?? (json['body'] as String?) ?? '';
    final isHidden = _readIsHidden(json);
    final rawAttachment =
        json['attachment'] ?? json['attachment_json'] as Map<String, dynamic>?;

    return MessageDto(
      id: json['id'] as String,
      chatRoomId:
          (json['chat_room_id'] as String?) ?? (json['room_id'] as String),
      senderId: json['sender_id'] as String,
      senderName: senderName,
      senderUsername: senderUsername,
      senderAvatar: json['sender_avatar'] as String?,
      content: content,
      isHidden: isHidden,
      type: type,
      mediaUrls: (json['media_urls'] as List<dynamic>?)?.cast<String>(),
      attachment: rawAttachment != null
          ? parseAttachmentDto(rawAttachment as Map<String, dynamic>)
          : null,
      status: (json['status'] as String?) ?? 'sent',
      isRead: (json['is_read'] as bool?) ?? false,
      isEdited: (json['is_edited'] as bool?) ?? false,
      editedAt: json['edited_at'] != null
          ? DateTime.parse(json['edited_at'] as String)
          : null,
      replyToId: json['reply_to_id'] as String?,
      replyPreview: json['reply_preview'] != null
          ? ReplyPreviewDto.fromJson(
              json['reply_preview'] as Map<String, dynamic>,
            )
          : null,
      mentionedUserIds: (json['mentioned_user_ids'] as List<dynamic>?)
          ?.cast<String>(),
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : DateTime.parse(json['created_at'] as String),
      senderLifecycle: _readSenderLifecycle(json),
      attachmentSellerTrustLifecycle: _readAttachmentSellerTrustLifecycle(json),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'chat_room_id': chatRoomId,
    'sender_id': senderId,
    'sender_name': senderName,
    if (senderUsername.isNotEmpty) 'sender_username': senderUsername,
    if (senderAvatar != null) 'sender_avatar': senderAvatar,
    'content': content,
    'is_hidden': isHidden,
    'type': type,
    if (mediaUrls != null) 'media_urls': mediaUrls,
    if (attachment != null) 'attachment': attachment!.toJson(),
    'status': status,
    'is_read': isRead,
    'is_edited': isEdited,
    if (editedAt != null) 'edited_at': editedAt!.toIso8601String(),
    if (replyToId != null) 'reply_to_id': replyToId,
    if (replyPreview != null) 'reply_preview': replyPreview!.toJson(),
    if (mentionedUserIds != null) 'mentioned_user_ids': mentionedUserIds,
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    id,
    chatRoomId,
    senderId,
    senderUsername,
    status,
    createdAt,
  ];
}

/// E4.1 — Extract the embedded message-sender lifecycle string from the
/// chat-message wire envelope.
///
/// Preference order (mirrors the E2.1 feed / E3.1 comment patterns):
///   1. `sender.lifecycle`        — populated by a future backend slice via
///                                   publiccard.NewWithLifecycle on the
///                                   message-sender card. Returns nil today
///                                   (publiccard.New is still in use at
///                                   chat_handler.go hydrateMessageSenders).
///   2. `sender_lifecycle`        — flat top-level fallback for envelope
///                                   shapes that may sidecar the lifecycle
///                                   string alongside the legacy
///                                   `sender_id` / `sender_name` scalars.
///                                   Not currently emitted; included for
///                                   forward-compat parity.
///
/// Returns null when both paths are absent / empty / non-string. The mapper
/// layer converts null into `ContentLifecycle.active` via the canonical
/// `ContentLifecycleParse.fromWire` helper. Never throws.
String? _readSenderLifecycle(Map<String, dynamic> json) {
  final sender = json['sender'];
  if (sender is Map<String, dynamic>) {
    final lc = sender['lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  final flat = json['sender_lifecycle'];
  if (flat is String && flat.isNotEmpty) return flat;
  return null;
}

/// B1/B2 — Extract seller trust lifecycle from the message's attachment JSON.
///
/// The backend emits `seller_trust_lifecycle` at the TOP level of the
/// attachment_json map (not inside `data`). Checks both `attachment_json`
/// (backend wire key) and `attachment` (mobile DTO key) to handle both
/// API response formats and WebSocket payloads.
///
/// Returns null when absent — mapper layer defaults to ContentLifecycle.active.
String? _readAttachmentSellerTrustLifecycle(Map<String, dynamic> json) {
  // Primary: backend emits under response-side metadata
  final attachmentMetadata = json['attachment_metadata'];
  if (attachmentMetadata is Map<String, dynamic>) {
    final lc = attachmentMetadata['seller_trust_lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }

  // Primary: backend emits under "attachment_json" key
  final attachmentJson = json['attachment_json'];
  if (attachmentJson is Map<String, dynamic>) {
    final lc = attachmentJson['seller_trust_lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  // Fallback: mobile DTO format uses "attachment" key
  final attachment = json['attachment'];
  if (attachment is Map<String, dynamic>) {
    final lc = attachment['seller_trust_lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  return null;
}

bool _readIsHidden(Map<String, dynamic> json) {
  final raw = json['is_hidden'];
  if (raw is bool) return raw;
  if (raw is num) return raw != 0;
  if (raw is String) {
    final normalized = raw.trim().toLowerCase();
    return normalized == 'true' || normalized == '1';
  }
  return false;
}

/// Reply Preview DTO
class ReplyPreviewDto extends Equatable {
  final String content;
  final String senderName;
  final String type;

  const ReplyPreviewDto({
    required this.content,
    required this.senderName,
    required this.type,
  });

  factory ReplyPreviewDto.fromJson(Map<String, dynamic> json) {
    return ReplyPreviewDto(
      content: json['content'] as String,
      senderName: json['sender_name'] as String,
      type: json['type'] as String,
    );
  }

  Map<String, dynamic> toJson() => {
    'content': content,
    'sender_name': senderName,
    'type': type,
  };

  @override
  List<Object?> get props => [content, senderName, type];
}

/// Send Message Request DTO
class SendMessageDto {
  final String body;
  final String messageType;
  final List<String>? mediaUrls;
  final AttachmentDto? attachment;
  final String? replyToId;
  final List<String>? mentionedUserIds;
  final String idempotencyKey;

  const SendMessageDto({
    required this.body,
    required this.messageType,
    required this.idempotencyKey,
    this.mediaUrls,
    this.attachment,
    this.replyToId,
    this.mentionedUserIds,
  });

  Map<String, dynamic> toJson() => {
    'body': body,
    'message_type': messageType,
    'idempotency_key': idempotencyKey,
    if (mediaUrls != null) 'media_urls': mediaUrls,
    if (attachment != null) 'attachment_json': attachment!.toJson(),
    if (replyToId != null) 'reply_to_id': replyToId,
    if (mentionedUserIds != null) 'mentioned_user_ids': mentionedUserIds,
  };
}

/// List Messages Response DTO
class ListMessagesDto {
  final List<MessageDto> messages;
  final bool hasMore;
  final String? nextCursor;

  const ListMessagesDto({
    required this.messages,
    required this.hasMore,
    this.nextCursor,
  });

  factory ListMessagesDto.fromJson(Map<String, dynamic> json) {
    final list = (json['messages'] ?? json['data']) as List<dynamic>;
    return ListMessagesDto(
      messages: list
          .map((e) => MessageDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      hasMore: (json['has_more'] as bool?) ?? false,
      nextCursor: json['next_cursor'] as String?,
    );
  }

  List<Object?> get props => [messages, hasMore, nextCursor];
}

/// Typing Event DTO
class TypingEventDto extends Equatable {
  final String chatRoomId;
  final String userId;
  final String userName;
  final bool isTyping;

  const TypingEventDto({
    required this.chatRoomId,
    required this.userId,
    required this.userName,
    required this.isTyping,
  });

  factory TypingEventDto.fromJson(Map<String, dynamic> json) {
    return TypingEventDto(
      chatRoomId: json['chat_room_id'] as String,
      userId: json['user_id'] as String,
      userName: json['user_name'] as String,
      isTyping: json['is_typing'] as bool,
    );
  }

  @override
  List<Object?> get props => [chatRoomId, userId, isTyping];
}

/// Mark Read Request DTO
class MarkReadDto {
  final DateTime timestamp;

  const MarkReadDto({required this.timestamp});

  Map<String, dynamic> toJson() => {
    'timestamp': timestamp.toUtc().toIso8601String(),
  };
}

/// Message Read Event DTO
class MessageReadEventDto extends Equatable {
  final String chatRoomId;
  final String messageId;
  final String userId;

  const MessageReadEventDto({
    required this.chatRoomId,
    required this.messageId,
    required this.userId,
  });

  factory MessageReadEventDto.fromJson(Map<String, dynamic> json) {
    return MessageReadEventDto(
      chatRoomId: json['chat_room_id'] as String,
      messageId: json['message_id'] as String,
      userId: json['user_id'] as String,
    );
  }

  @override
  List<Object?> get props => [chatRoomId, messageId, userId];
}

/// WebSocket Event DTO
class WebSocketEventDto {
  final WebSocketEventType type;
  final Map<String, dynamic> payload;

  const WebSocketEventDto({required this.type, required this.payload});

  factory WebSocketEventDto.fromJson(Map<String, dynamic> json) {
    return WebSocketEventDto(
      type: WebSocketEventType.fromString(json['type'] as String),
      payload: (json['payload'] as Map<String, dynamic>),
    );
  }
}

/// WebSocket Event Type
enum WebSocketEventType {
  messageNew,
  messageHidden,
  messageRestored,
  messageRead,
  roomCreated,
  roomUpdated,
  typingStarted,
  typingStopped,
  userOnline,
  userOffline,
  unknown;

  static WebSocketEventType fromString(String value) {
    switch (value) {
      case 'message.new':
      case 'chat.message.sent':
        return messageNew;
      case 'chat.message.hidden':
        return messageHidden;
      case 'chat.message.restored':
        return messageRestored;
      case 'message.read':
        return messageRead;
      case 'chat.room.created':
        return roomCreated;
      case 'chat.room.updated':
        return roomUpdated;
      case 'typing.started':
        return typingStarted;
      case 'typing.stopped':
        return typingStopped;
      case 'user.online':
        return userOnline;
      case 'user.offline':
        return userOffline;
      default:
        return unknown;
    }
  }
}
