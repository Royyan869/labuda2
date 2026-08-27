import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/attachment_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/attachment/attachment.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Chat Mapper - Entity ↔ DTO conversion
///
/// **MESSAGE TYPE NORMALIZATION:**
/// - When SENDING to API: Media types (image, video, audio, file) → "text"
/// - When PARSING from API: "text" → MessageType.text (media determined by mediaUrls presence)
/// - Backend only recognizes: "text", "system", "negotiation_proposal"
///
/// **This is intentional:** Backend treats media as attachment payload, not message type.
/// Frontend UI uses separate enum values for rendering convenience only.
class ChatMapper {
  /// Convert DTO to Domain Entity
  static Chat toDomain(ChatDto dto) {
    return Chat(
      id: dto.id,
      type: _stringToChatType(dto.type),
      participantIds: dto.participantIds,
      participantNames: dto.participantNames,
      participantAvatars: dto.participantAvatars,
      context: _mapToShareReference(dto.context),
      contextSetBy: dto.contextSetBy,
      lastMessage: dto.lastMessage != null
          ? _lastMessageToDomain(dto.lastMessage!)
          : null,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      unreadCounts: dto.unreadCounts,
      isActive: dto.isActive,
      status: _stringToChatStatus(dto.status),
      deletedBy: dto.deletedBy,
      supportCategory: dto.supportCategory != null
          ? _stringToSupportCategory(dto.supportCategory!)
          : null,
      supportPriority: dto.supportPriority != null
          ? _stringToSupportPriority(dto.supportPriority!)
          : null,
      supportStatus: dto.supportStatus != null
          ? _stringToSupportStatus(dto.supportStatus!)
          : null,
      assignedToAdmin: dto.assignedToAdmin,
      assignedAdminName: dto.assignedAdminName,
      assignedAt: dto.assignedAt,
      resolvedAt: dto.resolvedAt,
      resolvedBy: dto.resolvedBy,
      firstResponseAt: dto.firstResponseAt,
      linkedOrderId: dto.linkedOrderId,
      // E4.1/E4.2/E4.3 — Per-participant lifecycle. Wire strings converted
      // via ContentLifecycleParse.fromWire (null/unknown/missing → active).
      // Absent participants default to active via getParticipantLifecycle.
      // ChatCard reads this via getOtherParticipantLifecycle (E4.3).
      participantLifecycles: _participantLifecyclesToDomain(
        dto.participantLifecycles,
      ),
    );
  }

  /// Convert Domain Entity to DTO
  static ChatDto toDto(Chat entity) {
    return ChatDto(
      id: entity.id,
      type: _chatTypeToString(entity.type),
      participantIds: entity.participantIds,
      participantNames: entity.participantNames,
      participantAvatars: entity.participantAvatars,
      context: _shareReferenceToMap(entity.context),
      contextSetBy: entity.contextSetBy,
      lastMessage: entity.lastMessage != null
          ? _messageToLastMessageDto(entity.lastMessage!)
          : null,
      createdAt: entity.createdAt,
      updatedAt: entity.updatedAt,
      unreadCounts: entity.unreadCounts,
      isActive: entity.isActive,
      status: _chatStatusToString(entity.status),
      deletedBy: entity.deletedBy,
      supportCategory: entity.supportCategory != null
          ? _supportCategoryToString(entity.supportCategory!)
          : null,
      supportPriority: entity.supportPriority != null
          ? _supportPriorityToString(entity.supportPriority!)
          : null,
      supportStatus: entity.supportStatus != null
          ? _supportStatusToString(entity.supportStatus!)
          : null,
      assignedToAdmin: entity.assignedToAdmin,
      assignedAdminName: entity.assignedAdminName,
      assignedAt: entity.assignedAt,
      resolvedAt: entity.resolvedAt,
      resolvedBy: entity.resolvedBy,
      firstResponseAt: entity.firstResponseAt,
      linkedOrderId: entity.linkedOrderId,
    );
  }

  /// Convert `List<DTO>` to `List<Entity>`
  static List<Chat> toDomainList(List<ChatDto> dtos) {
    return dtos.map((dto) => toDomain(dto)).toList();
  }

  /// Convert `List<Entity>` to `List<DTO>`
  static List<ChatDto> toDtoList(List<Chat> entities) {
    return entities.map((entity) => toDto(entity)).toList();
  }

  // ========================================
  // Message Mapping
  // ========================================

