import 'package:equatable/equatable.dart';

enum ContentResourceProjectionState { live, tombstone }

extension ContentResourceProjectionStateX on ContentResourceProjectionState {
  String get wireValue {
    switch (this) {
      case ContentResourceProjectionState.live:
        return 'LIVE';
      case ContentResourceProjectionState.tombstone:
        return 'TOMBSTONE';
    }
  }

  static ContentResourceProjectionState fromWire(String value) {
    switch (value) {
      case 'LIVE':
        return ContentResourceProjectionState.live;
      case 'TOMBSTONE':
        return ContentResourceProjectionState.tombstone;
      default:
        throw FormatException('invalid resource projection state: $value');
    }
  }
}

enum ContentResourceProjectionType { profile, content, fixedPriceSale, auction }

extension ContentResourceProjectionTypeX on ContentResourceProjectionType {
  String get wireValue {
    switch (this) {
      case ContentResourceProjectionType.profile:
        return 'profile';
      case ContentResourceProjectionType.content:
        return 'content';
      case ContentResourceProjectionType.fixedPriceSale:
        return 'fixed_price_sale';
      case ContentResourceProjectionType.auction:
        return 'auction';
    }
  }

  String get displayLabel {
    switch (this) {
      case ContentResourceProjectionType.profile:
        return 'Profil';
      case ContentResourceProjectionType.content:
        return 'Konten';
      case ContentResourceProjectionType.fixedPriceSale:
        return 'Listing';
      case ContentResourceProjectionType.auction:
        return 'Lelang';
    }
  }

  static ContentResourceProjectionType fromWire(String value) {
    switch (value) {
      case 'profile':
        return ContentResourceProjectionType.profile;
      case 'content':
        return ContentResourceProjectionType.content;
      case 'fixed_price_sale':
        return ContentResourceProjectionType.fixedPriceSale;
      case 'auction':
        return ContentResourceProjectionType.auction;
      default:
        throw FormatException('invalid resource type: $value');
    }
  }
}

class ContentResourceProjectionMediaRef extends Equatable {
  final String url;
  final String? kind;
  final int? width;
  final int? height;

  const ContentResourceProjectionMediaRef({
    required this.url,
    this.kind,
    this.width,
    this.height,
  });

  factory ContentResourceProjectionMediaRef.fromJson(
    Map<String, dynamic> json,
  ) {
    final url = json['url'];
    if (url is! String || url.trim().isEmpty) {
      throw const FormatException('media ref requires url');
    }
    return ContentResourceProjectionMediaRef(
      url: url,
      kind: json['kind'] as String?,
      width: (json['width'] as num?)?.toInt(),
      height: (json['height'] as num?)?.toInt(),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'url': url,
    if (kind != null) 'kind': kind,
    if (width != null) 'width': width,
    if (height != null) 'height': height,
  };

  @override
  List<Object?> get props => [url, kind, width, height];
}

class ContentResourceProjectionUserCard extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;
  final String? lifecycle;

