enum TargetType { listing, auction }

enum IntentType { bookmark, watch }

class SavedItemModel {
  final String id;
  final String userId;
  final TargetType targetType;
  final String targetId;
  final IntentType intentType; // NEW: Semantic intent
  final String? sellerId;
  final DateTime createdAt;

  // For listings
  final String? listingTitle;
  final int? listingPrice;
  final String? listingType;
  final int? quantityAvailable;
  final String? listingStatus;
  final String? listingVisibility;
  final List<String>? listingMediaUrls;

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
    this.listingTitle,
    this.listingPrice,
    this.listingType,
    this.quantityAvailable,
    this.listingStatus,
    this.listingVisibility,
    this.listingMediaUrls,
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
      targetType: targetTypeStr == 'listing'
          ? TargetType.listing
          : TargetType.auction,
      targetId: json['target_id'] as String,
      intentType: intentTypeStr == 'bookmark'
          ? IntentType.bookmark
          : IntentType.watch,
      sellerId: json['seller_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      listingTitle: json['listing_title'] as String?,
      listingPrice: json['listing_price'] as int?,
      listingType: json['listing_type'] as String?,
      quantityAvailable: json['quantity_available'] as int?,
      listingStatus: json['listing_status'] as String?,
      listingVisibility: json['listing_visibility'] as String?,
      listingMediaUrls: json['listing_media_urls'] != null
          ? List<String>.from(json['listing_media_urls'] as List)
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
      'target_type': targetType == TargetType.listing ? 'listing' : 'auction',
      'target_id': targetId,
      'intent_type': intentType == IntentType.bookmark ? 'bookmark' : 'watch',
      'seller_id': sellerId,
      'created_at': createdAt.toIso8601String(),
      if (listingTitle != null) 'listing_title': listingTitle,
      if (listingPrice != null) 'listing_price': listingPrice,
      if (listingType != null) 'listing_type': listingType,
      if (quantityAvailable != null) 'quantity_available': quantityAvailable,
      if (listingStatus != null) 'listing_status': listingStatus,
      if (listingVisibility != null) 'listing_visibility': listingVisibility,
      if (listingMediaUrls != null) 'listing_media_urls': listingMediaUrls,
      if (auctionTitle != null) 'auction_title': auctionTitle,
      if (auctionStatus != null) 'auction_status': auctionStatus,
      if (startPrice != null) 'start_price': startPrice,
      if (currentBid != null) 'current_bid': currentBid,
      if (endAt != null) 'end_at': endAt!.toIso8601String(),
    };
  }

  bool get isListing => targetType == TargetType.listing;
  bool get isAuction => targetType == TargetType.auction;
  bool get isBookmark => intentType == IntentType.bookmark;
  bool get isWatch => intentType == IntentType.watch;
}
