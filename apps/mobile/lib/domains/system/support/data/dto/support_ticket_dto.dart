library;

/// Data Transfer Objects (DTOs) for Support Module
/// Digunakan untuk serialization/deserialization dari API
/// Data layer - agnostic to data source
///
/// MIGRATED: Now uses Go Backend API (Firestore Timestamp handling removed)

import 'package:labuda/domains/system/support/domain/domain.dart';

// ============================================
// TICKET DTO
// ============================================

/// Support Ticket DTO - untuk API serialization
class SupportTicketDto {
  final String id;
  final String type; // 'support'
  final List<String> participantIds;
  final Map<String, dynamic> participantNames;
  final Map<String, dynamic> participantAvatars;
  final String supportCategory;
  final String supportPriority;
  final String supportStatus;
  final String? linkedOrderId;
  final String? assignedToAdmin;
  final String? assignedAdminName;
  final dynamic createdAt; // Timestamp or DateTime
  final dynamic updatedAt; // Timestamp or DateTime?
  final dynamic assignedAt; // Timestamp or DateTime?
  final dynamic resolvedAt; // Timestamp or DateTime?
  final String? resolvedBy;
  final dynamic firstResponseAt; // Timestamp or DateTime?
  final bool isActive;
  final String status; // 'active', 'blocked', 'deleted'

  // Last message preview
  final Map<String, dynamic>? lastMessage;

  const SupportTicketDto({
    required this.id,
    required this.type,
    required this.participantIds,
    required this.participantNames,
    required this.participantAvatars,
    required this.supportCategory,
    required this.supportPriority,
    required this.supportStatus,
    this.linkedOrderId,
    this.assignedToAdmin,
    this.assignedAdminName,
    required this.createdAt,
    this.updatedAt,
    this.assignedAt,
    this.resolvedAt,
    this.resolvedBy,
    this.firstResponseAt,
    required this.isActive,
    required this.status,
    this.lastMessage,
  });

  /// Create from Map (API response or Firestore)
  factory SupportTicketDto.fromMap(String id, Map<String, dynamic> data) {
    return SupportTicketDto(
      id: id,
      type: data['type'] as String? ?? 'support',
      participantIds: List<String>.from(data['participantIds'] ?? []),
      participantNames: Map<String, dynamic>.from(
        data['participantNames'] ?? {},
      ),
      participantAvatars: Map<String, dynamic>.from(
        data['participantAvatars'] ?? {},
      ),
      supportCategory: data['supportCategory'] as String? ?? 'general',
      supportPriority: data['supportPriority'] as String? ?? 'medium',
      supportStatus: data['supportStatus'] as String? ?? 'open',
      linkedOrderId: data['linkedOrderId'] as String?,
      assignedToAdmin: data['assignedToAdmin'] as String?,
      assignedAdminName: data['assignedAdminName'] as String?,
      createdAt: data['createdAt'],
      updatedAt: data['updatedAt'],
      assignedAt: data['assignedAt'],
      resolvedAt: data['resolvedAt'],
      resolvedBy: data['resolvedBy'] as String?,
      firstResponseAt: data['firstResponseAt'],
      isActive: data['isActive'] as bool? ?? true,
      status: data['status'] as String? ?? 'active',
      lastMessage: data['lastMessage'] as Map<String, dynamic>?,
    );
  }

  /// Convert to Map (for API or Firestore)
  Map<String, dynamic> toMap() {
    return {
      'id': id,
      'type': type,
      'participantIds': participantIds,
      'participantNames': participantNames,
      'participantAvatars': participantAvatars,
      'supportCategory': supportCategory,
      'supportPriority': supportPriority,
      'supportStatus': supportStatus,
      if (linkedOrderId != null) 'linkedOrderId': linkedOrderId,
      if (assignedToAdmin != null) 'assignedToAdmin': assignedToAdmin,
      if (assignedAdminName != null) 'assignedAdminName': assignedAdminName,
      'createdAt': createdAt,
      if (updatedAt != null) 'updatedAt': updatedAt,
      if (assignedAt != null) 'assignedAt': assignedAt,
      if (resolvedAt != null) 'resolvedAt': resolvedAt,
      if (resolvedBy != null) 'resolvedBy': resolvedBy,
      if (firstResponseAt != null) 'firstResponseAt': firstResponseAt,
      'isActive': isActive,
      'status': status,
      if (lastMessage != null) 'lastMessage': lastMessage,
    };
  }

