import 'package:equatable/equatable.dart';

enum ChatResourceProjectionState { live, tombstone }

extension ChatResourceProjectionStateX on ChatResourceProjectionState {
  String get wireValue {
    switch (this) {
      case ChatResourceProjectionState.live:
        return 'LIVE';
      case ChatResourceProjectionState.tombstone:
        return 'TOMBSTONE';
    }
  }
}

ChatResourceProjectionState _chatResourceProjectionStateFromWire(String value) {
  switch (value) {
    case 'LIVE':
      return ChatResourceProjectionState.live;
    case 'TOMBSTONE':
      return ChatResourceProjectionState.tombstone;
    default:
      throw FormatException('invalid resource projection state: $value');
  }
}

enum ChatResourceType { profile, content, forSale, auction }

extension ChatResourceTypeX on ChatResourceType {
  String get wireValue {
    switch (this) {
      case ChatResourceType.profile:
        return 'profile';
      case ChatResourceType.content:
        return 'content';
      case ChatResourceType.forSale:
        return 'for_sale';
      case ChatResourceType.auction:
        return 'auction';
    }
  }

  String get displayLabel {
    switch (this) {
      case ChatResourceType.profile:
        return 'Profil';
      case ChatResourceType.content:
        return 'Konten';
      case ChatResourceType.forSale:
        return 'For Sale';
      case ChatResourceType.auction:
        return 'Lelang';
    }
  }
}

ChatResourceType _chatResourceTypeFromWire(String value) {
  switch (value) {
    case 'profile':
      return ChatResourceType.profile;
    case 'content':
      return ChatResourceType.content;
    case 'for_sale':
      return ChatResourceType.forSale;
    case 'auction':
      return ChatResourceType.auction;
    default:
      throw FormatException('invalid resource type: $value');
  }
}

class ChatResourceViewerCapabilities extends Equatable {
  final bool canView;
  final bool canInteract;
  final bool blockedByTombstone;

  const ChatResourceViewerCapabilities({
    required this.canView,
    required this.canInteract,
    required this.blockedByTombstone,
  });

  const ChatResourceViewerCapabilities.live({required this.canInteract})
    : canView = true,
      blockedByTombstone = false;

  const ChatResourceViewerCapabilities.tombstone()
    : canView = false,
      canInteract = false,
      blockedByTombstone = true;

  factory ChatResourceViewerCapabilities.fromJson(
    Map<String, dynamic> json, {
    required ChatResourceProjectionState state,
  }) {
    final canView = json['can_view'];
    final canInteract = json['can_interact'];
    final blocked = json['blocked_by_tombstone'];
    if (canView is! bool || canInteract is! bool || blocked is! bool) {
      throw const FormatException('viewer_capabilities must contain booleans');
    }

    final caps = ChatResourceViewerCapabilities(
      canView: canView,
      canInteract: canInteract,
      blockedByTombstone: blocked,
    );
    caps.validate(state: state);
    return caps;
  }

  void validate({required ChatResourceProjectionState state}) {
    switch (state) {
      case ChatResourceProjectionState.live:
        if (!canView) {
          throw const FormatException('LIVE projection requires can_view=true');
        }
        if (blockedByTombstone) {
          throw const FormatException(
            'LIVE projection requires blocked_by_tombstone=false',
          );
        }
      case ChatResourceProjectionState.tombstone:
        if (canView) {
          throw const FormatException('TOMBSTONE requires can_view=false');
        }
        if (canInteract) {
          throw const FormatException('TOMBSTONE requires can_interact=false');
        }
        if (!blockedByTombstone) {
          throw const FormatException(
            'TOMBSTONE requires blocked_by_tombstone=true',
          );
        }
    }
  }

  Map<String, dynamic> toJson() => {
    'can_view': canView,
    'can_interact': canInteract,
    'blocked_by_tombstone': blockedByTombstone,
  };

  @override
  List<Object?> get props => [canView, canInteract, blockedByTombstone];
}

class ChatCommerceActionCapabilities extends Equatable {
  final String role;
  final bool canChat;
  final bool canNegotiate;
  final bool canBuy;
  final bool canBid;
  final bool canManage;

  const ChatCommerceActionCapabilities({
    required this.role,
    required this.canChat,
    required this.canNegotiate,
    required this.canBuy,
    required this.canBid,
    required this.canManage,
  });