  /// Convert MessageDto to Domain Entity
  static Message messageToDomain(MessageDto dto) {
    if (dto.isHidden) {
      return Message(
        id: dto.id,
        chatId: dto.chatRoomId,
        senderId: dto.senderId,
        senderName: dto.senderName,
        senderUsername: dto.senderUsername,
        senderAvatar: dto.senderAvatar,
        content: '',
        isHidden: true,
        type: _stringToMessageType(dto.type),
        mediaUrls: const [],
        createdAt: dto.createdAt,
        status: _stringToMessageStatus(dto.status),
        isEdited: dto.isEdited,
        replyToId: null,
        mentionedUserIds: const [],
        deletedBy: const [],
        senderLifecycle: ContentLifecycleParse.fromWire(dto.senderLifecycle),
        attachmentSellerTrustLifecycle:
            dto.attachmentSellerTrustLifecycle != null
            ? ContentLifecycleParse.fromWire(dto.attachmentSellerTrustLifecycle)
            : ContentLifecycle.active,
      );
    }

    final attachments = _dtoAttachmentToDomain(dto.attachment);

    return Message(
      id: dto.id,
      chatId: dto.chatRoomId,
      senderId: dto.senderId,
      senderName: dto.senderName,
      senderUsername: dto.senderUsername,
      senderAvatar: dto.senderAvatar,
      content: dto.content,
      isHidden: dto.isHidden,
      type: _stringToMessageType(dto.type),
      mediaUrls: dto.mediaUrls ?? const [],
      objectReference: attachments['objectReference'] as ShareReference?,
      negotiationOffer:
          attachments['negotiationOffer'] as NegotiationOfferAttachment?,
      negotiationProposal:
          attachments['negotiationProposal'] as NegotiationProposalAttachment?,
      negotiationResult:
          attachments['negotiationResult'] as NegotiationResultAttachment?,
      shippingQuote: attachments['shippingQuote'] as ShippingQuoteAttachment?,
      location: attachments['location'] as LocationAttachment?,
      createdAt: dto.createdAt,
      status: _stringToMessageStatus(dto.status),
      isEdited: dto.isEdited,
      replyToId: dto.replyToId,
      mentionedUserIds: dto.mentionedUserIds ?? const [],
      deletedBy: const [],
      // E4.1 — Tolerant parse of the message-sender lifecycle. Null /
      // unknown / missing all fall back to ContentLifecycle.active, so
      // legacy backends (where senderLifecycle is always null) continue
      // to render exactly as today. Render-passive: no widget consumes
      // this field yet.
      senderLifecycle: ContentLifecycleParse.fromWire(dto.senderLifecycle),
      // B1/B2 — Seller trust lifecycle for the attachment's referenced item.
      // Null → active (legacy-safe; old backends don't emit this field).
      attachmentSellerTrustLifecycle: dto.attachmentSellerTrustLifecycle != null
          ? ContentLifecycleParse.fromWire(dto.attachmentSellerTrustLifecycle)
          : ContentLifecycle.active,
    );
  }

  /// Convert Domain Entity to MessageDto
  static MessageDto messageToDto(Message entity) {
    return MessageDto(
      id: entity.id,
      chatRoomId: entity.chatId,
      senderId: entity.senderId,
      senderName: entity.senderName,
      senderUsername: entity.senderUsername,
      senderAvatar: entity.senderAvatar,
      content: entity.content,
      isHidden: entity.isHidden,
      type: _messageTypeToString(entity.type),
      mediaUrls: entity.mediaUrls,
      attachment: domainAttachmentToDto(entity),
      status: _messageStatusToString(entity.status),
      isRead: entity.status == MessageStatus.read,
      isEdited: entity.isEdited,
      replyToId: entity.replyToId,
      mentionedUserIds: entity.mentionedUserIds,
      createdAt: entity.createdAt,
      updatedAt: entity.createdAt,
    );
  }

  /// Convert `List<MessageDto>` to `List<Message>`
  static List<Message> messageListToDomain(List<MessageDto> dtos) {
    return dtos.map((dto) => messageToDomain(dto)).toList();
  }

  // ========================================
  // Helper Methods
  // ========================================

  /// E4.1 — Convert a wire-shaped participant-lifecycle map (keyed by user
  /// id → raw wire string) into a domain map (keyed by user id →
  /// ContentLifecycle).
  ///
  /// Each entry is converted via the canonical
  /// `ContentLifecycleParse.fromWire` helper, which tolerates null /
  /// unknown / empty values by collapsing them to `ContentLifecycle.active`.
  /// Empty input → empty map; participants not present here resolve to
  /// `active` via `Chat.getParticipantLifecycle`.
  static Map<String, ContentLifecycle> _participantLifecyclesToDomain(
    Map<String, String> wire,
  ) {
    if (wire.isEmpty) return const {};
    final out = <String, ContentLifecycle>{};
    wire.forEach((userId, raw) {
      out[userId] = ContentLifecycleParse.fromWire(raw);
    });
    return out;
  }