  /// Convert to Entity
  SupportTicket toEntity() {
    // Extract user info (not support pool)
    final userId = participantIds.firstWhere(
      (id) => id != SupportIdentity.poolId,
      orElse: () => '',
    );
    final userName = (participantNames[userId] ?? 'Unknown User') as String;
    final userAvatar = participantAvatars[userId] as String?;

    // Extract last message info
    String? lastMessageContent;
    DateTime? lastMessageDateTime;
    if (lastMessage != null) {
      final message = lastMessage!;
      lastMessageContent = message['content'] as String?;
      final createdAt = message['createdAt'];
      if (createdAt is DateTime) {
        lastMessageDateTime = createdAt;
      } else if (createdAt is String) {
        lastMessageDateTime = DateTime.tryParse(createdAt);
      }
    }

    return SupportTicket(
      id: id,
      userId: userId,
      userName: userName,
      userAvatar: userAvatar,
      category: SupportCategory.values.firstWhere(
        (e) => e.name == supportCategory,
        orElse: () => SupportCategory.general,
      ),
      priority: SupportPriority.values.firstWhere(
        (e) => e.name == supportPriority,
        orElse: () => SupportPriority.medium,
      ),
      status: SupportStatus.values.firstWhere(
        (e) => e.name == supportStatus,
        orElse: () => SupportStatus.open,
      ),
      linkedOrderId: linkedOrderId,
      assignedToAdmin: assignedToAdmin,
      assignedAdminName: assignedAdminName,
      createdAt: _toDateTime(createdAt) ?? DateTime.now(),
      updatedAt: _toDateTime(updatedAt),
      assignedAt: _toDateTime(assignedAt),
      resolvedAt: _toDateTime(resolvedAt),
      resolvedBy: resolvedBy,
      firstResponseAt: _toDateTime(firstResponseAt),
      lastMessage: lastMessageContent,
      lastMessageAt: lastMessageDateTime,
      isActive: isActive,
    );
  }

  /// Helper: convert ISO String/DateTime to DateTime
  DateTime? _toDateTime(dynamic value) {
    if (value == null) return null;
    if (value is DateTime) return value;
    if (value is String) return DateTime.tryParse(value);
    return null;
  }

  /// Create from Entity
  static SupportTicketDto fromEntity(
    SupportTicket entity, {
    DateTime? updatedAt,
  }) {
    // Create participant lists with support pool
    final participantIds = [entity.userId, SupportIdentity.poolId];
    final participantNames = {
      entity.userId: entity.userName,
      SupportIdentity.poolId: SupportIdentity.displayName,
    };
    final participantAvatars = {
      entity.userId: entity.userAvatar,
      SupportIdentity.poolId: SupportIdentity.avatarUrl,
    };

    return SupportTicketDto(
      id: entity.id,
      type: 'support',
      participantIds: participantIds,
      participantNames: participantNames,
      participantAvatars: participantAvatars,
      supportCategory: entity.category.name,
      supportPriority: entity.priority.name,
      supportStatus: entity.status.name,
      linkedOrderId: entity.linkedOrderId,
      assignedToAdmin: entity.assignedToAdmin,
      assignedAdminName: entity.assignedAdminName,
      createdAt: entity.createdAt.toIso8601String(),
      updatedAt:
          updatedAt?.toIso8601String() ?? entity.updatedAt?.toIso8601String(),
      assignedAt: entity.assignedAt?.toIso8601String(),
      resolvedAt: entity.resolvedAt?.toIso8601String(),
      resolvedBy: entity.resolvedBy,
      firstResponseAt: entity.firstResponseAt?.toIso8601String(),
      isActive: entity.isActive,
      status: entity.isActive ? 'active' : 'deleted',
      lastMessage: entity.lastMessage != null
          ? {
              'content': entity.lastMessage,
              'createdAt': entity.lastMessageAt?.toIso8601String(),
            }
          : null,
    );
  }
}

// ============================================
// MESSAGE DTO
// ============================================

/// Message DTO untuk Firestore
class MessageDto {
  final String id;
  final String chatId;
  final String senderId;
  final String senderName;
  final String? senderAvatar;
  final String content;
  final String type;
  final List<String> mediaUrls;
  final dynamic createdAt; // Timestamp or DateTime
  final String status;
  final bool isEdited;
  final List<String> mentionedUserIds;