  factory ChatCommerceActionCapabilities.fromJson(Map<String, dynamic> json) {
    final role = json['role'];
    final canChat = json['can_chat'];
    final canNegotiate = json['can_negotiate'];
    final canBuy = json['can_buy'];
    final canBid = json['can_bid'];
    final canManage = json['can_manage'];
    if (role is! String ||
        canChat is! bool ||
        canNegotiate is! bool ||
        canBuy is! bool ||
        canBid is! bool ||
        canManage is! bool) {
      throw const FormatException(
        'commerce_actions must contain role and boolean flags',
      );
    }
    return ChatCommerceActionCapabilities(
      role: role,
      canChat: canChat,
      canNegotiate: canNegotiate,
      canBuy: canBuy,
      canBid: canBid,
      canManage: canManage,
    );
  }

  Map<String, dynamic> toJson() => {
    'role': role,
    'can_chat': canChat,
    'can_negotiate': canNegotiate,
    'can_buy': canBuy,
    'can_bid': canBid,
    'can_manage': canManage,
  };

  bool get hasAnyAction =>
      canChat || canNegotiate || canBuy || canBid || canManage;

  @override
  List<Object?> get props => [
    role,
    canChat,
    canNegotiate,
    canBuy,
    canBid,
    canManage,
  ];
}

abstract class ChatResourceProjectionPayload extends Equatable {
  const ChatResourceProjectionPayload();

  ChatResourceType get resourceType;

  Map<String, dynamic> toJson();
}

