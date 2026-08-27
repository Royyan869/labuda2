import 'package:equatable/equatable.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/chat/attachment/attachment.dart';

// Support ticket enums — canonical source is the Support/CS domain.
// Imported for use within this file; re-exported so Chat consumers that
// reference support fields on Chat entity don't need a second import.
import 'package:labuda/domains/system/support/domain/entities/support_ticket.dart'
    show SupportCategory, SupportPriority, SupportStatus;
export 'package:labuda/domains/system/support/domain/entities/support_ticket.dart'
    show SupportCategory, SupportPriority, SupportStatus;

/// Chat Type - tipe chat untuk mention logic
///
/// **BACKEND ALIGNMENT V1:**
/// Frontend ChatType aligns with backend canonical room types:
/// - Backend room types: `direct`, `negotiation`, `support`
/// - Frontend mapping: `direct`/`negotiation` → `private`, `support` → `support`
///
/// **ROOM MODEL CLARITY:**
/// - All chats use the SAME Chat entity structure - no special "negotiation room" types
/// - Room purpose is determined by `context` ShareReference (listing, auction, content, etc)
/// - Negotiations happen within regular private chats between buyer and seller
/// - Backend "negotiation" rooms map to frontend "private" (same chat, different backend context)
///
/// **Type meanings:**
/// - private: 1-on-1 chat (mentions DISABLED) - used for buyer-seller commerce, includes both backend "direct" and "negotiation" rooms
/// - support: Support chat user-to-admin (mentions DISABLED)
enum ChatType {
  /// 1-on-1 chat (mentions DISABLED)
  /// Maps to backend: `direct` OR `negotiation` room types
  /// Used for buyer-seller commerce chats
  private,

  /// Support chat user-to-admin (mentions DISABLED)
  /// Maps to backend: `support` room type
  support,
}

/// Chat Status
enum ChatStatus { active, blocked, deleted }

/// Message Type
///
/// **IMPORTANT: Backend vs Frontend Alignment**
///
/// **Backend recognizes ONLY:**
/// - "text" - All regular messages including media (media carried in mediaUrls array)
/// - "system" - System-generated messages (order created, payment confirmed, etc)
/// - "negotiation_proposal" - Commerce: negotiation proposal message
///
/// **Frontend UI convenience types:**
/// - text, image, video, audio, file - For UI rendering differentiation
/// - All media types NORMALIZE to "text" when sending to API
/// - Backend receives "text" with mediaUrls array for media content
///
/// **DOMAIN BOUNDARY:**
/// - Chat owns message delivery and rendering
/// - Commerce attachments (negotiation_proposal) are carried via Attachment payload
/// - Chat does NOT process commerce logic - only displays attachment state
///
/// **Migration Note:** Do NOT add new message types for business features.
/// Use ShareReference for object references (listing, auction, content)
/// Use Attachment system for workflow payloads (NegotiationOfferAttachment, etc.)
enum MessageType {
  text,
  image, // UI-only - normalizes to "text" for API
  video, // UI-only - normalizes to "text" for API
  audio, // UI-only - normalizes to "text" for API
  file, // UI-only - normalizes to "text" for API
  system, // System messages (order created, etc)
  negotiationProposal, // Commerce: negotiation proposal (aligns with backend "negotiation_proposal")
  shippingQuote, // Commerce: manual shipping quote (aligns with backend "shipping_quote")
}

/// Message Status
enum MessageStatus { sending, sent, delivered, read, failed }

/// MessageType extension for backend string serialization
///
/// Note: This extension is for API response parsing (from backend to frontend).
/// When sending TO API, use ChatMapper.messageTypeToString() which normalizes media types to "text".
///
/// Backend message types: text, negotiation_proposal, shipping_quote, system (see message_type.go)
extension MessageTypeExtension on MessageType {
  String get value {
    switch (this) {
      case MessageType.text:
        return 'text';
      case MessageType.image:
        return 'text'; // Media types normalize to text for API
      case MessageType.video:
        return 'text'; // Media types normalize to text for API
      case MessageType.audio:
        return 'text'; // Media types normalize to text for API
      case MessageType.file:
        return 'text'; // Media types normalize to text for API
      case MessageType.system:
        return 'system';
      case MessageType.negotiationProposal:
        return 'negotiation_proposal';
      case MessageType.shippingQuote:
        return 'shipping_quote';
    }
  }

