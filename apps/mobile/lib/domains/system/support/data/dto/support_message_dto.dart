library;

/// Data Transfer Object for Support Message
/// Used for serialization/deserialization from API

import 'package:labuda/domains/system/support/domain/domain.dart';

// ============================================
// // MESSAGE DTO
// ============================================

/// Support Message DTO - for API serialization
class SupportMessageDto {
  final String id;
  final String roomId;
  final String senderId;
  final String senderType; // 'user', 'admin', 'system'
  final String messageType; // 'text', 'system', 'negotiation_proposal'
  final String? body;
  final Map<String, dynamic>? attachmentJson;
  final String createdAt; // ISO 8601 timestamp

  const SupportMessageDto({
    required this.id,
    required this.roomId,
    required this.senderId,
    required this.senderType,
    required this.messageType,
    this.body,
    this.attachmentJson,
    required this.createdAt,
  });

  /// Create DTO from API map
  factory SupportMessageDto.fromMap(Map<String, dynamic> map) {
    return SupportMessageDto(
      id: map['id'] as String? ?? '',
      roomId: map['room_id'] as String? ?? '',
      senderId: map['sender_id'] as String? ?? '',
      senderType: map['sender_type'] as String? ?? 'system',
      messageType: map['message_type'] as String? ?? 'text',
      body: map['body'] as String?,
      attachmentJson: map['attachment_json'] as Map<String, dynamic>?,
      createdAt:
          map['created_at'] as String? ?? DateTime.now().toIso8601String(),
    );
  }

  /// Convert to entity
  SupportMessage toEntity() {
    return SupportMessage(
      id: id,
      roomId: roomId,
      senderId: senderId,
      senderType: SupportSenderType.fromString(senderType),
      messageType: SupportMessageType.fromString(messageType),
      body: body,
      attachmentJson: attachmentJson,
      createdAt: DateTime.parse(createdAt),
    );
  }
}
