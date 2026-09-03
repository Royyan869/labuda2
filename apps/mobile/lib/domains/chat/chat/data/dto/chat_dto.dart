import 'package:equatable/equatable.dart';

/// Chat DTO from API
class ChatDto extends Equatable {
  final String id;
  final String type;
  final List<String> participantIds;
  final Map<String, String> participantNames;
  final Map<String, String?> participantAvatars;
  final LastMessageDto? lastMessage;
  final DateTime createdAt;
  final DateTime? updatedAt;
  final Map<String, int> unreadCounts;
  final bool isActive;
  final String status;
  final List<String> deletedBy;

  // Support fields
  final String? supportCategory;
  final String? supportPriority;
  final String? supportStatus;
  final String? assignedToAdmin;
  final String? assignedAdminName;
  final DateTime? assignedAt;
  final DateTime? resolvedAt;
  final String? resolvedBy;
  final DateTime? firstResponseAt;
  final String? linkedOrderId;

  /// E4.1 — Per-participant lifecycle wire strings, keyed by user id.
  ///
  /// Sources, in preference order (all tolerant):
  ///   1. `other_user.lifecycle`         — the canonical nested UserCard on
  ///                                        direct-room and by-order responses
  ///                                        (chat_handler.go roomToResponse).
  ///                                        When present, this DTO stores it
  ///                                        as `{ otherUserId: lifecycle }`.
  ///   2. `participant_lifecycles`       — a forward-compat map keyed by
  ///                                        user id, parallel to
  ///                                        `participant_names` /
  ///                                        `participant_avatars`. Not
  ///                                        currently emitted by backend;
  ///                                        included so future ListRooms
  ///                                        responses don't need a parser
  ///                                        change.
  ///
  /// Always tolerant: null / missing / unknown / non-string values are
  /// dropped. The mapper layer converts entries into `ContentLifecycle`
  /// via the canonical `ContentLifecycleParse.fromWire` helper; users not
  /// in the map default to `ContentLifecycle.active`.
  ///
  /// Backend emits lifecycle via buildChatParticipantCardsWithLifecycle
  /// (E4.2 — landed 2026-05-13; `other_user.lifecycle` on the room list
  /// wire since that activation).
  final Map<String, String> participantLifecycles;

  const ChatDto({
    required this.id,
    required this.type,
    required this.participantIds,
    required this.participantNames,
    required this.participantAvatars,
    this.lastMessage,
    required this.createdAt,
    this.updatedAt,
    this.unreadCounts = const {},
    this.isActive = true,
    this.status = 'active',
    this.deletedBy = const [],
    this.supportCategory,
    this.supportPriority,
    this.supportStatus,
    this.assignedToAdmin,
    this.assignedAdminName,
    this.assignedAt,
    this.resolvedAt,
    this.resolvedBy,
    this.firstResponseAt,
    this.linkedOrderId,
    this.participantLifecycles = const {},
  });