  const ContentResourceProjectionUserCard({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ContentResourceProjectionUserCard.fromJson(
    Map<String, dynamic> json,
  ) {
    final id = json['id'];
    final username = json['username'];
    if (id is! String || id.trim().isEmpty) {
      throw const FormatException('user card requires id');
    }
    if (username is! String || username.trim().isEmpty) {
      throw const FormatException('user card requires username');
    }
    return ContentResourceProjectionUserCard(
      id: id,
      username: username,
      avatarUrl: json['avatar_url'] as String?,
      lifecycle: json['lifecycle'] as String?,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'id': id,
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (lifecycle != null) 'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [id, username, avatarUrl, lifecycle];
}

class ContentResourceProjectionSellerCard extends Equatable {
  final ContentResourceProjectionUserCard user;
  final String? farmName;
  final String? avatarUrl;
  final String? lifecycle;

  const ContentResourceProjectionSellerCard({
    required this.user,
    this.farmName,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ContentResourceProjectionSellerCard.fromJson(
    Map<String, dynamic> json,
  ) {
    final user = json['user'];
    if (user is! Map<String, dynamic>) {
      throw const FormatException('seller card requires user');
    }
    return ContentResourceProjectionSellerCard(
      user: ContentResourceProjectionUserCard.fromJson(user),
      farmName: json['farm_name'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      lifecycle: json['lifecycle'] as String?,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'user': user.toJson(),
    if (farmName != null) 'farm_name': farmName,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (lifecycle != null) 'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [user, farmName, avatarUrl, lifecycle];
}

class ContentResourceProjectionNestedResource extends Equatable {
  final ContentResourceProjectionType resourceType;
  final String resourceId;

  const ContentResourceProjectionNestedResource({
    required this.resourceType,
    required this.resourceId,
  });

  factory ContentResourceProjectionNestedResource.fromJson(
    Map<String, dynamic> json,
  ) {
    final rawType = json['resource_type'];
    final rawId = json['resource_id'];
    if (rawType is! String || rawType.isEmpty) {
      throw const FormatException('nested_resource requires resource_type');
    }
    if (rawId is! String || rawId.isEmpty) {
      throw const FormatException('nested_resource requires resource_id');
    }
    return ContentResourceProjectionNestedResource(
      resourceType: ContentResourceProjectionTypeX.fromWire(rawType),
      resourceId: rawId,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
  };

  @override
  List<Object?> get props => [resourceType, resourceId];
}

class ContentResourceProjectionProfilePayload extends Equatable {
  final String username;
  final String? avatarUrl;
  final String lifecycle;

  const ContentResourceProjectionProfilePayload({
    required this.username,
    this.avatarUrl,
    required this.lifecycle,
  });

  factory ContentResourceProjectionProfilePayload.fromJson(
    Map<String, dynamic> json,
  ) {
    final username = json['username'];
    final lifecycle = json['lifecycle'];
    if (username is! String || username.trim().isEmpty) {
      throw const FormatException('profile payload requires username');
    }
    if (lifecycle is! String || lifecycle.trim().isEmpty) {
      throw const FormatException('profile payload requires lifecycle');
    }
    return ContentResourceProjectionProfilePayload(
      username: username,
      avatarUrl: json['avatar_url'] as String?,
      lifecycle: lifecycle,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [username, avatarUrl, lifecycle];
}

class ContentResourceProjectionContentPayload extends Equatable {
  final String? caption;
  final List<ContentResourceProjectionMediaRef> media;
  final String lifecycle;
  final String createdAt;
  final ContentResourceProjectionUserCard author;
  final ContentResourceProjectionNestedResource? nestedResource;

  const ContentResourceProjectionContentPayload({
    required this.caption,
    required this.media,
    required this.lifecycle,
    required this.createdAt,
    required this.author,
    this.nestedResource,
  });

  factory ContentResourceProjectionContentPayload.fromJson(
    Map<String, dynamic> json,
  ) {
    final lifecycle = json['lifecycle'];
    final createdAt = json['created_at'];
    final author = json['author'];
    if (lifecycle is! String || lifecycle.trim().isEmpty) {
      throw const FormatException('content payload requires lifecycle');
    }
    if (createdAt is! String || createdAt.trim().isEmpty) {
      throw const FormatException('content payload requires created_at');
    }
    if (author is! Map<String, dynamic>) {
      throw const FormatException('content payload requires author');
    }
    final mediaRaw = json['media'];
    if (mediaRaw is! List) {
      throw const FormatException('content payload requires media');
    }
    final media = <ContentResourceProjectionMediaRef>[];
    for (final item in mediaRaw) {
      if (item is! Map<String, dynamic>) {
        throw const FormatException('content payload requires media items');
      }
      media.add(ContentResourceProjectionMediaRef.fromJson(item));
    }
    final nestedResourceRaw = json['nested_resource'];
    if (nestedResourceRaw != null &&
        nestedResourceRaw is! Map<String, dynamic>) {
      throw const FormatException('content payload requires nested_resource');
    }
    return ContentResourceProjectionContentPayload(
      caption: json['caption'] as String?,
      media: media,
      lifecycle: lifecycle,
      createdAt: createdAt,
      author: ContentResourceProjectionUserCard.fromJson(author),
      nestedResource: nestedResourceRaw is Map<String, dynamic>
          ? ContentResourceProjectionNestedResource.fromJson(nestedResourceRaw)
          : null,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    if (caption != null) 'caption': caption,
    'media': media.map((item) => item.toJson()).toList(growable: false),
    'lifecycle': lifecycle,
    'created_at': createdAt,
    'author': author.toJson(),
    if (nestedResource != null) 'nested_resource': nestedResource!.toJson(),
  };

  @override
  List<Object?> get props => [
    caption,
    media,
    lifecycle,
    createdAt,
    author,
    nestedResource,
  ];
}

class ContentResourceProjectionFixedPriceSalePayload extends Equatable {
  final String title;
  final List<ContentResourceProjectionMediaRef> media;
  final String? thumbnailUrl;
  final int price;
  final String status;
  final int quantityAvailable;
  final bool canInteract;
  final ContentResourceProjectionSellerCard seller;

  const ContentResourceProjectionFixedPriceSalePayload({
    required this.title,
    required this.media,
    this.thumbnailUrl,
    required this.price,
    required this.status,
    required this.quantityAvailable,
    required this.canInteract,
    required this.seller,
  });

  factory ContentResourceProjectionFixedPriceSalePayload.fromJson(
    Map<String, dynamic> json,
  ) {
    final title = json['title'];
    final price = json['price'];
    final status = json['status'];
    final quantity = json['quantity_available'];
    final canInteract = json['can_interact'];
    final seller = json['seller'];
    final mediaRaw = json['media'];
    if (title is! String || title.trim().isEmpty) {
      throw const FormatException('fixed_price_sale requires title');
    }
    if (price is! num) {
      throw const FormatException('fixed_price_sale requires price');
    }
    if (status is! String || status.trim().isEmpty) {
      throw const FormatException('fixed_price_sale requires status');
    }
    if (quantity is! num) {
      throw const FormatException(
        'fixed_price_sale requires quantity_available',
      );
    }
    if (canInteract is! bool) {
      throw const FormatException('fixed_price_sale requires can_interact');
    }
    if (seller is! Map<String, dynamic>) {
      throw const FormatException('fixed_price_sale requires seller');
    }
    if (mediaRaw is! List) {
      throw const FormatException('fixed_price_sale requires media');
    }
    final media = <ContentResourceProjectionMediaRef>[];
    for (final item in mediaRaw) {
      if (item is! Map<String, dynamic>) {
        throw const FormatException('fixed_price_sale requires media items');
      }
      media.add(ContentResourceProjectionMediaRef.fromJson(item));
    }
    return ContentResourceProjectionFixedPriceSalePayload(
      title: title,
      media: media,
      thumbnailUrl: json['thumbnail_url'] as String?,
      price: price.toInt(),
      status: status,
      quantityAvailable: quantity.toInt(),
      canInteract: canInteract,
      seller: ContentResourceProjectionSellerCard.fromJson(seller),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'title': title,
    'media': media.map((item) => item.toJson()).toList(growable: false),
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
    'price': price,
    'status': status,
    'quantity_available': quantityAvailable,
    'can_interact': canInteract,
    'seller': seller.toJson(),
  };

  @override
  List<Object?> get props => [
    title,
    media,
    thumbnailUrl,
    price,
    status,
    quantityAvailable,
    canInteract,
    seller,
  ];
}

class ContentResourceProjectionAuctionPayload extends Equatable {
  final String title;
  final List<ContentResourceProjectionMediaRef> media;
  final String? thumbnailUrl;
  final int? currentBid;
  final int? buyNowPrice;
  final String endAt;
  final String lifecycle;
  final bool canInteract;
  final ContentResourceProjectionSellerCard seller;

  const ContentResourceProjectionAuctionPayload({
    required this.title,
    required this.media,
    this.thumbnailUrl,
    this.currentBid,
    this.buyNowPrice,
    required this.endAt,
    required this.lifecycle,
    required this.canInteract,
    required this.seller,
  });

  factory ContentResourceProjectionAuctionPayload.fromJson(
    Map<String, dynamic> json,
  ) {
    final title = json['title'];
    final endAt = json['end_at'];
    final lifecycle = json['lifecycle'];
    final canInteract = json['can_interact'];
    final seller = json['seller'];
    final mediaRaw = json['media'];
    if (title is! String || title.trim().isEmpty) {
      throw const FormatException('auction requires title');
    }
    if (endAt is! String || endAt.trim().isEmpty) {
      throw const FormatException('auction requires end_at');
    }
    if (lifecycle is! String || lifecycle.trim().isEmpty) {
      throw const FormatException('auction requires lifecycle');
    }
    if (canInteract is! bool) {
      throw const FormatException('auction requires can_interact');
    }
    if (seller is! Map<String, dynamic>) {
      throw const FormatException('auction requires seller');
    }
    if (mediaRaw is! List) {
      throw const FormatException('auction requires media');
    }
    final media = <ContentResourceProjectionMediaRef>[];
    for (final item in mediaRaw) {
      if (item is! Map<String, dynamic>) {
        throw const FormatException('auction requires media items');
      }
      media.add(ContentResourceProjectionMediaRef.fromJson(item));
    }
    final currentBidRaw = json['current_bid'];
    final buyNowPriceRaw = json['buy_now_price'];
    if (currentBidRaw != null && currentBidRaw is! num) {
      throw const FormatException('auction current_bid must be numeric');
    }
    if (buyNowPriceRaw != null && buyNowPriceRaw is! num) {
      throw const FormatException('auction buy_now_price must be numeric');
    }
    return ContentResourceProjectionAuctionPayload(
      title: title,
      media: media,
      thumbnailUrl: json['thumbnail_url'] as String?,
      currentBid: (currentBidRaw as num?)?.toInt(),
      buyNowPrice: (buyNowPriceRaw as num?)?.toInt(),
      endAt: endAt,
      lifecycle: lifecycle,
      canInteract: canInteract,
      seller: ContentResourceProjectionSellerCard.fromJson(seller),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'title': title,
    'media': media.map((item) => item.toJson()).toList(growable: false),
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
    if (currentBid != null) 'current_bid': currentBid,
    if (buyNowPrice != null) 'buy_now_price': buyNowPrice,
    'end_at': endAt,
    'lifecycle': lifecycle,
    'can_interact': canInteract,
    'seller': seller.toJson(),
  };

  @override
  List<Object?> get props => [
    title,
    media,
    thumbnailUrl,
    currentBid,
    buyNowPrice,
    endAt,
    lifecycle,
    canInteract,
    seller,
  ];
}

class ContentResourceProjection extends Equatable {
  final ContentResourceProjectionState state;
  final ContentResourceProjectionType resourceType;
  final String resourceId;

  final ContentResourceProjectionProfilePayload? profile;
  final ContentResourceProjectionContentPayload? content;
  final ContentResourceProjectionFixedPriceSalePayload? fixedPriceSale;
  final ContentResourceProjectionAuctionPayload? auction;

  const ContentResourceProjection({
    required this.state,
    required this.resourceType,
    required this.resourceId,
    this.profile,
    this.content,
    this.fixedPriceSale,
    this.auction,
  });

  bool get isLive => state == ContentResourceProjectionState.live;
  bool get isTombstone => state == ContentResourceProjectionState.tombstone;

  ContentResourceProjectionProfilePayload? get profilePayload => profile;
  ContentResourceProjectionContentPayload? get contentPayload => content;
  ContentResourceProjectionFixedPriceSalePayload? get fixedPriceSalePayload =>
      fixedPriceSale;
  ContentResourceProjectionAuctionPayload? get auctionPayload => auction;

  String get typeLabel => resourceType.displayLabel;

  String get canonicalPath {
    switch (resourceType) {
      case ContentResourceProjectionType.profile:
        return '/user/$resourceId';
      case ContentResourceProjectionType.content:
        return '/content/$resourceId';
      case ContentResourceProjectionType.fixedPriceSale:
        return '/for-sale/$resourceId';
      case ContentResourceProjectionType.auction:
        return '/auction/$resourceId';
    }
  }

  String get titleText {
    if (!isLive) {
      return '${resourceType.displayLabel} tidak tersedia';
    }
    switch (resourceType) {
      case ContentResourceProjectionType.profile:
        return '@${profile?.username ?? ''}';
      case ContentResourceProjectionType.content:
        final caption = content?.caption?.trim();
        return caption != null && caption.isNotEmpty ? caption : 'Konten';
      case ContentResourceProjectionType.fixedPriceSale:
        return fixedPriceSale?.title ?? 'Listing';
      case ContentResourceProjectionType.auction:
        return auction?.title ?? 'Lelang';
    }
  }

  String? get imageUrl {
    if (!isLive) return null;
    switch (resourceType) {
      case ContentResourceProjectionType.profile:
        return profile?.avatarUrl;
      case ContentResourceProjectionType.content:
        return _firstMediaUrl(content?.media);
      case ContentResourceProjectionType.fixedPriceSale:
        return fixedPriceSale?.thumbnailUrl ??
            _firstMediaUrl(fixedPriceSale?.media);
      case ContentResourceProjectionType.auction:
        return auction?.thumbnailUrl ?? _firstMediaUrl(auction?.media);
    }
  }

  String? get valueText {
    if (!isLive) return null;
    switch (resourceType) {
      case ContentResourceProjectionType.profile:
        return profile?.username;
      case ContentResourceProjectionType.content:
        return content?.author.username;
      case ContentResourceProjectionType.fixedPriceSale:
        return fixedPriceSale == null
            ? null
            : 'Rp ${fixedPriceSale!.price.toStringAsFixed(0)}';
      case ContentResourceProjectionType.auction:
        final price = auction?.currentBid ?? auction?.buyNowPrice;
        return price == null ? null : 'Rp ${price.toStringAsFixed(0)}';
    }
  }

  String get statusText {
    if (!isLive) return 'TOMBSTONE';
    switch (resourceType) {
      case ContentResourceProjectionType.profile:
        return profile?.lifecycle ?? 'LIVE';
      case ContentResourceProjectionType.content:
        return content?.lifecycle ?? 'LIVE';
      case ContentResourceProjectionType.fixedPriceSale:
        return fixedPriceSale?.status ?? 'LIVE';
      case ContentResourceProjectionType.auction:
        return auction?.lifecycle ?? 'LIVE';
    }
  }

  String? get nestedResourceLabel {
    final nested = content?.nestedResource;
    if (nested == null) return null;
    return nested.resourceType.displayLabel;
  }

  factory ContentResourceProjection.fromJson(Map<String, dynamic> json) {
    final rawState = json['state'];
    final rawType = json['resource_type'];
    final rawId = json['resource_id'];
    if (rawState is! String || rawState.isEmpty) {
      throw const FormatException('resource_projection requires state');
    }
    if (rawType is! String || rawType.isEmpty) {
      throw const FormatException('resource_projection requires resource_type');
    }
    if (rawId is! String || rawId.isEmpty) {
      throw const FormatException('resource_projection requires resource_id');
    }

    final state = ContentResourceProjectionStateX.fromWire(rawState);
    final resourceType = ContentResourceProjectionTypeX.fromWire(rawType);
    final profile = json['profile'];
    final content = json['content'];
    final fixedPriceSale = json['fixed_price_sale'];
    final auction = json['auction'];

    switch (state) {
      case ContentResourceProjectionState.live:
        switch (resourceType) {
          case ContentResourceProjectionType.profile:
            return ContentResourceProjection(
              state: state,
              resourceType: resourceType,
              resourceId: rawId,
              profile: profile is Map<String, dynamic>
                  ? ContentResourceProjectionProfilePayload.fromJson(profile)
                  : throw const FormatException(
                      'LIVE profile projection requires profile payload',
                    ),
            );
          case ContentResourceProjectionType.content:
            return ContentResourceProjection(
              state: state,
              resourceType: resourceType,
              resourceId: rawId,
              content: content is Map<String, dynamic>
                  ? ContentResourceProjectionContentPayload.fromJson(content)
                  : throw const FormatException(
                      'LIVE content projection requires content payload',
                    ),
            );
          case ContentResourceProjectionType.fixedPriceSale:
            return ContentResourceProjection(
              state: state,
              resourceType: resourceType,
              resourceId: rawId,
              fixedPriceSale: fixedPriceSale is Map<String, dynamic>
                  ? ContentResourceProjectionFixedPriceSalePayload.fromJson(
                      fixedPriceSale,
                    )
                  : throw const FormatException(
                      'LIVE fixed_price_sale projection requires fixed_price_sale payload',
                    ),
            );
          case ContentResourceProjectionType.auction:
            return ContentResourceProjection(
              state: state,
              resourceType: resourceType,
              resourceId: rawId,
              auction: auction is Map<String, dynamic>
                  ? ContentResourceProjectionAuctionPayload.fromJson(auction)
                  : throw const FormatException(
                      'LIVE auction projection requires auction payload',
                    ),
            );
        }
      case ContentResourceProjectionState.tombstone:
        if (profile != null ||
            content != null ||
            fixedPriceSale != null ||
            auction != null) {
          throw const FormatException(
            'TOMBSTONE resource_projection must not carry live payload',
          );
        }
        return ContentResourceProjection(
          state: state,
          resourceType: resourceType,
          resourceId: rawId,
        );
    }
  }

  Map<String, dynamic> toJson() {
    final out = <String, dynamic>{
      'state': state.wireValue,
      'resource_type': resourceType.wireValue,
      'resource_id': resourceId,
    };
    if (profile != null) out['profile'] = profile!.toJson();
    if (content != null) out['content'] = content!.toJson();
    if (fixedPriceSale != null) {
      out['fixed_price_sale'] = fixedPriceSale!.toJson();
    }
    if (auction != null) out['auction'] = auction!.toJson();
    return out;
  }

  @override
  List<Object?> get props => [
    state,
    resourceType,
    resourceId,
    profile,
    content,
    fixedPriceSale,
    auction,
  ];
}

String? _firstMediaUrl(List<ContentResourceProjectionMediaRef>? media) {
  if (media == null || media.isEmpty) return null;
  return media.first.url;
}
