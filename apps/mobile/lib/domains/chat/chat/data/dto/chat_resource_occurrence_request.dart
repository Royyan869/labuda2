import 'package:equatable/equatable.dart';

enum ChatResourceOccurrenceOperation {
  shareToChat,
  directCommerceInsertChat;

  String get wireValue {
    switch (this) {
      case ChatResourceOccurrenceOperation.shareToChat:
        return 'share_to_chat';
      case ChatResourceOccurrenceOperation.directCommerceInsertChat:
        return 'direct_commerce_insert_chat';
    }
  }

  static ChatResourceOccurrenceOperation fromWire(String value) {
    switch (value) {
      case 'share_to_chat':
        return ChatResourceOccurrenceOperation.shareToChat;
      case 'direct_commerce_insert_chat':
        return ChatResourceOccurrenceOperation.directCommerceInsertChat;
      default:
        throw FormatException('invalid resource occurrence operation: $value');
    }
  }
}

enum ChatResourceOccurrenceResourceType {
  profile,
  content,
  forSale,
  auction;

  String get wireValue {
    switch (this) {
      case ChatResourceOccurrenceResourceType.profile:
        return 'profile';
      case ChatResourceOccurrenceResourceType.content:
        return 'content';
      case ChatResourceOccurrenceResourceType.forSale:
        return 'for_sale';
      case ChatResourceOccurrenceResourceType.auction:
        return 'auction';
    }
  }

  static ChatResourceOccurrenceResourceType fromWire(String value) {
    switch (value) {
      case 'profile':
        return ChatResourceOccurrenceResourceType.profile;
      case 'content':
        return ChatResourceOccurrenceResourceType.content;
      case 'for_sale':
        return ChatResourceOccurrenceResourceType.forSale;
      case 'auction':
        return ChatResourceOccurrenceResourceType.auction;
      default:
        throw FormatException('invalid resource occurrence type: $value');
    }
  }
}

class ChatResourceOccurrenceRequest extends Equatable {
  final ChatResourceOccurrenceOperation operation;
  final ChatResourceOccurrenceResourceType resourceType;
  final String resourceId;

  const ChatResourceOccurrenceRequest._({
    required this.operation,
    required this.resourceType,
    required this.resourceId,
  });

  factory ChatResourceOccurrenceRequest({
    required ChatResourceOccurrenceOperation operation,
    required ChatResourceOccurrenceResourceType resourceType,
    required String resourceId,
  }) {
    final normalizedResourceId = resourceId.trim();
    if (normalizedResourceId.isEmpty) {
      throw const FormatException(
        'resource occurrence resource_id is required',
      );
    }
    if (!_isCanonicalUuid(normalizedResourceId)) {
      throw FormatException(
        'resource occurrence resource_id must be a canonical UUID',
      );
    }
    if (normalizedResourceId == '00000000-0000-0000-0000-000000000000') {
      throw const FormatException(
        'resource occurrence resource_id must not be the nil UUID',
      );
    }
    if (operation == ChatResourceOccurrenceOperation.directCommerceInsertChat &&
        !(resourceType == ChatResourceOccurrenceResourceType.forSale ||
            resourceType == ChatResourceOccurrenceResourceType.auction)) {
      throw const FormatException(
        'direct_commerce_insert_chat requires for_sale or auction',
      );
    }

    return ChatResourceOccurrenceRequest._(
      operation: operation,
      resourceType: resourceType,
      resourceId: normalizedResourceId,
    );
  }

  factory ChatResourceOccurrenceRequest.shareToChat({
    required ChatResourceOccurrenceResourceType resourceType,
    required String resourceId,
  }) {
    return ChatResourceOccurrenceRequest(
      operation: ChatResourceOccurrenceOperation.shareToChat,
      resourceType: resourceType,
      resourceId: resourceId,
    );
  }

  factory ChatResourceOccurrenceRequest.directCommerceInsertChat({
    required ChatResourceOccurrenceResourceType resourceType,
    required String resourceId,
  }) {
    return ChatResourceOccurrenceRequest(
      operation: ChatResourceOccurrenceOperation.directCommerceInsertChat,
      resourceType: resourceType,
      resourceId: resourceId,
    );
  }

  Map<String, dynamic> toJson() => {
    'operation': operation.wireValue,
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
  };

  @override
  List<Object?> get props => [operation, resourceType, resourceId];

  static bool _isCanonicalUuid(String value) {
    if (value.length != 36) return false;
    return RegExp(
      r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
    ).hasMatch(value);
  }
}
