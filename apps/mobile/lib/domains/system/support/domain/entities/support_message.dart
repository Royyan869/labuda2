/// Support Message Entity
///
/// Represents a single message in a support ticket conversation.
/// Messages are retrieved from the chat system via the ticket's chat_room_id.

library;

// ============================================
// // SENDER TYPE
// ============================================

/// Sender type for support messages
enum SupportSenderType {
  /// Message from the ticket owner (user)
  user('user'),

  /// Message from an admin/support agent
  admin('admin'),

  /// System-generated message (no sender)
  system('system');

  final String value;
  const SupportSenderType(this.value);

  static SupportSenderType fromString(String value) {
    return SupportSenderType.values.firstWhere(
      (type) => type.value == value,
      orElse: () => SupportSenderType.system,
    );
  }
}

// ============================================
// // MESSAGE TYPE
// ============================================

/// Message type (from chat system)
enum SupportMessageType {
  /// Regular text message
  text('text'),

  /// System-generated message
  system('system'),

  /// Negotiation-related message
  negotiationProposal('negotiation_proposal');

  final String value;
  const SupportMessageType(this.value);

  static SupportMessageType fromString(String value) {
    return SupportMessageType.values.firstWhere(
      (type) => type.value == value,
      orElse: () => SupportMessageType.text,
    );
  }
}

// ============================================
// // SUPPORT MESSAGE ENTITY
// ============================================

/// Support Message Entity
///
/// Represents a single message in a support ticket conversation.
class SupportMessage {
  /// Unique message ID
  final String id;

  /// Chat room ID (from ticket's chat_room_id)
  final String roomId;

  /// ID of the user who sent the message
  final String senderId;

  /// Type of sender (user, admin, or system)
  final SupportSenderType senderType;

  /// Message type (text, system, etc.)
  final SupportMessageType messageType;

  /// Message body (text content)
  final String? body;

  /// Attachment data (if any)
  final Map<String, dynamic>? attachmentJson;

  /// When the message was created
  final DateTime createdAt;

  const SupportMessage({
    required this.id,
    required this.roomId,
    required this.senderId,
    required this.senderType,
    required this.messageType,
    this.body,
    this.attachmentJson,
    required this.createdAt,
  });

  /// Whether this is a system message
  bool get isSystem => senderType == SupportSenderType.system;

  /// Whether this message has text content
  bool get hasTextContent => body != null && body!.isNotEmpty;

  /// Get display text for the message
  String get displayText {
    if (hasTextContent) return body!;
    if (attachmentJson != null) {
      return '[Attachment]';
    }
    return '';
  }

  /// Create a copy with modified fields
  SupportMessage copyWith({
    String? id,
    String? roomId,
    String? senderId,
    SupportSenderType? senderType,
    SupportMessageType? messageType,
    String? body,
    Map<String, dynamic>? attachmentJson,
    DateTime? createdAt,
  }) {
    return SupportMessage(
      id: id ?? this.id,
      roomId: roomId ?? this.roomId,
      senderId: senderId ?? this.senderId,
      senderType: senderType ?? this.senderType,
      messageType: messageType ?? this.messageType,
      body: body ?? this.body,
      attachmentJson: attachmentJson ?? this.attachmentJson,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is SupportMessage &&
          runtimeType == other.runtimeType &&
          id == other.id;

  @override
  int get hashCode => id.hashCode;

  @override
  String toString() {
    return 'SupportMessage(id: $id, senderType: $senderType, body: $body)';
  }
}
