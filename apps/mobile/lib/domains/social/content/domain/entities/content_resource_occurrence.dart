import 'package:equatable/equatable.dart';

enum ContentResourceOccurrenceOperation {
  shareToFeed,
  directCommerceInsertContent,
}

extension ContentResourceOccurrenceOperationX
    on ContentResourceOccurrenceOperation {
  String get wireValue {
    switch (this) {
      case ContentResourceOccurrenceOperation.shareToFeed:
        return 'share_to_feed';
      case ContentResourceOccurrenceOperation.directCommerceInsertContent:
        return 'direct_commerce_insert_content';
    }
  }

  static ContentResourceOccurrenceOperation fromWire(String value) {
    switch (value) {
      case 'share_to_feed':
        return ContentResourceOccurrenceOperation.shareToFeed;
      case 'direct_commerce_insert_content':
        return ContentResourceOccurrenceOperation.directCommerceInsertContent;
      default:
        throw FormatException(
          'invalid content resource occurrence operation: $value',
        );
    }
  }
}

enum ContentResourceOccurrenceResourceType {
  profile,
  content,
  fixedPriceSale,
  auction,
}

extension ContentResourceOccurrenceResourceTypeX
    on ContentResourceOccurrenceResourceType {
  String get wireValue {
    switch (this) {
      case ContentResourceOccurrenceResourceType.profile:
        return 'profile';
      case ContentResourceOccurrenceResourceType.content:
        return 'content';
      case ContentResourceOccurrenceResourceType.fixedPriceSale:
        return 'fixed_price_sale';
      case ContentResourceOccurrenceResourceType.auction:
        return 'auction';
    }
  }

  static ContentResourceOccurrenceResourceType fromWire(String value) {
    switch (value) {
      case 'profile':
        return ContentResourceOccurrenceResourceType.profile;
      case 'content':
        return ContentResourceOccurrenceResourceType.content;
      case 'fixed_price_sale':
        return ContentResourceOccurrenceResourceType.fixedPriceSale;
      case 'auction':
        return ContentResourceOccurrenceResourceType.auction;
      default:
        throw FormatException(
          'invalid content resource occurrence type: $value',
        );
    }
  }
}

class ContentResourceOccurrence extends Equatable {
  final ContentResourceOccurrenceOperation operation;
  final ContentResourceOccurrenceResourceType resourceType;
  final String resourceId;

  const ContentResourceOccurrence._({
    required this.operation,
    required this.resourceType,
    required this.resourceId,
  });

  factory ContentResourceOccurrence({
    required ContentResourceOccurrenceOperation operation,
    required ContentResourceOccurrenceResourceType resourceType,
    required String resourceId,
  }) {
    final normalizedResourceId = resourceId.trim();
    if (!_isCanonicalUuid(normalizedResourceId)) {
      throw const FormatException(
        'content resource occurrence resource_id must be a canonical UUID',
      );
    }
    if (normalizedResourceId == '00000000-0000-0000-0000-000000000000') {
      throw const FormatException(
        'content resource occurrence resource_id must not be the nil UUID',
      );
    }
    if (operation ==
            ContentResourceOccurrenceOperation.directCommerceInsertContent &&
        !(resourceType ==
                ContentResourceOccurrenceResourceType.fixedPriceSale ||
            resourceType == ContentResourceOccurrenceResourceType.auction)) {
      throw const FormatException(
        'direct_commerce_insert_content requires fixed_price_sale or auction',
      );
    }

    return ContentResourceOccurrence._(
      operation: operation,
      resourceType: resourceType,
      resourceId: normalizedResourceId,
    );
  }

  factory ContentResourceOccurrence.shareToFeed({
    required ContentResourceOccurrenceResourceType resourceType,
    required String resourceId,
  }) {
    return ContentResourceOccurrence(
      operation: ContentResourceOccurrenceOperation.shareToFeed,
      resourceType: resourceType,
      resourceId: resourceId,
    );
  }

  factory ContentResourceOccurrence.directCommerceInsertContent({
    required ContentResourceOccurrenceResourceType resourceType,
    required String resourceId,
  }) {
    return ContentResourceOccurrence(
      operation: ContentResourceOccurrenceOperation.directCommerceInsertContent,
      resourceType: resourceType,
      resourceId: resourceId,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'operation': operation.wireValue,
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
  };

  @override
  List<Object?> get props => [operation, resourceType, resourceId];
}

bool _isCanonicalUuid(String value) {
  if (value.length != 36) return false;
  return RegExp(
    r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
  ).hasMatch(value);
}