  static Message _lastMessageToDomain(LastMessageDto dto) {
    return Message(
      id: dto.id,
      chatId: '', // Not available in last message
      senderId: dto.senderId,
      senderName: dto.senderName,
      content: dto.content,
      isHidden: dto.isHidden,
      type: _stringToMessageType(dto.type),
      createdAt: dto.createdAt,
      status: _stringToMessageStatus(dto.status),
    );
  }

  static LastMessageDto _messageToLastMessageDto(Message message) {
    return LastMessageDto(
      id: message.id,
      senderId: message.senderId,
      senderName: message.senderName,
      content: message.content,
      type: _messageTypeToString(message.type),
      createdAt: message.createdAt,
      status: _messageStatusToString(message.status),
      isHidden: message.isHidden,
    );
  }

  // Chat Type conversions
  //
  // **BACKEND ALIGNMENT V1:**
  // Frontend ChatType aligns with backend canonical room types:
  // - Backend: `direct`, `negotiation`, `support`
  // - Frontend: `private` (maps to both direct/negotiation), `support`
  //
  // Legacy `group` and `channel` types were removed as they are not supported by backend.
  static ChatType _stringToChatType(String type) {
    switch (type) {
      case 'private':
      case 'direct': // Backend room type maps to frontend private
      case 'negotiation': // Backend room type maps to frontend private
        return ChatType.private;
      case 'support':
        return ChatType.support;
      default:
        return ChatType.private; // Default fallback
    }
  }

  static String _chatTypeToString(ChatType type) {
    // Frontend types map to backend for API calls
    // Backend uses: `direct`, `negotiation`, `support`
    switch (type) {
      case ChatType.private:
        return 'direct'; // Private chats map to backend 'direct' room type
      case ChatType.support:
        return 'support';
    }
  }

  // Chat Status conversions
  static ChatStatus _stringToChatStatus(String status) {
    switch (status) {
      case 'active':
        return ChatStatus.active;
      case 'blocked':
        return ChatStatus.blocked;
      case 'deleted':
        return ChatStatus.deleted;
      default:
        return ChatStatus.active;
    }
  }

  static String _chatStatusToString(ChatStatus status) {
    switch (status) {
      case ChatStatus.active:
        return 'active';
      case ChatStatus.blocked:
        return 'blocked';
      case ChatStatus.deleted:
        return 'deleted';
    }
  }

  // Message Type conversions
  static MessageType _stringToMessageType(String type) {
    switch (type) {
      case 'text':
        return MessageType.text;
      case 'image':
        return MessageType.image;
      case 'video':
        return MessageType.video;
      case 'audio':
        return MessageType.audio;
      case 'file':
        return MessageType.file;
      case 'system':
        return MessageType.system;
      case 'negotiation_proposal':
        return MessageType.negotiationProposal;
      default:
        return MessageType
            .text; // Fallback for unknown types (including legacy offer_reference)
    }
  }

  static String _messageTypeToString(MessageType type) {
    // Normalize message types for API compatibility.
    // Media types (image, video, audio, file) are sent as "text" with mediaUrls attachment.
    // This aligns with backend's message type truth where media is carried as attachment payload.
    switch (type) {
      case MessageType.text:
      case MessageType.image:
      case MessageType.video:
      case MessageType.audio:
      case MessageType.file:
        return 'text';
      case MessageType.system:
        return 'system';
      case MessageType.negotiationProposal:
        return 'negotiation_proposal';
      case MessageType.shippingQuote:
        return 'shipping_quote';
    }
  }

  // Message Status conversions
  static MessageStatus _stringToMessageStatus(String status) {
    switch (status) {
      case 'sending':
        return MessageStatus.sending;
      case 'sent':
        return MessageStatus.sent;
      case 'delivered':
        return MessageStatus.delivered;
      case 'read':
        return MessageStatus.read;
      case 'failed':
        return MessageStatus.failed;
      default:
        return MessageStatus.sent;
    }
  }

  static String _messageStatusToString(MessageStatus status) {
    switch (status) {
      case MessageStatus.sending:
        return 'sending';
      case MessageStatus.sent:
        return 'sent';
      case MessageStatus.delivered:
        return 'delivered';
      case MessageStatus.read:
        return 'read';
      case MessageStatus.failed:
        return 'failed';
    }
  }

