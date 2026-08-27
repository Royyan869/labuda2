import 'package:labuda/shared/attachment/entities/share_reference.dart';

class ChatCommerceReferenceDto {
  final String id;
  final String roomId;
  final String targetType;
  final String targetId;
  final String creatorId;
  final DateTime createdAt;
  final Map<String, dynamic> displaySnapshot;

  const ChatCommerceReferenceDto({
    required this.id,
    required this.roomId,
    required this.targetType,
    required this.targetId,
    required this.creatorId,
    required this.createdAt,
    required this.displaySnapshot,
  });

  factory ChatCommerceReferenceDto.fromJson(Map<String, dynamic> json) {
    final snapshot =
        json['display_snapshot'] as Map<String, dynamic>? ?? const {};
    return ChatCommerceReferenceDto(
      id: json['id'] as String,
      roomId: json['room_id'] as String,
      targetType: json['target_type'] as String,
      targetId: json['target_id'] as String,
      creatorId: json['creator_id'] as String,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      displaySnapshot: Map<String, dynamic>.from(snapshot),
    );
  }

  ShareReference toShareReference() {
    final isAvailable = displaySnapshot['is_available'] == true;
    final isSold = displaySnapshot['is_sold'] == true;
    final isClosed = displaySnapshot['is_closed'] == true;
    final isDeleted = displaySnapshot['is_deleted'] == true;
    final title = (displaySnapshot['title'] as String?) ?? '';
    final imageUrl = displaySnapshot['image_url'] as String?;

    switch (targetType) {
      case 'auction':
        return ShareReference.auction(
          auctionId: targetId,
          title: title,
          imageUrl: imageUrl,
          isAvailable: isAvailable,
          isSold: isSold,
          isClosed: isClosed,
          isDeleted: isDeleted,
        );
      case 'for_sale':
      default:
        return ShareReference.forSale(
          forSaleId: targetId,
          title: title,
          imageUrl: imageUrl,
          isAvailable: isAvailable,
          isSold: isSold,
          isClosed: isClosed,
          isDeleted: isDeleted,
        );
    }
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'room_id': roomId,
    'target_type': targetType,
    'target_id': targetId,
    'creator_id': creatorId,
    'created_at': createdAt.toIso8601String(),
    'display_snapshot': displaySnapshot,
  };
}