  static MessageType fromString(String value) {
    switch (value) {
      case 'text':
        return MessageType.text;
      case 'system':
        return MessageType.system;
      case 'negotiation_proposal':
        return MessageType.negotiationProposal;
      case 'shipping_quote':
        return MessageType.shippingQuote;
      default:
        return MessageType.text; // Default fallback for unknown types
    }
  }
}

// SupportCategory, SupportPriority, SupportStatus — re-exported from
// package:labuda/domains/system/support/domain/entities/support_ticket.dart
// (see top-level export directive).  Do NOT redefine them here.

/// Chat Entity - untuk komunikasi buyer-seller
///
/// **DOMAIN BOUNDARY:**
/// - Chat is a COMMUNICATION LAYER - not a transaction system
/// - Chat carries context via ShareReference (for object references) or Attachment (for workflow payloads)
/// - Chat does NOT own commerce state - it only displays it
/// - Commerce actions (nego, quote, checkout) trigger domain operations
/// - Commerce results flow back through attachments in messages
///
/// **REFERENCE TRUTH ALIGNMENT V1:**
/// Chat memiliki DUA LEVEL reference yang TERPISAH secara tegas:
///
/// 1. ROOM CONTEXT REFERENCE (Chat.context):
///    - Room-level relation: "chat ini tentang apa"
///    - Ditetapkan saat room dibuat atau di-update kemudian
///    - Bersifat semi-permanen untuk lifecycle room
///    - Dipakai untuk: menampilkan header chat, navigation back to source
///    - TIDAK terpengaruh oleh pesan-pesan individual
///
/// 2. MESSAGE EMBEDDED REFERENCE (Message.attachment):
///    - Message-level payload: "pesan ini mengirim apa"
///    - Setiap pesan bisa membawa attachment berbeda
///    - Bersifat ephemeral per pesan
///    - Dipakai untuk: menampilkan object yang dikirim di pesan tertentu
///    - BISA berbeda dari room context (user bisa kirim object lain dalam chat)
///
/// **KEY PRINCIPLE:**
/// Room context TIDAK boleh di-overwrite oleh message attachment.
/// Room context = persistent topic untuk seluruh conversation.
/// Message attachment = specific object dalam pesan tersebut.
///
/// **ROOM CONTEXT:**
/// - `context` attachment defines what the chat is about (listing, auction, etc)
/// - `linkedOrderId` connects chat to an order after checkout
/// - Chat type (private/group/channel/support) defines mention behavior
class Chat extends Equatable {
  final String id;
  final ChatType type;
  final List<String> participantIds;
  final Map<String, String> participantNames;
  final Map<String, String?> participantAvatars;

  /// E4.1 — Canonical governance lifecycle per participant, parallel to
  /// [participantNames] and [participantAvatars].
  ///
  /// Carries the coarsened public lifecycle state for each participant
  /// identity (PublicCard.UserCard.Lifecycle on the chat-participant seam)
  /// from the chat-room wire envelope. Users not present in the map fail
  /// closed to [ContentLifecycle.unavailable] via [getParticipantLifecycle]
  /// — a missing entry is treated as missing wire, not as active.
  ///
  /// Independent of [isActive] / [status] (which describe the ROOM state,
  /// not a participant's identity state) and of [deletedBy] (which is the
  /// per-user soft-delete bit, not author governance).
  ///
  /// E4.2 backend activation (2026-05-13): `other_user.lifecycle` is now
  /// emitted on the room list via buildChatParticipantCardsWithLifecycle.
  /// E4.3 render gate: ChatCard reads this map via getOtherParticipantLifecycle.
  final Map<String, ContentLifecycle> participantLifecycles;