class ChatResourceUserCard extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;
  final String? lifecycle;

  const ChatResourceUserCard({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ChatResourceUserCard.fromJson(Map<String, dynamic> json) {
    final id = json['id'];
    final username = json['username'];
    if (id is! String || id.isEmpty) {
      throw const FormatException('user card requires id');
    }
    if (username is! String || username.isEmpty) {
      throw const FormatException('user card requires username');
    }
    return ChatResourceUserCard(
      id: id,
      username: username,
      avatarUrl: json['avatar_url'] as String?,
      lifecycle: json['lifecycle'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (lifecycle != null) 'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [id, username, avatarUrl, lifecycle];
}

class ChatResourceSellerCard extends Equatable {
  final ChatResourceUserCard user;
  final String? farmName;
  final String? avatarUrl;
  final String? lifecycle;

  const ChatResourceSellerCard({
    required this.user,
    this.farmName,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ChatResourceSellerCard.fromJson(Map<String, dynamic> json) {
    final user = json['user'];
    if (user is! Map<String, dynamic>) {
      throw const FormatException('seller card requires user');
    }
    return ChatResourceSellerCard(
      user: ChatResourceUserCard.fromJson(user),
      farmName: json['farm_name'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      lifecycle: json['lifecycle'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'user': user.toJson(),
    if (farmName != null) 'farm_name': farmName,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (lifecycle != null) 'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [user, farmName, avatarUrl, lifecycle];
}

class ChatContentMediaRef extends Equatable {
  final String url;
  final String? kind;
  final int? width;
  final int? height;

  const ChatContentMediaRef({
    required this.url,
    this.kind,
    this.width,
    this.height,
  });

  factory ChatContentMediaRef.fromJson(Map<String, dynamic> json) {
    final url = json['url'];
    if (url is! String || url.isEmpty) {
      throw const FormatException('media item requires url');
    }
    return ChatContentMediaRef(
      url: url,
      kind: json['kind'] as String?,
      width: (json['width'] as num?)?.toInt(),
      height: (json['height'] as num?)?.toInt(),
    );
  }

  Map<String, dynamic> toJson() => {
    'url': url,
    if (kind != null) 'kind': kind,
    if (width != null) 'width': width,
    if (height != null) 'height': height,
  };

  @override
  List<Object?> get props => [url, kind, width, height];
}

class ChatNestedResourceIndicator extends Equatable {
  final ChatResourceType resourceType;
  final String resourceId;

  const ChatNestedResourceIndicator({
    required this.resourceType,
    required this.resourceId,
  });

  factory ChatNestedResourceIndicator.fromJson(Map<String, dynamic> json) {
    final type = json['resource_type'];
    final id = json['resource_id'];
    if (type is! String || type.isEmpty) {
      throw const FormatException('nested_resource requires resource_type');
    }
    if (id is! String || id.isEmpty) {
      throw const FormatException('nested_resource requires resource_id');
    }
    return ChatNestedResourceIndicator(
      resourceType: _chatResourceTypeFromWire(type),
      resourceId: id,
    );
  }

  Map<String, dynamic> toJson() => {
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
  };

  @override
  List<Object?> get props => [resourceType, resourceId];
}

class ChatResourceProfileLivePayload extends ChatResourceProjectionPayload {
  final String username;
  final String? avatarUrl;
  final String? storeName;
  final bool isSeller;
  final String lifecycle;

  const ChatResourceProfileLivePayload({
    required this.username,
    this.avatarUrl,
    this.storeName,
    required this.isSeller,
    required this.lifecycle,
  });

  factory ChatResourceProfileLivePayload.fromJson(Map<String, dynamic> json) {
    final username = json['username'];
    final isSeller = json['is_seller'];
    final lifecycle = json['lifecycle'];
    if (username is! String || username.isEmpty) {
      throw const FormatException('profile payload requires username');
    }
    if (isSeller is! bool) {
      throw const FormatException('profile payload requires is_seller');
    }
    if (lifecycle is! String || lifecycle.isEmpty) {
      throw const FormatException('profile payload requires lifecycle');
    }
    return ChatResourceProfileLivePayload(
      username: username,
      avatarUrl: json['avatar_url'] as String?,
      storeName: json['store_name'] as String?,
      isSeller: isSeller,
      lifecycle: lifecycle,
    );
  }

  @override
  ChatResourceType get resourceType => ChatResourceType.profile;

  @override
  Map<String, dynamic> toJson() => {
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (storeName != null) 'store_name': storeName,
    'is_seller': isSeller,
    'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [
    username,
    avatarUrl,
    storeName,
    isSeller,
    lifecycle,
  ];
}

class ChatResourceContentLivePayload extends ChatResourceProjectionPayload {
  final String? caption;
  final List<ChatContentMediaRef> media;
  final String lifecycle;
  final String createdAt;
  final ChatResourceUserCard author;
  final ChatNestedResourceIndicator? nestedResource;

  const ChatResourceContentLivePayload({
    required this.caption,
    required this.media,
    required this.lifecycle,
    required this.createdAt,
    required this.author,
    this.nestedResource,
  });

  factory ChatResourceContentLivePayload.fromJson(Map<String, dynamic> json) {
    final lifecycle = json['lifecycle'];
    final createdAt = json['created_at'];
    final author = json['author'];
    if (lifecycle is! String || lifecycle.isEmpty) {
      throw const FormatException('content payload requires lifecycle');
    }
    if (createdAt is! String || createdAt.isEmpty) {
      throw const FormatException('content payload requires created_at');
    }
    if (author is! Map<String, dynamic>) {
      throw const FormatException('content payload requires author');
    }
    final mediaRaw = json['media'];
    if (mediaRaw is! List) {
      throw const FormatException('content payload requires media');
    }
    final media = <ChatContentMediaRef>[];
    for (final item in mediaRaw) {
      if (item is! Map<String, dynamic>) {
        throw const FormatException('content payload requires media items');
      }
      media.add(ChatContentMediaRef.fromJson(item));
    }
    final nestedResourceRaw = json['nested_resource'];
    if (nestedResourceRaw != null &&
        nestedResourceRaw is! Map<String, dynamic>) {
      throw const FormatException('content payload requires nested_resource');
    }
    return ChatResourceContentLivePayload(
      caption: json['caption'] as String?,
      media: media,
      lifecycle: lifecycle,
      createdAt: createdAt,
      author: ChatResourceUserCard.fromJson(author),
      nestedResource: nestedResourceRaw is Map<String, dynamic>
          ? ChatNestedResourceIndicator.fromJson(nestedResourceRaw)
          : null,
    );
  }

  @override
  ChatResourceType get resourceType => ChatResourceType.content;

  @override
  Map<String, dynamic> toJson() => {
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

class ChatResourceForSalePrice extends Equatable {
  final int amount;
  final String currency;

  const ChatResourceForSalePrice({
    required this.amount,
    required this.currency,
  });

  factory ChatResourceForSalePrice.fromJson(Map<String, dynamic> json) {
    final amount = json['amount'];
    final currency = json['currency'];
    if (amount is! int && amount is! num) {
      throw const FormatException('price requires amount');
    }
    if (currency is! String || currency.isEmpty) {
      throw const FormatException('price requires currency');
    }
    return ChatResourceForSalePrice(
      amount: (amount as num).toInt(),
      currency: currency,
    );
  }

  Map<String, dynamic> toJson() => {'amount': amount, 'currency': currency};

  @override
  List<Object?> get props => [amount, currency];
}

class ChatResourceForSaleLivePayload extends ChatResourceProjectionPayload {
  final String title;
  final String? imageUrl;
  final ChatResourceForSalePrice price;
  final String status;
  final ChatResourceSellerCard seller;
  final int quantityAvailable;

  const ChatResourceForSaleLivePayload({
    required this.title,
    this.imageUrl,
    required this.price,
    required this.status,
    required this.seller,
    required this.quantityAvailable,
  });

  factory ChatResourceForSaleLivePayload.fromJson(Map<String, dynamic> json) {
    final title = json['title'];
    final price = json['price'];
    final status = json['status'];
    final seller = json['seller'];
    final quantity = json['quantity_available'];
    if (title is! String || title.isEmpty) {
      throw const FormatException('for_sale requires title');
    }
    if (price is! Map<String, dynamic>) {
      throw const FormatException('for_sale requires price');
    }
    if (status is! String || status.isEmpty) {
      throw const FormatException('for_sale requires status');
    }
    if (seller is! Map<String, dynamic>) {
      throw const FormatException('for_sale requires seller');
    }
    if (quantity is! int && quantity is! num) {
      throw const FormatException('for_sale requires quantity_available');
    }
    return ChatResourceForSaleLivePayload(
      title: title,
      imageUrl: json['image_url'] as String?,
      price: ChatResourceForSalePrice.fromJson(price),
      status: status,
      seller: ChatResourceSellerCard.fromJson(seller),
      quantityAvailable: (quantity as num).toInt(),
    );
  }

  @override
  ChatResourceType get resourceType => ChatResourceType.forSale;

  @override
  Map<String, dynamic> toJson() => {
    'title': title,
    if (imageUrl != null) 'image_url': imageUrl,
    'price': price.toJson(),
    'status': status,
    'seller': seller.toJson(),
    'quantity_available': quantityAvailable,
  };

  @override
  List<Object?> get props => [
    title,
    imageUrl,
    price,
    status,
    seller,
    quantityAvailable,
  ];
}

class ChatResourceAuctionLivePayload extends ChatResourceProjectionPayload {
  final String title;
  final String? thumbnailUrl;
  final int? currentBid;
  final int? buyNowPrice;
  final String endAt;
  final String? lifecycle;
  final ChatResourceSellerCard? seller;

  const ChatResourceAuctionLivePayload({
    required this.title,
    this.thumbnailUrl,
    this.currentBid,
    this.buyNowPrice,
    required this.endAt,
    this.lifecycle,
    this.seller,
  });

  factory ChatResourceAuctionLivePayload.fromJson(Map<String, dynamic> json) {
    final title = json['title'];
    final endAt = json['end_at'];
    final lifecycle = json['lifecycle'];
    final seller = json['seller'];
    final currentBidRaw = json['current_bid'];
    final buyNowPriceRaw = json['buy_now_price'];
    if (title is! String || title.isEmpty) {
      throw const FormatException('auction requires title');
    }
    if (endAt is! String || endAt.isEmpty) {
      throw const FormatException('auction requires end_at');
    }
    if (lifecycle is! String || lifecycle.isEmpty) {
      throw const FormatException('auction requires lifecycle');
    }
    if (seller is! Map<String, dynamic>) {
      throw const FormatException('auction requires seller');
    }
    if (currentBidRaw != null && currentBidRaw is! num) {
      throw const FormatException('auction requires numeric current_bid');
    }
    if (buyNowPriceRaw != null && buyNowPriceRaw is! num) {
      throw const FormatException('auction requires numeric buy_now_price');
    }
    return ChatResourceAuctionLivePayload(
      title: title,
      thumbnailUrl: json['thumbnail_url'] as String?,
      currentBid: (currentBidRaw as num?)?.toInt(),
      buyNowPrice: (buyNowPriceRaw as num?)?.toInt(),
      endAt: endAt,
      lifecycle: lifecycle,
      seller: ChatResourceSellerCard.fromJson(seller),
    );
  }

  @override
  ChatResourceType get resourceType => ChatResourceType.auction;

  @override
  Map<String, dynamic> toJson() => {
    'title': title,
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
    if (currentBid != null) 'current_bid': currentBid,
    if (buyNowPrice != null) 'buy_now_price': buyNowPrice,
    'end_at': endAt,
    if (lifecycle != null) 'lifecycle': lifecycle,
    if (seller != null) 'seller': seller!.toJson(),
  };

  @override
  List<Object?> get props => [
    title,
    thumbnailUrl,
    currentBid,
    buyNowPrice,
    endAt,
    lifecycle,
    seller,
  ];
}

sealed class ChatResourceProjection extends Equatable {
  final ChatResourceProjectionState state;
  final ChatResourceType resourceType;
  final ChatResourceViewerCapabilities viewerCapabilities;

  const ChatResourceProjection({
    required this.state,
    required this.resourceType,
    required this.viewerCapabilities,
  });

  String? get resourceId;
  String? get canonicalUrl;
  ChatCommerceActionCapabilities? get commerceActions;
  ChatResourceProjectionPayload? get payload;

  bool get isLive => state == ChatResourceProjectionState.live;
  bool get isTombstone => state == ChatResourceProjectionState.tombstone;

  String get canonicalDisplayLabel {
    switch (state) {
      case ChatResourceProjectionState.live:
        return resourceType.displayLabel;
      case ChatResourceProjectionState.tombstone:
        return '${resourceType.displayLabel} tidak tersedia';
    }
  }

  String get compactPreviewText {
    if (isTombstone) {
      return '${resourceType.displayLabel} tidak tersedia';
    }
    final payload = this.payload;
    switch (payload) {
      case ChatResourceProfileLivePayload():
        return '@${payload.username}';
      case ChatResourceContentLivePayload():
        return (payload.caption != null && payload.caption!.trim().isNotEmpty)
            ? payload.caption!.trim()
            : 'Konten';
      case ChatResourceForSaleLivePayload():
        return _formatSummary(
          payload.title,
          _formatMoney(payload.price.amount, payload.price.currency),
        );
      case ChatResourceAuctionLivePayload():
        final price = payload.currentBid ?? payload.buyNowPrice;
        return _formatSummary(
          payload.title,
          price != null ? _formatMoney(price, 'IDR') : null,
        );
      case null:
        return resourceType.displayLabel;
      default:
        return resourceType.displayLabel;
    }
  }

  String get titleText {
    if (isTombstone) {
      return '${resourceType.displayLabel} tidak tersedia';
    }
    final payload = this.payload;
    switch (payload) {
      case ChatResourceProfileLivePayload():
        return '@${payload.username}';
      case ChatResourceContentLivePayload():
        return (payload.caption != null && payload.caption!.trim().isNotEmpty)
            ? payload.caption!.trim()
            : 'Konten';
      case ChatResourceForSaleLivePayload():
        return payload.title;
      case ChatResourceAuctionLivePayload():
        return payload.title;
      case null:
        return resourceType.displayLabel;
      default:
        return resourceType.displayLabel;
    }
  }

  String? get valueText {
    if (isTombstone) {
      return 'Tidak dapat ditampilkan';
    }
    final payload = this.payload;
    switch (payload) {
      case ChatResourceProfileLivePayload():
        return payload.storeName?.trim().isNotEmpty == true
            ? payload.storeName!.trim()
            : (payload.isSeller ? 'Penjual' : 'Profil');
      case ChatResourceContentLivePayload():
        return '@${payload.author.username}';
      case ChatResourceForSaleLivePayload():
        return _formatMoney(payload.price.amount, payload.price.currency);
      case ChatResourceAuctionLivePayload():
        final price = payload.currentBid ?? payload.buyNowPrice;
        if (price == null) return null;
        return _formatMoney(price, 'IDR');
      default:
        return null;
    }
  }

  Map<String, dynamic> toJson() {
    final out = <String, dynamic>{
      'state': state.wireValue,
      'resource_type': resourceType.wireValue,
      'viewer_capabilities': viewerCapabilities.toJson(),
    };
    if (isLive) {
      out['resource_id'] = resourceId;
      out['canonical_url'] = canonicalUrl;
      if (commerceActions != null) {
        out['commerce_actions'] = commerceActions!.toJson();
      }
      final payload = this.payload;
      if (payload != null) {
        out[payload.resourceType.wireValue] = payload.toJson();
      }
    }
    return out;
  }

  static ChatResourceProjection fromJson(Map<String, dynamic> json) {
    final stateRaw = json['state'];
    final typeRaw = json['resource_type'];
    final viewerRaw = json['viewer_capabilities'];
    if (stateRaw is! String || stateRaw.isEmpty) {
      throw const FormatException('resource_projection requires state');
    }
    if (typeRaw is! String || typeRaw.isEmpty) {
      throw const FormatException('resource_projection requires resource_type');
    }
    if (viewerRaw is! Map<String, dynamic>) {
      throw const FormatException(
        'resource_projection requires viewer_capabilities',
      );
    }

    final state = _chatResourceProjectionStateFromWire(stateRaw);
    final resourceType = _chatResourceTypeFromWire(typeRaw);
    final viewerCapabilities = ChatResourceViewerCapabilities.fromJson(
      viewerRaw,
      state: state,
    );

    switch (state) {
      case ChatResourceProjectionState.live:
        return _parseLive(json, resourceType, viewerCapabilities);
      case ChatResourceProjectionState.tombstone:
        return _parseTombstone(json, resourceType, viewerCapabilities);
    }
  }

  static ChatResourceProjection _parseLive(
    Map<String, dynamic> json,
    ChatResourceType resourceType,
    ChatResourceViewerCapabilities viewerCapabilities,
  ) {
    final resourceId = _readRequiredString(json, 'resource_id');
    final canonicalUrl = _readRequiredString(json, 'canonical_url');
    final commerceActionsRaw = json['commerce_actions'];

    ChatCommerceActionCapabilities? commerceActions;
    if (resourceType == ChatResourceType.forSale ||
        resourceType == ChatResourceType.auction) {
      if (commerceActionsRaw is! Map<String, dynamic>) {
        throw FormatException(
          'LIVE ${resourceType.wireValue} projection requires commerce_actions',
        );
      }
      commerceActions = ChatCommerceActionCapabilities.fromJson(
        commerceActionsRaw,
      );
      if (viewerCapabilities.canInteract && !commerceActions.hasAnyAction) {
        throw FormatException(
          'can_interact=true requires at least one actionable commerce flag',
        );
      }
    } else if (commerceActionsRaw != null) {
      throw FormatException(
        'LIVE ${resourceType.wireValue} projection requires no commerce_actions',
      );
    }

    if (resourceType == ChatResourceType.profile) {
      _rejectUnexpectedPayloadKeys(json, const ['profile']);
      final payloadJson = json['profile'];
      if (payloadJson is! Map<String, dynamic>) {
        throw const FormatException('LIVE profile requires profile payload');
      }
      return ChatLiveResourceProjection(
        state: ChatResourceProjectionState.live,
        resourceType: resourceType,
        resourceId: resourceId,
        canonicalUrl: canonicalUrl,
        viewerCapabilities: viewerCapabilities,
        commerceActions: null,
        payload: ChatResourceProfileLivePayload.fromJson(payloadJson),
      );
    }

    if (resourceType == ChatResourceType.content) {
      _rejectUnexpectedPayloadKeys(json, const ['content']);
      final payloadJson = json['content'];
      if (payloadJson is! Map<String, dynamic>) {
        throw const FormatException('LIVE content requires content payload');
      }
      return ChatLiveResourceProjection(
        state: ChatResourceProjectionState.live,
        resourceType: resourceType,
        resourceId: resourceId,
        canonicalUrl: canonicalUrl,
        viewerCapabilities: viewerCapabilities,
        commerceActions: null,
        payload: ChatResourceContentLivePayload.fromJson(payloadJson),
      );
    }

    if (resourceType == ChatResourceType.forSale) {
      _rejectUnexpectedPayloadKeys(json, const ['for_sale']);
      final payloadJson = json['for_sale'];
      if (payloadJson is! Map<String, dynamic>) {
        throw const FormatException('LIVE for_sale requires for_sale payload');
      }
      return ChatLiveResourceProjection(
        state: ChatResourceProjectionState.live,
        resourceType: resourceType,
        resourceId: resourceId,
        canonicalUrl: canonicalUrl,
        viewerCapabilities: viewerCapabilities,
        commerceActions: commerceActions,
        payload: ChatResourceForSaleLivePayload.fromJson(payloadJson),
      );
    }

    _rejectUnexpectedPayloadKeys(json, const ['auction']);
    final payloadJson = json['auction'];
    if (payloadJson is! Map<String, dynamic>) {
      throw const FormatException('LIVE auction requires auction payload');
    }
    return ChatLiveResourceProjection(
      state: ChatResourceProjectionState.live,
      resourceType: resourceType,
      resourceId: resourceId,
      canonicalUrl: canonicalUrl,
      viewerCapabilities: viewerCapabilities,
      commerceActions: commerceActions,
      payload: ChatResourceAuctionLivePayload.fromJson(payloadJson),
    );
  }

  static ChatResourceProjection _parseTombstone(
    Map<String, dynamic> json,
    ChatResourceType resourceType,
    ChatResourceViewerCapabilities viewerCapabilities,
  ) {
    for (final key in const [
      'resource_id',
      'canonical_url',
      'commerce_actions',
      'profile',
      'content',
      'for_sale',
      'auction',
    ]) {
      if (json.containsKey(key)) {
        throw FormatException(
          'TOMBSTONE resource_projection must not contain $key',
        );
      }
    }
    return ChatTombstoneResourceProjection(
      state: ChatResourceProjectionState.tombstone,
      resourceType: resourceType,
      viewerCapabilities: viewerCapabilities,
    );
  }
}

class ChatLiveResourceProjection extends ChatResourceProjection {
  @override
  final String resourceId;
  @override
  final String canonicalUrl;
  @override
  final ChatCommerceActionCapabilities? commerceActions;
  @override
  final ChatResourceProjectionPayload payload;

  const ChatLiveResourceProjection({
    required super.state,
    required super.resourceType,
    required super.viewerCapabilities,
    required this.resourceId,
    required this.canonicalUrl,
    required this.commerceActions,
    required this.payload,
  });

  @override
  List<Object?> get props => [
    state,
    resourceType,
    viewerCapabilities,
    resourceId,
    canonicalUrl,
    commerceActions,
    payload,
  ];
}

class ChatTombstoneResourceProjection extends ChatResourceProjection {
  @override
  String? get resourceId => null;

  @override
  String? get canonicalUrl => null;

  @override
  ChatCommerceActionCapabilities? get commerceActions => null;

  @override
  ChatResourceProjectionPayload? get payload => null;

  const ChatTombstoneResourceProjection({
    required super.state,
    required super.resourceType,
    required super.viewerCapabilities,
  });

  @override
  List<Object?> get props => [state, resourceType, viewerCapabilities];
}

void _rejectUnexpectedPayloadKeys(
  Map<String, dynamic> json,
  List<String> allowedKeys,
) {
  for (final key in const ['profile', 'content', 'for_sale', 'auction']) {
    if (!allowedKeys.contains(key) && json.containsKey(key)) {
      throw FormatException(
        'unexpected payload key in resource_projection: $key',
      );
    }
  }
  if (!allowedKeys.contains('content') && json.containsKey('nested_resource')) {
    throw const FormatException(
      'unexpected payload key in resource_projection: nested_resource',
    );
  }
}

String _readRequiredString(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is String && value.isNotEmpty) {
    return value;
  }
  throw FormatException('resource_projection requires $key');
}

String _formatSummary(String primary, String? secondary) {
  final trimmedPrimary = primary.trim();
  final trimmedSecondary = secondary?.trim();
  if (trimmedSecondary == null || trimmedSecondary.isEmpty) {
    return trimmedPrimary;
  }
  if (trimmedPrimary.isEmpty) {
    return trimmedSecondary;
  }
  return '$trimmedPrimary - $trimmedSecondary';
}

String _formatMoney(int amount, String currency) {
  final digits = amount.toString();
  final buffer = StringBuffer();
  var count = 0;
  for (var i = digits.length - 1; i >= 0; i--) {
    buffer.write(digits[i]);
    count += 1;
    if (count % 3 == 0 && i != 0) {
      buffer.write('.');
    }
  }
  final formattedAmount = buffer.toString().split('').reversed.join();
  return currency == 'IDR'
      ? 'Rp $formattedAmount'
      : '$currency $formattedAmount';
}