  factory ChatDto.fromJson(Map<String, dynamic> json) {
    // Handle backend response format which uses other_user_id
    // For direct room responses from the new API
    if (json.containsKey('other_user_id')) {
      final otherUserId = json['other_user_id'] as String?;
      final otherUserCard = json['other_user'];
      final participantNamesOut = <String, String>{};
      final participantAvatarsOut = <String, String?>{};
      if (otherUserId != null && otherUserCard is Map<String, dynamic>) {
        final rawUsername = otherUserCard['username'];
        if (rawUsername is String && rawUsername.isNotEmpty) {
          participantNamesOut[otherUserId] = rawUsername;
        }
        final rawAvatar = otherUserCard['avatar_url'];
        if (rawAvatar is String && rawAvatar.isNotEmpty) {
          participantAvatarsOut[otherUserId] = rawAvatar;
        }
      }
      return ChatDto(
        id: json['id'] as String,
        type: _roomTypeToChatType(json['room_type'] as String? ?? 'direct'),
        participantIds: otherUserId != null ? [otherUserId] : [],
        participantNames: participantNamesOut,
        participantAvatars: participantAvatarsOut,
        lastMessage: json['last_message'] != null
            ? LastMessageDto.fromJson(
                json['last_message'] as Map<String, dynamic>,
              )
            : null,
        createdAt: json['created_at'] != null
            ? DateTime.parse(json['created_at'] as String)
            : DateTime.now(),
        updatedAt: json['updated_at'] != null
            ? DateTime.parse(json['updated_at'] as String)
            : null,
        unreadCounts: _readUnreadCounts(json, otherUserId: otherUserId),
        isActive: true,
        status: 'active',
        deletedBy: const [],
        linkedOrderId: json['linked_order_id'] as String?,
        participantLifecycles: _readParticipantLifecycles(
          json,
          otherUserId: otherUserId,
        ),
      );
    }

    // Handle legacy/list response format with full participant info
    final participantNamesMap = <String, String>{};
    final participantNamesRaw =
        json['participant_names'] as Map<String, dynamic>? ?? {};
    participantNamesRaw.forEach((key, value) {
      participantNamesMap[key] = value?.toString() ?? 'Unknown';
    });

    final participantAvatarsMap = <String, String?>{};
    final participantAvatarsRaw =
        json['participant_avatars'] as Map<String, dynamic>? ?? {};
    participantAvatarsRaw.forEach((key, value) {
      participantAvatarsMap[key] = value?.toString();
    });

    return ChatDto(
      id: json['id'] as String,
      type: json['type'] as String? ?? 'private',
      participantIds: List<String>.from(json['participant_ids'] ?? []),
      participantNames: participantNamesMap,
      participantAvatars: participantAvatarsMap,
      lastMessage: json['last_message'] != null
          ? LastMessageDto.fromJson(
              json['last_message'] as Map<String, dynamic>,
            )
          : null,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : null,
      unreadCounts: Map<String, int>.from(json['unread_counts'] ?? {}),
      isActive: json['is_active'] as bool? ?? true,
      status: json['status'] as String? ?? 'active',
      deletedBy: List<String>.from(json['deleted_by'] ?? []),
      supportCategory: json['support_category'] as String?,
      supportPriority: json['support_priority'] as String?,
      supportStatus: json['support_status'] as String?,
      assignedToAdmin: json['assigned_to_admin'] as String?,
      assignedAdminName: json['assigned_admin_name'] as String?,
      assignedAt: json['assigned_at'] != null
          ? DateTime.parse(json['assigned_at'] as String)
          : null,
      resolvedAt: json['resolved_at'] != null
          ? DateTime.parse(json['resolved_at'] as String)
          : null,
      resolvedBy: json['resolved_by'] as String?,
      firstResponseAt: json['first_response_at'] != null
          ? DateTime.parse(json['first_response_at'] as String)
          : null,
      linkedOrderId: json['linked_order_id'] as String?,
      participantLifecycles: _readParticipantLifecycles(json),
    );
  }

  /// Convert backend room_type to frontend ChatType
  static String _roomTypeToChatType(String roomType) {
    switch (roomType) {
      case 'direct':
        return 'private';
      case 'negotiation':
        return 'private';
      case 'support':
        return 'support';
      default:
        return 'private';
    }
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'type': type,
    'participant_ids': participantIds,
    'participant_names': participantNames,
    'participant_avatars': participantAvatars,
    if (lastMessage != null) 'last_message': lastMessage!.toJson(),
    'created_at': createdAt.toIso8601String(),
    if (updatedAt != null) 'updated_at': updatedAt!.toIso8601String(),
    'unread_counts': unreadCounts,
    'is_active': isActive,
    'status': status,
    'deleted_by': deletedBy,
    if (supportCategory != null) 'support_category': supportCategory,
    if (supportPriority != null) 'support_priority': supportPriority,
    if (supportStatus != null) 'support_status': supportStatus,
    if (assignedToAdmin != null) 'assigned_to_admin': assignedToAdmin,
    if (assignedAdminName != null) 'assigned_admin_name': assignedAdminName,
    if (assignedAt != null) 'assigned_at': assignedAt!.toIso8601String(),
    if (resolvedAt != null) 'resolved_at': resolvedAt!.toIso8601String(),
    if (resolvedBy != null) 'resolved_by': resolvedBy,
    if (firstResponseAt != null)
      'first_response_at': firstResponseAt!.toIso8601String(),
    if (linkedOrderId != null) 'linked_order_id': linkedOrderId,
  };

  @override
  List<Object?> get props => [id, type, participantIds, createdAt];
}