  // Support Category conversions
  static SupportCategory _stringToSupportCategory(String category) {
    return SupportCategory.values.firstWhere(
      (e) => e.name == category,
      orElse: () => SupportCategory.general,
    );
  }

  static String _supportCategoryToString(SupportCategory category) {
    return category.name;
  }

  // Support Priority conversions
  static SupportPriority _stringToSupportPriority(String priority) {
    return SupportPriority.values.firstWhere(
      (e) => e.name == priority,
      orElse: () => SupportPriority.medium,
    );
  }

  static String _supportPriorityToString(SupportPriority priority) {
    return priority.name;
  }

  // Support Status conversions
  static SupportStatus _stringToSupportStatus(String status) {
    return SupportStatus.values.firstWhere(
      (e) => e.name == status,
      orElse: () => SupportStatus.open,
    );
  }

  static String _supportStatusToString(SupportStatus status) {
    return status.name;
  }

  // ========================================
  // Chat Context Mapping (ShareReference)
  // ========================================

  /// **SOCIAL FIX 1.1:** Chat context now uses ShareReference for all object references.
  /// This replaces the legacy _mapToAttachment() method that returned Attachment types.
  static ShareReference? _mapToShareReference(Map<String, dynamic>? map) {
    if (map == null) return null;

    if (!(map.containsKey('target_type') && map.containsKey('target_id'))) {
      return null;
    }

    try {
      return ShareReferenceAttachmentDto.fromJson({
        'type': 'reference',
        'data': map,
      }).toShareReference();
    } catch (_) {
      return null;
    }
  }

  /// Convert ShareReference to Map for API requests
  static Map<String, dynamic>? _shareReferenceToMap(ShareReference? reference) {
    if (reference == null) return null;

    final chatReference = reference.asChatReference();
    if (chatReference == null) {
      return null;
    }

    return {
      'target_type': chatReference.wireTargetType,
      'target_id': chatReference.targetId,
      'preview': {
        'title': chatReference.preview.title,
        if (chatReference.preview.imageUrl != null)
          'imageUrl': chatReference.preview.imageUrl,
        'isAvailable': chatReference.preview.isAvailable,
        'isSold': chatReference.preview.isSold,
        'isClosed': chatReference.preview.isClosed,
        'isDeleted': chatReference.preview.isDeleted,
      },
    };
  }

  // ========================================
  // Attachment Mapping (Message attachments only)
  // ========================================

  static Map<String, dynamic> _dtoAttachmentToDomain(AttachmentDto? dto) {
    if (dto == null) return {};

    // Handle ShareReference (object reference)
    if (dto.type == 'reference') {
      try {
        return {
          'objectReference': ShareReferenceAttachmentDto.fromJson({
            'type': dto.type,
            'data': dto.data,
          }).toShareReference(),
        };
      } catch (_) {
        return {};
      }
    }

    // Convert API format (snake_case) to domain format (camelCase)
    final map = <String, dynamic>{'type': dto.type};

    switch (dto.type) {
      case 'negotiation_offer':
        map['negotiationId'] = dto.data['negotiation_id'];
        map['forSaleId'] = dto.data['for_sale_id'];
        map['status'] = dto.data['status'];
        map['preview'] = dto.data['preview'];
        break;
      case 'negotiation_proposal':
        final proposalDto = dto as NegotiationProposalAttachmentDto;
        map['sessionId'] = proposalDto.sessionId;
        map['proposalSequence'] = proposalDto.proposalSequence;
        map['price'] = proposalDto.price;
        map['resourceType'] = proposalDto.resourceType;
        map['resourceId'] = proposalDto.resourceId;
        map['note'] = proposalDto.note;
        break;
      case 'negotiation_result':
        map['negotiationId'] = dto.data['negotiation_id'];
        map['forSaleId'] = dto.data['for_sale_id'];
        map['status'] = dto.data['status'];
        map['preview'] = dto.data['preview'];
        break;
      case 'shipping_quote':
        map['offerId'] = dto.data['offer_id'];
        map['linkedItemId'] = dto.data['linked_item_id'];
        map['linkedItemType'] = dto.data['linked_item_type'];
        map['auctionId'] = dto.data['auction_id'];
        map['linkedItemName'] = dto.data['linked_item_name'];
        map['linkedItemImage'] = dto.data['linked_item_image'];
        map['linkedItemPrice'] = dto.data['linked_item_price'];
        map['shippingType'] = dto.data['shipping_type'];
        map['shippingTypeName'] = dto.data['shipping_type_name'];
        map['shippingTypeEmoji'] = dto.data['shipping_type_emoji'];
        map['expeditionName'] = dto.data['expedition_name'];
        map['rate'] = dto.data['rate'];
        map['estimatedDays'] = dto.data['estimated_days'];
        map['notes'] = dto.data['notes'];
        map['validUntil'] = dto.data['valid_until'];
        map['status'] = dto.data['status'];
        map['sellerId'] = dto.data['seller_id'];
        break;
      case 'location':
        map['latitude'] = dto.data['latitude'];
        map['longitude'] = dto.data['longitude'];
        map['placeName'] = dto.data['placeName'];
        map['address'] = dto.data['address'];
        break;
      default:
        map.addAll(dto.data);
    }

    // Convert to specific attachment types
    final attachment = AttachmentMapper.fromMap(map);

    if (attachment is NegotiationOfferAttachment) {
      return {'negotiationOffer': attachment};
    } else if (attachment is NegotiationProposalAttachment) {
      return {'negotiationProposal': attachment};
    } else if (attachment is NegotiationResultAttachment) {
      return {'negotiationResult': attachment};
    } else if (attachment is ShippingQuoteAttachment) {
      return {'shippingQuote': attachment};
    } else if (attachment is LocationAttachment) {
      return {'location': attachment};
    }

    // Note: Object reference attachments (listing, auction, post, request) removed
    // These should now use ShareReference directly instead of Attachment wrappers

    return {};
  }