  /// ROOM CONTEXT REFERENCE (room-level)
  ///
  /// **SOCIAL FIX 1.1:** Uses ShareReference for all object references.
  /// Chat context is ALWAYS a ShareReference (listing, auction, content, profile).
  /// Workflow payloads (negotiation, shipping) are carried in Message.attachment only.
  ///
  /// Defines what this chat is about - set at room creation or updated later
  /// This is DIFFERENT from Message.attachment (message-level reference)
  /// Room context is persistent for the conversation lifecycle
  final ShareReference? context;

  /// User who set the room context (for audit trail)
  final String? contextSetBy;
  final Message? lastMessage;
  final DateTime createdAt;
  final DateTime? updatedAt;
  final Map<String, int> unreadCounts;
  final bool isActive;
  final ChatStatus status;
  final List<String> deletedBy;

  // Support-specific fields
  final SupportCategory? supportCategory;
  final SupportPriority? supportPriority;
  final SupportStatus? supportStatus;
  final String? assignedToAdmin;
  final String? assignedAdminName;
  final DateTime? assignedAt;
  final DateTime? resolvedAt;
  final String? resolvedBy;
  final DateTime? firstResponseAt;
  final String? linkedOrderId;

  const Chat({
    required this.id,
    this.type = ChatType.private,
    required this.participantIds,
    required this.participantNames,
    required this.participantAvatars,
    this.context,
    this.contextSetBy,
    this.lastMessage,
    required this.createdAt,
    this.updatedAt,
    this.unreadCounts = const {},
    this.isActive = true,
    this.status = ChatStatus.active,
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

  bool get isMentionEnabled =>
      false; // No chat types support mentions (group/channel removed in V1)
  bool get isPrivateChat => type == ChatType.private;
  bool get isSupportChat => type == ChatType.support;
  bool get isSupportAssigned => isSupportChat && assignedToAdmin != null;
  bool get isSupportResolved =>
      isSupportChat && supportStatus == SupportStatus.resolved;

  String getOtherParticipantId(String currentUserId) {
    return participantIds.firstWhere((id) => id != currentUserId);
  }

  String getOtherParticipantName(String currentUserId) {
    final otherUserId = getOtherParticipantId(currentUserId);
    final name = participantNames[otherUserId];
    if (name != null && name.isNotEmpty) {
      return name;
    }
    return 'User ${otherUserId.substring(0, otherUserId.length.clamp(0, 8))}...';
  }

  /// E4.1 — Lookup helper for the per-participant governance lifecycle.
  ///
  /// FAIL CLOSED: returns [ContentLifecycle.unavailable] for any user id not
  /// present in the [participantLifecycles] map. A missing entry means the
  /// room-list wire did not carry lifecycle for that participant — treat it
  /// the same as missing wire elsewhere (3-state truth doctrine).
  ContentLifecycle getParticipantLifecycle(String userId) {
    return participantLifecycles[userId] ?? ContentLifecycle.unavailable;
  }

  /// E4.1 — Convenience over [getParticipantLifecycle] for the canonical
  /// "the other user in a direct chat" axis.
  ContentLifecycle getOtherParticipantLifecycle(String currentUserId) {
    return getParticipantLifecycle(getOtherParticipantId(currentUserId));
  }

  int getUnreadCount(String userId) {
    if (unreadCounts.containsKey(userId)) {
      return unreadCounts[userId] ?? 0;
    }
    if (unreadCounts.isNotEmpty) {
      return unreadCounts.values.fold(0, (sum, count) => sum + count);
    }
    return 0;
  }

  bool isDeletedBy(String userId) {
    return deletedBy.contains(userId);
  }

  @override
  List<Object?> get props => [
    id,
    type,
    participantIds,
    participantNames,
    participantAvatars,
    context,
    contextSetBy,
    lastMessage,
    createdAt,
    updatedAt,
    unreadCounts,
    isActive,
    status,
    deletedBy,
    supportCategory,
    supportPriority,
    supportStatus,
    assignedToAdmin,
    assignedAdminName,
    assignedAt,
    resolvedAt,
    resolvedBy,
    firstResponseAt,
    linkedOrderId,
    participantLifecycles,
  ];

  Chat copyWith({
    String? id,
    ChatType? type,
    List<String>? participantIds,
    Map<String, String>? participantNames,
    Map<String, String?>? participantAvatars,
    ShareReference? context,
    String? contextSetBy,
    Message? lastMessage,
    DateTime? createdAt,
    DateTime? updatedAt,
    Map<String, int>? unreadCounts,
    bool? isActive,
    ChatStatus? status,
    List<String>? deletedBy,
    SupportCategory? supportCategory,
    SupportPriority? supportPriority,
    SupportStatus? supportStatus,
    String? assignedToAdmin,
    String? assignedAdminName,
    DateTime? assignedAt,
    DateTime? resolvedAt,
    String? resolvedBy,
    DateTime? firstResponseAt,
    String? linkedOrderId,
    Map<String, ContentLifecycle>? participantLifecycles,
  }) {
    return Chat(
      id: id ?? this.id,
      type: type ?? this.type,
      participantIds: participantIds ?? this.participantIds,
      participantNames: participantNames ?? this.participantNames,
      participantAvatars: participantAvatars ?? this.participantAvatars,
      context: context ?? this.context,
      contextSetBy: contextSetBy ?? this.contextSetBy,
      lastMessage: lastMessage ?? this.lastMessage,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      unreadCounts: unreadCounts ?? this.unreadCounts,
      isActive: isActive ?? this.isActive,
      status: status ?? this.status,
      deletedBy: deletedBy ?? this.deletedBy,
      supportCategory: supportCategory ?? this.supportCategory,
      supportPriority: supportPriority ?? this.supportPriority,
      supportStatus: supportStatus ?? this.supportStatus,
      assignedToAdmin: assignedToAdmin ?? this.assignedToAdmin,
      assignedAdminName: assignedAdminName ?? this.assignedAdminName,
      assignedAt: assignedAt ?? this.assignedAt,
      resolvedAt: resolvedAt ?? this.resolvedAt,
      resolvedBy: resolvedBy ?? this.resolvedBy,
      firstResponseAt: firstResponseAt ?? this.firstResponseAt,
      linkedOrderId: linkedOrderId ?? this.linkedOrderId,
      participantLifecycles:
          participantLifecycles ?? this.participantLifecycles,
    );
  }
}

/// Message dalam chat
///
/// **REFERENCE TRUTH ALIGNMENT V1:**
/// Message memiliki EXPLICIT ATTACHMENT FIELDS (message-level):
/// - "pesan ini mengirim apa" - object yang dikirim dalam pesan ini
/// - Setiap pesan bisa membawa attachment berbeda
/// - Bersifat ephemeral per pesan
/// - BISA berbeda dari room context (Chat.context)
///
/// **RELATIONSHIP TO ROOM CONTEXT:**
/// - Chat.context = room-level, persistent topic untuk seluruh conversation
/// - Message attachment fields = message-level, specific object dalam pesan ini
/// - Message attachment TIDAK mengubah room context
///
/// **ATTACHMENT SIMPLIFICATION:**
/// Removed MessageAttachment wrapper abstraction - now using explicit fields for clarity.
/// Each attachment type has its own nullable field:
/// - objectReference: ShareReference (listing, auction, content, profile)
/// - negotiationOffer: NegotiationOfferAttachment (active negotiation state)
/// - negotiationProposal: NegotiationProposalAttachment (live backend proposal payload)
/// - negotiationResult: NegotiationResultAttachment (negotiation outcome)
/// - shippingQuote: ShippingQuoteAttachment (shipping offer data)
/// - location: LocationAttachment (location data)
///
/// Contoh:
/// - Room context = Listing A (chat dimulai dari Listing A)
/// - Message 1 = text "hai"
/// - Message 2 = objectReference Auction B (user kirim auction lain)
/// -> Room context TETAP Listing A, Message 2 objectReference Auction B
class Message extends Equatable {
  final String id;
  final String chatId;
  final String senderId;
  final String senderName;
  final String senderUsername;
  final String? senderAvatar;
  final String content;
  final bool isHidden;
  final MessageType type;
  final List<String> mediaUrls;

  /// MESSAGE EMBEDDED REFERENCE (message-level)
  /// Object yang dikirim dalam pesan ini
  /// This is DIFFERENT from Chat.context (room-level reference)
  /// Each message can have a different attachment
  ///
  /// **SIMPLIFIED:** Explicit attachment fields instead of wrapper abstraction

  /// Object Reference - ShareReference for listing, auction, content, profile
  final ShareReference? objectReference;

  /// Negotiation Offer - active negotiation state
  final NegotiationOfferAttachment? negotiationOffer;

  /// Negotiation Proposal - live backend proposal payload (initial / counter)
  final NegotiationProposalAttachment? negotiationProposal;

  /// Negotiation Result - negotiation outcome
  final NegotiationResultAttachment? negotiationResult;

  /// Shipping Quote - shipping offer data
  final ShippingQuoteAttachment? shippingQuote;

  /// Location - location data
  final LocationAttachment? location;

  final DateTime createdAt;
  final MessageStatus status;
  final bool isEdited;
  final String? replyToId;
  final List<String> mentionedUserIds;
  final List<String> deletedBy;

  /// E4.1 — Canonical governance lifecycle for the message SENDER identity
  /// (PublicCard.UserCard.Lifecycle on the message-sender seam at
  /// backend/internal/interaction/chat/delivery/http/chat_handler.go
  /// messageToResponse).
  ///
  /// Independent of [status] (which is the message's own delivery state) and
  /// of [deletedBy] (which is the per-user soft-delete bit, not author
  /// governance). Defaults to [ContentLifecycle.active] for null / unknown /
  /// missing wire values so legacy backend payloads keep rendering today's
  /// behavior.
  ///
  /// Mobile preparation only — backend currently emits `null` for this
  /// field. The mobile ingestion seam is wired ahead of any backend
  /// activation per E4.1 doctrine. No widget reads this field yet.
  final ContentLifecycle senderLifecycle;

  /// B1/B2 — Seller trust lifecycle for the item referenced in this message's
  /// attachment. Defaults to [ContentLifecycle.active] when absent (legacy-safe).
  /// Used by the CTA gate and badge display on embedded commerce cards.
  final ContentLifecycle attachmentSellerTrustLifecycle;

  const Message({
    required this.id,
    required this.chatId,
    required this.senderId,
    required this.senderName,
    this.senderUsername = '',
    this.senderAvatar,
    required this.content,
    this.isHidden = false,
    this.type = MessageType.text,
    this.mediaUrls = const [],
    this.objectReference,
    this.negotiationOffer,
    this.negotiationProposal,
    this.negotiationResult,
    this.shippingQuote,
    this.location,
    required this.createdAt,
    this.status = MessageStatus.sent,
    this.isEdited = false,
    this.replyToId,
    this.mentionedUserIds = const [],
    this.deletedBy = const [],
    this.senderLifecycle = ContentLifecycle.active,
    this.attachmentSellerTrustLifecycle = ContentLifecycle.active,
  });

  bool isFromUser(String userId) => senderId == userId;
  bool isDeletedBy(String userId) => deletedBy.contains(userId);

  /// Check if message has any attachment
  bool get hasAttachment =>
      objectReference != null ||
      negotiationOffer != null ||
      negotiationProposal != null ||
      negotiationResult != null ||
      shippingQuote != null ||
      location != null;

  @override
  List<Object?> get props => [
    id,
    chatId,
    senderId,
    senderName,
    senderUsername,
    senderAvatar,
    content,
    isHidden,
    type,
    mediaUrls,
    objectReference,
    negotiationOffer,
    negotiationProposal,
    negotiationResult,
    shippingQuote,
    location,
    createdAt,
    status,
    isEdited,
    replyToId,
    mentionedUserIds,
    deletedBy,
    senderLifecycle,
    attachmentSellerTrustLifecycle,
  ];

  Message copyWith({
    String? id,
    String? chatId,
    String? senderId,
    String? senderName,
    String? senderUsername,
    String? senderAvatar,
    String? content,
    bool? isHidden,
    MessageType? type,
    List<String>? mediaUrls,
    ShareReference? objectReference,
    NegotiationOfferAttachment? negotiationOffer,
    NegotiationProposalAttachment? negotiationProposal,
    NegotiationResultAttachment? negotiationResult,
    ShippingQuoteAttachment? shippingQuote,
    LocationAttachment? location,
    DateTime? createdAt,
    MessageStatus? status,
    bool? isEdited,
    String? replyToId,
    List<String>? mentionedUserIds,
    List<String>? deletedBy,
    ContentLifecycle? senderLifecycle,
    ContentLifecycle? attachmentSellerTrustLifecycle,
  }) {
    return Message(
      id: id ?? this.id,
      chatId: chatId ?? this.chatId,
      senderId: senderId ?? this.senderId,
      senderName: senderName ?? this.senderName,
      senderUsername: senderUsername ?? this.senderUsername,
      senderAvatar: senderAvatar ?? this.senderAvatar,
      content: content ?? this.content,
      isHidden: isHidden ?? this.isHidden,
      type: type ?? this.type,
      mediaUrls: mediaUrls ?? this.mediaUrls,
      objectReference: objectReference ?? this.objectReference,
      negotiationOffer: negotiationOffer ?? this.negotiationOffer,
      negotiationProposal: negotiationProposal ?? this.negotiationProposal,
      negotiationResult: negotiationResult ?? this.negotiationResult,
      shippingQuote: shippingQuote ?? this.shippingQuote,
      location: location ?? this.location,
      createdAt: createdAt ?? this.createdAt,
      status: status ?? this.status,
      isEdited: isEdited ?? this.isEdited,
      replyToId: replyToId ?? this.replyToId,
      mentionedUserIds: mentionedUserIds ?? this.mentionedUserIds,
      deletedBy: deletedBy ?? this.deletedBy,
      senderLifecycle: senderLifecycle ?? this.senderLifecycle,
      attachmentSellerTrustLifecycle:
          attachmentSellerTrustLifecycle ?? this.attachmentSellerTrustLifecycle,
    );
  }

  /// Returns a tombstoned copy for hidden/deleted moderation states.
  ///
  /// Hidden messages must not retain content or attachment affordances in
  /// memory, even if the client had previously cached the visible version.
  Message tombstone({bool isHidden = true}) {
    return Message(
      id: id,
      chatId: chatId,
      senderId: senderId,
      senderName: senderName,
      senderUsername: senderUsername,
      senderAvatar: senderAvatar,
      content: '',
      isHidden: isHidden,
      type: type,
      mediaUrls: const [],
      createdAt: createdAt,
      status: status,
      isEdited: isEdited,
      replyToId: null,
      mentionedUserIds: const [],
      deletedBy: deletedBy,
      senderLifecycle: senderLifecycle,
      attachmentSellerTrustLifecycle: attachmentSellerTrustLifecycle,
    );
  }
}

/// Typing indicator state
class TypingIndicator extends Equatable {
  final String chatId;
  final Map<String, DateTime> typingUsers; // userId -> last typing time

  const TypingIndicator({required this.chatId, this.typingUsers = const {}});

  List<String> get activeTypingUsers {
    final now = DateTime.now();
    return typingUsers.entries
        .where((e) => now.difference(e.value).inSeconds < 5)
        .map((e) => e.key)
        .toList();
  }

  @override
  List<Object?> get props => [chatId, typingUsers];

  TypingIndicator copyWith({
    String? chatId,
    Map<String, DateTime>? typingUsers,
  }) {
    return TypingIndicator(
      chatId: chatId ?? this.chatId,
      typingUsers: typingUsers ?? this.typingUsers,
    );
  }
}

/// User presence info
class UserPresence extends Equatable {
  final String userId;
  final bool isOnline;
  final DateTime? lastSeen;

  const UserPresence({
    required this.userId,
    this.isOnline = false,
    this.lastSeen,
  });

  @override
  List<Object?> get props => [userId, isOnline, lastSeen];

  UserPresence copyWith({String? userId, bool? isOnline, DateTime? lastSeen}) {
    return UserPresence(
      userId: userId ?? this.userId,
      isOnline: isOnline ?? this.isOnline,
      lastSeen: lastSeen ?? this.lastSeen,
    );
  }
}
