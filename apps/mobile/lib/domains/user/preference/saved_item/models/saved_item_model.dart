enum TargetType { forSale, auction }

enum IntentType { bookmark, watch }

class SavedItemModel {
  final String id;
  final String userId;
  final TargetType targetType;
  final String targetId;
  final IntentType intentType; // NEW: Semantic intent
  final String? sellerId;
  final DateTime createdAt;

  // For for-sale items
  final String? forSaleTitle;
  final int? forSalePrice;
  final int? quantityAvailable;
  final String? forSaleStatus;
  final String? forSaleVisibility;
  final List<String>? forSaleMediaUrls;

  // For auctions
  final String? auctionTitle;
  final String? auctionStatus;
  final int? startPrice;
  final int? currentBid;
  final DateTime? endAt;

  SavedItemModel({
    required this.id,
    required this.userId,
    required this.targetType,
    required this.targetId,
    required this.intentType,
    this.sellerId,
    required this.createdAt,
    this.forSaleTitle,
    this.forSalePrice,
    this.quantityAvailable,
    this.forSaleStatus,
    this.forSaleVisibility,
    this.forSaleMediaUrls,
    this.auctionTitle,
    this.auctionStatus,
    this.startPrice,
    this.currentBid,
    this.endAt,
  });

  factory SavedItemModel.fromJson(Map<String, dynamic> json) {
    final targetTypeStr = json['target_type'] as String;
    final intentTypeStr = json['intent_type'] as String;
    return SavedItemModel(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      targetType: targetTypeStr == 'for_sale'
          ? TargetType.forSale
          : TargetType.auction,
      targetId: json['target_id'] as String,
      intentType: intentTypeStr == 'bookmark'
          ? IntentType.bookmark
          : IntentType.watch,
      sellerId: json['seller_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      forSaleTitle: json['for_sale_title'] as String?,
      forSalePrice: json['for_sale_price'] as int?,
      quantityAvailable: json['quantity_available'] as int?,
      forSaleStatus: json['for_sale_status'] as String?,
      forSaleVisibility: json['for_sale_visibility'] as String?,
      forSaleMediaUrls: json['for_sale_media_urls'] != null
          ? List<String>.from(json['for_sale_media_urls'] as List)
          : null,
      auctionTitle: json['auction_title'] as String?,
      auctionStatus: json['auction_status'] as String?,
      startPrice: json['start_price'] as int?,
      currentBid: json['current_bid'] as int?,
      endAt: json['end_at'] != null
          ? DateTime.parse(json['end_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'user_id': userId,
      'target_type': targetType == TargetType.forSale ? 'for_sale' : 'auction',
      'target_id': targetId,
      'intent_type': intentType == IntentType.bookmark ? 'bookmark' : 'watch',
      'seller_id': sellerId,
      'created_at': createdAt.toIso8601String(),
      if (forSaleTitle != null) 'for_sale_title': forSaleTitle,
      if (forSalePrice != null) 'for_sale_price': forSalePrice,
      if (quantityAvailable != null) 'quantity_available': quantityAvailable,
      if (forSaleStatus != null) 'for_sale_status': forSaleStatus,
      if (forSaleVisibility != null) 'for_sale_visibility': forSaleVisibility,
      if (forSaleMediaUrls != null) 'for_sale_media_urls': forSaleMediaUrls,
      if (auctionTitle != null) 'auction_title': auctionTitle,
      if (auctionStatus != null) 'auction_status': auctionStatus,
      if (startPrice != null) 'start_price': startPrice,
      if (currentBid != null) 'current_bid': currentBid,
      if (endAt != null) 'end_at': endAt!.toIso8601String(),
    };
  }

  bool get isForSale => targetType == TargetType.forSale;
  bool get isAuction => targetType == TargetType.auction;
  bool get isBookmark => intentType == IntentType.bookmark;
  bool get isWatch => intentType == IntentType.watch;
}