  const MessageDto({
    required this.id,
    required this.chatId,
    required this.senderId,
    required this.senderName,
    this.senderAvatar,
    required this.content,
    required this.type,
    required this.mediaUrls,
    required this.createdAt,
    required this.status,
    this.isEdited = false,
    this.mentionedUserIds = const [],
  });

  /// Create from Map
  factory MessageDto.fromMap(Map<String, dynamic> data) {
    return MessageDto(
      id: data['id'] as String? ?? '',
      chatId: data['chatId'] as String? ?? '',
      senderId: data['senderId'] as String? ?? '',
      senderName: data['senderName'] as String? ?? '',
      senderAvatar: data['senderAvatar'] as String?,
      content: data['content'] as String? ?? '',
      type: data['type'] as String? ?? 'text',
      mediaUrls: List<String>.from(data['mediaUrls'] ?? []),
      createdAt: data['createdAt'],
      status: data['status'] as String? ?? 'sent',
      isEdited: data['isEdited'] as bool? ?? false,
      mentionedUserIds: List<String>.from(data['mentionedUserIds'] ?? []),
    );
  }

  /// Convert to Map (for API or Firestore)
  Map<String, dynamic> toMap() {
    return {
      'id': id,
      'chatId': chatId,
      'senderId': senderId,
      'senderName': senderName,
      if (senderAvatar != null) 'senderAvatar': senderAvatar,
      'content': content,
      'type': type,
      'mediaUrls': mediaUrls,
      'createdAt': createdAt,
      'status': status,
      'isEdited': isEdited,
      'mentionedUserIds': mentionedUserIds,
    };
  }
}

// ============================================
// EVENT DTO
// ============================================

/// Support Event DTO - for API serialization
class SupportEventDto {
  final String id;
  final String ticketId;
  final String eventType;
  final String? actorId;
  final String? oldStatus;
  final String? newStatus;
  final String? notes;
  final Map<String, dynamic>? metadata;
  final DateTime createdAt;

  const SupportEventDto({
    required this.id,
    required this.ticketId,
    required this.eventType,
    this.actorId,
    this.oldStatus,
    this.newStatus,
    this.notes,
    this.metadata,
    required this.createdAt,
  });

  /// Create from Map (API response)
  factory SupportEventDto.fromMap(Map<String, dynamic> data) {
    return SupportEventDto(
      id: data['id'] as String? ?? '',
      ticketId: data['ticket_id'] as String? ?? '',
      eventType: data['event_type'] as String? ?? 'unknown',
      actorId: data['actor_id'] as String?,
      oldStatus: data['old_status'] as String?,
      newStatus: data['new_status'] as String?,
      notes: data['notes'] as String?,
      metadata: data['metadata'] as Map<String, dynamic>?,
      createdAt: data['created_at'] != null
          ? DateTime.parse(data['created_at'] as String)
          : DateTime.now(),
    );
  }

  /// Convert to Entity
  SupportEvent toEntity({String? actorName}) {
    return SupportEvent(
      id: id,
      ticketId: ticketId,
      eventType: _parseEventType(eventType),
      actorId: actorId,
      actorName: actorName,
      oldStatus: oldStatus,
      newStatus: newStatus,
      notes: notes,
      metadata: metadata,
      createdAt: createdAt,
    );
  }

  SupportEventType _parseEventType(String value) {
    switch (value) {
      case 'ticket_created':
        return SupportEventType.ticketCreated;
      case 'ticket_claimed':
        return SupportEventType.ticketClaimed;
      case 'ticket_waiting_user':
        return SupportEventType.ticketWaitingUser;
      case 'status_changed':
        return SupportEventType.statusChanged;
      case 'priority_changed':
        return SupportEventType.priorityChanged;
      case 'category_changed':
        return SupportEventType.categoryChanged;
      case 'ticket_resolved':
        return SupportEventType.ticketResolved;
      case 'ticket_closed':
        return SupportEventType.ticketClosed;
      case 'ticket_reopened':
        return SupportEventType.ticketReopened;
      case 'admin_assigned':
        return SupportEventType.adminAssigned;
      case 'admin_unassigned':
        return SupportEventType.adminUnassigned;
      default:
        return SupportEventType.unknown;
    }
  }
}