  static AttachmentDto? domainAttachmentToDto(Message message) {
    // Handle ObjectReference (ShareReference)
    if (message.objectReference != null) {
      final chatReference = message.objectReference!.asChatReference();
      if (chatReference == null) {
        return null;
      }
      return ShareReferenceAttachmentDto.fromShareReference(chatReference);
    }

    // Handle LocationAttachment
    if (message.location != null) {
      return LocationAttachmentDto(
        latitude: message.location!.latitude,
        longitude: message.location!.longitude,
        placeName: message.location!.placeName,
        address: message.location!.address,
      );
    }

    // Handle NegotiationOfferAttachment
    if (message.negotiationOffer != null) {
      final attachment = message.negotiationOffer!;
      return NegotiationOfferAttachmentDto(
        negotiationId: attachment.negotiationId,
        forSaleId: attachment.forSaleId,
        status: attachment.status,
        preview: SharePreviewDto(
          title: attachment.listingName,
          imageUrl: attachment.listingImage,
        ),
      );
    }

    // Handle NegotiationProposalAttachment
    if (message.negotiationProposal != null) {
      final proposal = message.negotiationProposal!;
      return NegotiationProposalAttachmentDto(
        sessionId: proposal.sessionId,
        proposalSequence: proposal.proposalSequence,
        price: proposal.price,
        resourceType: proposal.resourceType,
        resourceId: proposal.resourceId,
        note: proposal.note,
      );
    }

    // Handle NegotiationResultAttachment
    if (message.negotiationResult != null) {
      final attachment = message.negotiationResult!;
      return NegotiationResultAttachmentDto(
        negotiationId: attachment.negotiationId,
        forSaleId: attachment.forSaleId,
        status: attachment.status,
        preview: SharePreviewDto(
          title: attachment.listingName,
          imageUrl: attachment.listingImage,
        ),
      );
    }

    // Handle ShippingQuoteAttachment
    if (message.shippingQuote != null) {
      final attachment = message.shippingQuote!;
      return ShippingQuoteAttachmentDto(
        offerId: attachment.offerId,
        linkedItemId: attachment.linkedItemId,
        linkedItemType: attachment.linkedItemType,
        auctionId: attachment.auctionId,
        linkedItemName: attachment.linkedItemName,
        linkedItemImage: attachment.linkedItemImage,
        linkedItemPrice: attachment.linkedItemPrice,
        shippingType: attachment.shippingType,
        shippingTypeName: attachment.shippingTypeName,
        shippingTypeEmoji: attachment.shippingTypeEmoji,
        expeditionName: attachment.expeditionName,
        rate: attachment.rate,
        estimatedDays: attachment.estimatedDays,
        notes: attachment.notes,
        validUntil: attachment.validUntil.toIso8601String(),
        status: attachment.status,
        sellerId: attachment.sellerId,
      );
    }

    return null;
  }

  // ========================================
  // Public Helper Methods for Repository
  // ========================================

  /// Convert MessageType enum to String (public for repository use)
  static String messageTypeToString(MessageType type) {
    return _messageTypeToString(type);
  }
}