Map<String, int> _readUnreadCounts(
  Map<String, dynamic> json, {
  String? otherUserId,
}) {
  final out = <String, int>{};
  final legacy = json['unread_counts'];
  if (legacy is Map<String, dynamic>) {
    legacy.forEach((key, value) {
      if (value is int) {
        out[key] = value;
      }
    });
  }

  final singleUnread = json['unread_count'];
  if (singleUnread is int) {
    // Backend /chat/rooms currently emits a single unread count for viewer.
    // Keep it in-map so domain layer can read it through sum fallback.
    final key = otherUserId ?? '__room_unread__';
    out[key] = singleUnread;
  }

  return out;
}

/// E4.1 — Extract the per-participant lifecycle map from the chat-room wire
/// envelope.
///
/// Sources, in preference order (all tolerant — never throws):
///   1. `participant_lifecycles`        — forward-compat map keyed by user
///                                         id, parallel to `participant_names`
///                                         and `participant_avatars`. Not
///                                         currently emitted by backend;
///                                         parsed here so future ListRooms
///                                         responses Just Work.
///   2. `other_user.lifecycle`          — single-participant fallback for
///                                         direct-room / by-order responses
///                                         (chat_handler.go roomToResponse).
///                                         When present and an `otherUserId`
///                                         is known, stored as
///                                         `{ otherUserId: lifecycle }`.
///
/// Empty / missing fields → empty map. Non-string entries are skipped. The
/// mapper layer is responsible for converting raw strings into
/// `ContentLifecycle` and supplying the default `active` value for any user
/// id not present in the returned map.
Map<String, String> _readParticipantLifecycles(
  Map<String, dynamic> json, {
  String? otherUserId,
}) {
  final out = <String, String>{};

  final rawMap = json['participant_lifecycles'];
  if (rawMap is Map) {
    rawMap.forEach((key, value) {
      if (key is String && value is String && value.isNotEmpty) {
        out[key] = value;
      }
    });
  }

  final other = json['other_user'];
  if (otherUserId != null && other is Map<String, dynamic>) {
    final lc = other['lifecycle'];
    if (lc is String && lc.isNotEmpty && !out.containsKey(otherUserId)) {
      out[otherUserId] = lc;
    }
  }

  return out;
}

/// Last Message DTO (embedded in chat)
class LastMessageDto extends Equatable {
  final String id;
  final String senderId;
  final String senderName;
  final String content;
  final String type;
  final DateTime createdAt;
  final String status;
  final bool isHidden;

  const LastMessageDto({
    required this.id,
    required this.senderId,
    required this.senderName,
    required this.content,
    required this.type,
    required this.createdAt,
    required this.status,
    this.isHidden = false,
  });

  factory LastMessageDto.fromJson(Map<String, dynamic> json) {
    final messageType = (json['message_type'] ?? json['type'] ?? 'text')
        .toString();
    final content = (json['body'] ?? json['content'] ?? '').toString();
    return LastMessageDto(
      id: json['id'] as String,
      senderId: (json['sender_id'] ?? '').toString(),
      senderName: (json['sender_name'] ?? '').toString(),
      content: content,
      type: messageType,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      status: json['status'] as String? ?? 'sent',
      isHidden: json['is_hidden'] == true,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'sender_id': senderId,
    'sender_name': senderName,
    'content': content,
    'type': type,
    'created_at': createdAt.toIso8601String(),
    'status': status,
    'is_hidden': isHidden,
  };

  @override
  List<Object?> get props => [id, senderId, content, createdAt];
}

/// Create Chat Request DTO
class CreateChatDto {
  final List<String> participantIds;

  const CreateChatDto({required this.participantIds});

  Map<String, dynamic> toJson() => {
    'participant_ids': participantIds,
  };
}

/// Chat List Response DTO
class ChatListDto {
  final List<ChatDto> chats;
  final bool hasMore;
  final String? nextCursor;

  const ChatListDto({
    required this.chats,
    required this.hasMore,
    this.nextCursor,
  });

  factory ChatListDto.fromJson(Map<String, dynamic> json) {
    return ChatListDto(
      chats:
          (json['chats'] as List<dynamic>?)
              ?.map((e) => ChatDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      hasMore: json['has_more'] as bool? ?? false,
      nextCursor: json['next_cursor'] as String?,
    );
  }
}
