import 'package:equatable/equatable.dart';

import 'message_dto.dart';

/// Chat room WebSocket event DTO.
///
/// This models the backend `chat.room.created` / `chat.room.updated` payload
/// as a typed room-summary snapshot. It is intentionally parse-only for now:
/// consumers can deserialize it without mutating chat-list state yet.
class ChatRoomEventDto extends Equatable {
  final WebSocketEventType eventType;
  final String roomId;
  final String roomType;
  final String otherUserId;
  final ChatRoomParticipantDto? otherUser;
  final String? linkedOrderId;
  final ChatRoomLastMessageDto? lastMessage;
  final int unreadCount;
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime lastMessageAt;

  const ChatRoomEventDto({
    required this.eventType,
    required this.roomId,
    required this.roomType,
    required this.otherUserId,
    this.otherUser,
    this.linkedOrderId,
    this.lastMessage,
    required this.unreadCount,
    required this.createdAt,
    required this.updatedAt,
    required this.lastMessageAt,
  });

  factory ChatRoomEventDto.fromWebSocketEvent(WebSocketEventDto event) {
    if (event.type != WebSocketEventType.roomCreated &&
        event.type != WebSocketEventType.roomUpdated) {
      throw FormatException('unsupported chat room event type: ${event.type}');
    }
    return ChatRoomEventDto.fromJson(event.payload, eventType: event.type);
  }

  factory ChatRoomEventDto.fromJson(
    Map<String, dynamic> json, {
    required WebSocketEventType eventType,
  }) {
    final roomId = json['room_id'] as String?;
    final roomType = json['room_type'] as String?;
    final otherUserId = json['other_user_id'] as String?;
    final unreadCount = _readUnreadCount(json);
    final createdAt = _readDateTime(json, 'created_at');
    final updatedAt = _readDateTime(json, 'updated_at');
    final lastMessageAt = _readDateTime(json, 'last_message_at');

    if (roomId == null || roomId.isEmpty) {
      throw const FormatException('room_id is required');
    }
    if (roomType == null || roomType.isEmpty) {
      throw const FormatException('room_type is required');
    }
    if (otherUserId == null || otherUserId.isEmpty) {
      throw const FormatException('other_user_id is required');
    }

    return ChatRoomEventDto(
      eventType: eventType,
      roomId: roomId,
      roomType: roomType,
      otherUserId: otherUserId,
      otherUser: _readOtherUser(json),
      linkedOrderId: json['linked_order_id'] as String?,
      lastMessage: json['last_message'] is Map<String, dynamic>
          ? ChatRoomLastMessageDto.fromJson(
              json['last_message'] as Map<String, dynamic>,
            )
          : null,
      unreadCount: unreadCount,
      createdAt: createdAt,
      updatedAt: updatedAt,
      lastMessageAt: lastMessageAt,
    );
  }

  Map<String, dynamic> toJson() => {
    'room_id': roomId,
    'room_type': roomType,
    'other_user_id': otherUserId,
    if (otherUser != null) 'other_user': otherUser!.toJson(),
    if (linkedOrderId != null) 'linked_order_id': linkedOrderId,
    if (lastMessage != null) 'last_message': lastMessage!.toJson(),
    'unread_count': unreadCount,
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
    'last_message_at': lastMessageAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    eventType,
    roomId,
    roomType,
    otherUserId,
    otherUser,
    linkedOrderId,
    lastMessage,
    unreadCount,
    createdAt,
    updatedAt,
    lastMessageAt,
  ];
}

class ChatRoomParticipantDto extends Equatable {
  final String id;
  final String? username;
  final String? displayName;
  final String? avatarUrl;
  final String? lifecycle;

  const ChatRoomParticipantDto({
    required this.id,
    this.username,
    this.displayName,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ChatRoomParticipantDto.fromJson(Map<String, dynamic> json) {
    final id = (json['id'] as String?) ?? (json['user_id'] as String?) ?? '';
    if (id.isEmpty) {
      throw const FormatException('other_user.id is required');
    }

    return ChatRoomParticipantDto(
      id: id,
      username: json['username'] as String?,
      displayName: json['display_name'] as String?,
      avatarUrl:
          (json['avatar_url'] as String?) ?? (json['avatarUrl'] as String?),
      lifecycle: json['lifecycle'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    if (username != null) 'username': username,
    if (displayName != null) 'display_name': displayName,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (lifecycle != null) 'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [id, username, displayName, avatarUrl, lifecycle];
}

class ChatRoomLastMessageDto extends Equatable {
  final String id;
  final String roomId;
  final String senderId;
  final String messageType;
  final String? body;
  final Map<String, dynamic>? attachmentJson;
  final bool isHidden;
  final DateTime createdAt;

  const ChatRoomLastMessageDto({
    required this.id,
    required this.roomId,
    required this.senderId,
    required this.messageType,
    this.body,
    this.attachmentJson,
    required this.isHidden,
    required this.createdAt,
  });

  factory ChatRoomLastMessageDto.fromJson(Map<String, dynamic> json) {
    final id = json['id'] as String?;
    final roomId = json['room_id'] as String?;
    final senderId = json['sender_id'] as String?;
    final messageType =
        (json['message_type'] as String?) ??
        (json['type'] as String?) ??
        'text';
    final isHidden = _readIsHidden(json);
    final createdAt = _readDateTime(json, 'created_at');

    if (id == null || id.isEmpty) {
      throw const FormatException('last_message.id is required');
    }
    if (roomId == null || roomId.isEmpty) {
      throw const FormatException('last_message.room_id is required');
    }
    if (senderId == null || senderId.isEmpty) {
      throw const FormatException('last_message.sender_id is required');
    }

    return ChatRoomLastMessageDto(
      id: id,
      roomId: roomId,
      senderId: senderId,
      messageType: messageType,
      body: isHidden
          ? null
          : (json['body'] as String?) ?? (json['content'] as String?),
      attachmentJson: json['attachment_json'] is Map<String, dynamic>
          ? json['attachment_json'] as Map<String, dynamic>
          : json['attachment'] is Map<String, dynamic>
          ? json['attachment'] as Map<String, dynamic>
          : null,
      isHidden: isHidden,
      createdAt: createdAt,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'room_id': roomId,
    'sender_id': senderId,
    'message_type': messageType,
    if (body != null) 'body': body,
    if (attachmentJson != null) 'attachment_json': attachmentJson,
    if (isHidden) 'is_hidden': true,
    'created_at': createdAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    id,
    roomId,
    senderId,
    messageType,
    body,
    attachmentJson,
    isHidden,
    createdAt,
  ];
}

DateTime _readDateTime(Map<String, dynamic> json, String key) {
  final raw = json[key];
  if (raw is String && raw.isNotEmpty) {
    return DateTime.parse(raw);
  }
  throw FormatException('$key is required');
}

int _readUnreadCount(Map<String, dynamic> json) {
  final raw = json['unread_count'];
  if (raw is int) return raw;
  if (raw is num) return raw.toInt();
  if (raw is String) return int.tryParse(raw) ?? 0;
  return 0;
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

ChatRoomParticipantDto? _readOtherUser(Map<String, dynamic> json) {
  final raw = json['other_user'];
  if (raw is Map<String, dynamic>) {
    return ChatRoomParticipantDto.fromJson(raw);
  }
  return null;
}
