import 'package:equatable/equatable.dart';

/// CANONICAL RESOURCE PROJECTION (mobile mirror of the backend authority).
///
/// One envelope for every surface: content detail, feed, search, comment and
/// chat message attachment all parse THIS type. The former per-surface twins
/// (`chat_resource_projection.dart`, `content_resource_projection.dart`) are
/// dead — two parsers for one wire is dual authority, and whoever disagreed
/// with the backend won the argument on the client.
///
/// Backend authority: `commerceshared.ResourceProjection`
/// (`backend/internal/commerce/shared/resource_projection_envelope.go`).
///
/// Wire contract (strict, state-specific):
///
///   `LIVE:      {state, resource_type, resource_id, canonical_url,
///               viewer_capabilities, payload}`
///   `TOMBSTONE: {state, resource_type, resource_id, viewer_capabilities}`
///
/// Locked decisions this parser enforces:
///   - resource_id is ALWAYS present, in both states — identity survives death
///     for dedup/audit; exposing it is not a lifecycle claim.
///   - canonical_url is LIVE-only.
///   - viewer_capabilities is ENVELOPE-level; the former commerce_actions
///     viewer capability matrix is purged. Product attributes (e.g. for_sale
///     `negotiation_enabled`) live on the payload.
///   - price is a money object {amount, currency}; the scalar price is dead.
///   - media is []ResourceMediaRef plus optional thumbnail_url; the singular
///     image_url is dead.
///   - the payload is present iff LIVE and it must match resource_type.
///
/// Display policy does NOT live here: the envelope carries price on LIVE for
/// every surface (owner contract). Chat renders availability, discovery
/// renders the money — that split belongs to the cards.
/// Lifecycle state of a canonical projection.
enum ResourceProjectionState { live, tombstone }

extension ResourceProjectionStateX on ResourceProjectionState {
  String get wireValue {
    switch (this) {
      case ResourceProjectionState.live:
        return 'LIVE';
      case ResourceProjectionState.tombstone:
        return 'TOMBSTONE';
    }
  }

  static ResourceProjectionState fromWire(String value) {
    switch (value) {
      case 'LIVE':
        return ResourceProjectionState.live;
      case 'TOMBSTONE':
        return ResourceProjectionState.tombstone;
      default:
        throw FormatException('invalid resource projection state: $value');
    }
  }
}

/// Canonical resource vocabulary {profile, content, for_sale, auction}.
enum ResourceProjectionType { profile, content, fixedPriceSale, auction }

extension ResourceProjectionTypeX on ResourceProjectionType {
  String get wireValue {
    switch (this) {
      case ResourceProjectionType.profile:
        return 'profile';
      case ResourceProjectionType.content:
        return 'content';
      case ResourceProjectionType.fixedPriceSale:
        return 'for_sale';
      case ResourceProjectionType.auction:
        return 'auction';
    }
  }

  String get displayLabel {
    switch (this) {
      case ResourceProjectionType.profile:
        return 'Profil';
      case ResourceProjectionType.content:
        return 'Konten';
      case ResourceProjectionType.fixedPriceSale:
        return 'For Sale';
      case ResourceProjectionType.auction:
        return 'Lelang';
    }
  }

  static ResourceProjectionType fromWire(String value) {
    switch (value) {
      case 'profile':
        return ResourceProjectionType.profile;
      case 'content':
        return ResourceProjectionType.content;
      case 'for_sale':
        return ResourceProjectionType.fixedPriceSale;
      case 'auction':
        return ResourceProjectionType.auction;
      default:
        throw FormatException('invalid resource type: $value');
    }
  }
}

/// Render authority for a transported media reference.
enum ResourceMediaKind { image, video }

/// Envelope-level viewer truth (present in BOTH states; in TOMBSTONE it
/// carries the honest blocked_by_tombstone explanation).
class ResourceViewerCapabilities extends Equatable {
  final bool canView;
  final bool canInteract;

  /// Canonical Commerce OWNERSHIP capability for the viewer — true iff the
  /// authenticated viewer is the product's seller (`Role == "owner"`), as
  /// evaluated by the Commerce authority. It is the ONLY viewer-scoped signal a
  /// conversation surface may use to offer an owner-only product action; a
  /// surface must never derive ownership from a message/bubble sender.
  ///
  /// Absent on the wire (legacy) defaults to false (fail-closed).
  final bool canManage;

  final bool blockedByTombstone;

  const ResourceViewerCapabilities({
    required this.canView,
    required this.canInteract,
    this.canManage = false,
    required this.blockedByTombstone,
  });

  const ResourceViewerCapabilities.live({
    required this.canInteract,
    this.canManage = false,
  }) : canView = true,
       blockedByTombstone = false;

  const ResourceViewerCapabilities.tombstone()
    : canView = false,
      canInteract = false,
      canManage = false,
      blockedByTombstone = true;

  factory ResourceViewerCapabilities.fromJson(
    Map<String, dynamic> json, {
    required ResourceProjectionState state,
  }) {
    final canView = json['can_view'];
    final canInteract = json['can_interact'];
    final blocked = json['blocked_by_tombstone'];
    if (canView is! bool || canInteract is! bool || blocked is! bool) {
      throw const FormatException('viewer_capabilities must contain booleans');
    }
    final canManage = json['can_manage'] as bool? ?? false;
    final caps = ResourceViewerCapabilities(
      canView: canView,
      canInteract: canInteract,
      canManage: canManage,
      blockedByTombstone: blocked,
    );
    caps.validate(state: state);
    return caps;
  }

  void validate({required ResourceProjectionState state}) {
    switch (state) {
      case ResourceProjectionState.live:
        if (!canView) {
          throw const FormatException('LIVE projection requires can_view=true');
        }
        if (blockedByTombstone) {
          throw const FormatException(
            'LIVE projection requires blocked_by_tombstone=false',
          );
        }
      case ResourceProjectionState.tombstone:
        if (canView) {
          throw const FormatException('TOMBSTONE requires can_view=false');
        }
        if (canInteract) {
          throw const FormatException('TOMBSTONE requires can_interact=false');
        }
        if (canManage) {
          throw const FormatException('TOMBSTONE requires can_manage=false');
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
    'can_manage': canManage,
    'blocked_by_tombstone': blockedByTombstone,
  };

  @override
  List<Object?> get props => [
    canView,
    canInteract,
    canManage,
    blockedByTombstone,
  ];
}

/// The single money formatter of the canonical envelope.
///
/// Every commerce surface in the app groups thousands with '.', e.g.
/// `Rp 1.250.000`. Cards must render money through the payload getters below —
/// a second formatter in a card is a second authority for the same number.
String formatGroupedAmount(int amount) {
  final digits = amount.abs().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) {
      buffer.write('.');
    }
    buffer.write(digits[i]);
  }
  return amount < 0 ? '-$buffer' : buffer.toString();
}

/// Canonical live money envelope {amount, currency}.
class LivePrice extends Equatable {
  final int amount;
  final String currency;

  const LivePrice({required this.amount, required this.currency});

  factory LivePrice.fromJson(Map<String, dynamic> json) {
    final amount = json['amount'];
    final currency = json['currency'];
    if (amount is! num) {
      throw const FormatException('price requires amount');
    }
    if (currency is! String || currency.isEmpty) {
      throw const FormatException('price requires currency');
    }
    return LivePrice(amount: amount.toInt(), currency: currency);
  }

  /// Canonical display string: `Rp 1.250.000` for IDR, `<CUR> 1.250.000`
  /// otherwise — one string for every surface.
  String get formatted => currency == 'IDR'
      ? 'Rp ${formatGroupedAmount(amount)}'
      : '$currency ${formatGroupedAmount(amount)}';

  Map<String, dynamic> toJson() => {'amount': amount, 'currency': currency};

  @override
  List<Object?> get props => [amount, currency];
}

class ResourceUserCard extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;
  final String? lifecycle;

  const ResourceUserCard({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ResourceUserCard.fromJson(Map<String, dynamic> json) {
    final id = json['id'];
    final username = json['username'];
    if (id is! String || id.isEmpty) {
      throw const FormatException('user card requires id');
    }
    if (username is! String || username.isEmpty) {
      throw const FormatException('user card requires username');
    }
    return ResourceUserCard(
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

/// Seller identity is always the tier-gated public card; the chat-era flat
/// seller died with the chat envelope.
class ResourceSellerCard extends Equatable {
  final ResourceUserCard user;
  final String? farmName;
  final String? avatarUrl;
  final String? lifecycle;

  const ResourceSellerCard({
    required this.user,
    this.farmName,
    this.avatarUrl,
    this.lifecycle,
  });

  factory ResourceSellerCard.fromJson(Map<String, dynamic> json) {
    final user = json['user'];
    if (user is! Map<String, dynamic>) {
      throw const FormatException('seller card requires user');
    }
    return ResourceSellerCard(
      user: ResourceUserCard.fromJson(user),
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

class ResourceMediaRef extends Equatable {
  final String url;
  final String? kind;
  final int? width;
  final int? height;

  const ResourceMediaRef({
    required this.url,
    this.kind,
    this.width,
    this.height,
  });

  factory ResourceMediaRef.fromJson(Map<String, dynamic> json) {
    final url = json['url'];
    if (url is! String || url.isEmpty) {
      throw const FormatException('media item requires url');
    }
    return ResourceMediaRef(
      url: url,
      kind: json['kind'] as String?,
      width: (json['width'] as num?)?.toInt(),
      height: (json['height'] as num?)?.toInt(),
    );
  }

  /// [kind] is the transported persisted `content_media.media_type`; a URL
  /// extension is never sniffed.
  ResourceMediaKind get mediaKind =>
      kind == 'video' ? ResourceMediaKind.video : ResourceMediaKind.image;

  Map<String, dynamic> toJson() => {
    'url': url,
    if (kind != null) 'kind': kind,
    if (width != null) 'width': width,
    if (height != null) 'height': height,
  };

  @override
  List<Object?> get props => [url, kind, width, height];
}

/// Depth-1 nested identity (identity only, never a second envelope).
class NestedResourceIndicator extends Equatable {
  final ResourceProjectionType resourceType;
  final String resourceId;

  const NestedResourceIndicator({
    required this.resourceType,
    required this.resourceId,
  });

  factory NestedResourceIndicator.fromJson(Map<String, dynamic> json) {
    final type = json['resource_type'];
    final id = json['resource_id'];
    if (type is! String || type.isEmpty) {
      throw const FormatException('nested_resource requires resource_type');
    }
    if (id is! String || id.isEmpty) {
      throw const FormatException('nested_resource requires resource_id');
    }
    return NestedResourceIndicator(
      resourceType: ResourceProjectionTypeX.fromWire(type),
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

sealed class ResourceProjectionPayload extends Equatable {
  const ResourceProjectionPayload();

  ResourceProjectionType get resourceType;

  Map<String, dynamic> toJson();
}

/// Media on the wire may be `null` when the resolver had nothing to project;
/// absence of media is not a malformed envelope.
List<ResourceMediaRef> _parseMedia(Object? raw, String owner) {
  if (raw == null) {
    return const <ResourceMediaRef>[];
  }
  if (raw is! List) {
    throw FormatException('$owner requires media');
  }
  final media = <ResourceMediaRef>[];
  for (final item in raw) {
    if (item is! Map<String, dynamic>) {
      throw FormatException('$owner requires media items');
    }
    media.add(ResourceMediaRef.fromJson(item));
  }
  return media;
}

class ProfileLivePayload extends ResourceProjectionPayload {
  final String username;
  final String? avatarUrl;
  final String? storeName;
  final bool isSeller;
  final String lifecycle;

  const ProfileLivePayload({
    required this.username,
    this.avatarUrl,
    this.storeName,
    this.isSeller = false,
    required this.lifecycle,
  });

  factory ProfileLivePayload.fromJson(Map<String, dynamic> json) {
    final username = json['username'];
    final lifecycle = json['lifecycle'];
    final isSeller = json['is_seller'];
    if (username is! String || username.isEmpty) {
      throw const FormatException('profile payload requires username');
    }
    if (lifecycle is! String || lifecycle.isEmpty) {
      throw const FormatException('profile payload requires lifecycle');
    }
    if (isSeller != null && isSeller is! bool) {
      throw const FormatException('profile payload requires is_seller');
    }
    return ProfileLivePayload(
      username: username,
      avatarUrl: json['avatar_url'] as String?,
      storeName: json['store_name'] as String?,
      isSeller: isSeller == true,
      lifecycle: lifecycle,
    );
  }

  @override
  ResourceProjectionType get resourceType => ResourceProjectionType.profile;

  @override
  Map<String, dynamic> toJson() => {
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (storeName != null) 'store_name': storeName,
    if (isSeller) 'is_seller': true,
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

class ContentLivePayload extends ResourceProjectionPayload {
  final String? caption;
  final List<ResourceMediaRef> media;
  final String lifecycle;
  final String createdAt;
  final ResourceUserCard author;
  final NestedResourceIndicator? nestedResource;

  const ContentLivePayload({
    required this.caption,
    required this.media,
    required this.lifecycle,
    required this.createdAt,
    required this.author,
    this.nestedResource,
  });

  factory ContentLivePayload.fromJson(Map<String, dynamic> json) {
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
    final nestedRaw = json['nested_resource'];
    if (nestedRaw != null && nestedRaw is! Map<String, dynamic>) {
      throw const FormatException('content payload requires nested_resource');
    }
    return ContentLivePayload(
      caption: json['caption'] as String?,
      media: _parseMedia(json['media'], 'content payload'),
      lifecycle: lifecycle,
      createdAt: createdAt,
      author: ResourceUserCard.fromJson(author),
      nestedResource: nestedRaw is Map<String, dynamic>
          ? NestedResourceIndicator.fromJson(nestedRaw)
          : null,
    );
  }

  @override
  ResourceProjectionType get resourceType => ResourceProjectionType.content;

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

class ForSaleLivePayload extends ResourceProjectionPayload {
  final String title;
  final List<ResourceMediaRef> media;
  final String? thumbnailUrl;
  final LivePrice price;
  final String status;
  final int quantityAvailable;

  /// Canonical PRODUCT-LEVEL negotiation attribute: the listing is negotiable,
  /// active and in stock. Viewer-independent read-through — never derived from
  /// auth, seller trust, role or a capability evaluation. Rendered as the
  /// informational "Nego" attribute. Absent on the wire defaults to false.
  final bool negotiationEnabled;
  final ResourceSellerCard seller;

  const ForSaleLivePayload({
    required this.title,
    required this.media,
    this.thumbnailUrl,
    required this.price,
    required this.status,
    required this.quantityAvailable,
    this.negotiationEnabled = false,
    required this.seller,
  });

  factory ForSaleLivePayload.fromJson(Map<String, dynamic> json) {
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
    if (quantity is! num) {
      throw const FormatException('for_sale requires quantity_available');
    }
    return ForSaleLivePayload(
      title: title,
      media: _parseMedia(json['media'], 'for_sale'),
      thumbnailUrl: json['thumbnail_url'] as String?,
      price: LivePrice.fromJson(price),
      status: status,
      quantityAvailable: quantity.toInt(),
      negotiationEnabled: json['negotiation_enabled'] as bool? ?? false,
      seller: ResourceSellerCard.fromJson(seller),
    );
  }

  /// Canonical money string for every surface that renders this payload.
  String get formattedPrice => price.formatted;

  /// Canonical preview image: the projected thumbnail first, then the first
  /// media reference. No URL is ever invented.
  String? get primaryImageUrl {
    final thumbnail = thumbnailUrl?.trim();
    if (thumbnail != null && thumbnail.isNotEmpty) {
      return thumbnail;
    }
    if (media.isEmpty) {
      return null;
    }
    final url = media.first.url.trim();
    return url.isEmpty ? null : url;
  }

  @override
  ResourceProjectionType get resourceType => ResourceProjectionType.fixedPriceSale;

  @override
  Map<String, dynamic> toJson() => {
    'title': title,
    'media': media.map((item) => item.toJson()).toList(growable: false),
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
    'price': price.toJson(),
    'status': status,
    'quantity_available': quantityAvailable,
    'negotiation_enabled': negotiationEnabled,
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
    negotiationEnabled,
    seller,
  ];
}

class AuctionLivePayload extends ResourceProjectionPayload {
  final String title;
  final List<ResourceMediaRef> media;
  final String? thumbnailUrl;
  final int? currentBid;
  final int? buyNowPrice;
  final String endAt;

  /// Canonical public auction PHASE projection from Commerce
  /// (`Status.PublicPhase()`): scheduled | active | waiting_settlement |
  /// ended | cancelled. Read-through only; the conversation never derives it.
  final String lifecycle;

  /// Canonical outcome discriminator for an `ended` phase
  /// (ended+winner vs ended+no-winner), projected by Commerce. Read-through.
  final bool hasWinner;
  final ResourceSellerCard seller;

  const AuctionLivePayload({
    required this.title,
    required this.media,
    this.thumbnailUrl,
    this.currentBid,
    this.buyNowPrice,
    required this.endAt,
    required this.lifecycle,
    this.hasWinner = false,
    required this.seller,
  });

  factory AuctionLivePayload.fromJson(Map<String, dynamic> json) {
    final title = json['title'];
    final endAt = json['end_at'];
    final lifecycle = json['lifecycle'];
    final seller = json['seller'];
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
    final currentBidRaw = json['current_bid'];
    final buyNowPriceRaw = json['buy_now_price'];
    if (currentBidRaw != null && currentBidRaw is! num) {
      throw const FormatException('auction current_bid must be numeric');
    }
    if (buyNowPriceRaw != null && buyNowPriceRaw is! num) {
      throw const FormatException('auction buy_now_price must be numeric');
    }
    return AuctionLivePayload(
      title: title,
      media: _parseMedia(json['media'], 'auction'),
      thumbnailUrl: json['thumbnail_url'] as String?,
      currentBid: (currentBidRaw as num?)?.toInt(),
      buyNowPrice: (buyNowPriceRaw as num?)?.toInt(),
      endAt: endAt,
      lifecycle: lifecycle,
      hasWinner: json['has_winner'] as bool? ?? false,
      seller: ResourceSellerCard.fromJson(seller),
    );
  }

  /// Canonical money string: the current bid, falling back to buy-now. Null
  /// when the auction has neither yet (an empty auction shows no amount, it
  /// never invents 0). Auction amounts are canonical IDR ints on the wire.
  String? get formattedAmount {
    final amount = currentBid ?? buyNowPrice;
    return amount == null ? null : 'Rp ${formatGroupedAmount(amount)}';
  }

  String? get primaryImageUrl {
    final thumbnail = thumbnailUrl?.trim();
    if (thumbnail != null && thumbnail.isNotEmpty) {
      return thumbnail;
    }
    if (media.isEmpty) {
      return null;
    }
    final url = media.first.url.trim();
    return url.isEmpty ? null : url;
  }

  @override
  ResourceProjectionType get resourceType => ResourceProjectionType.auction;

  @override
  Map<String, dynamic> toJson() => {
    'title': title,
    'media': media.map((item) => item.toJson()).toList(growable: false),
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
    if (currentBid != null) 'current_bid': currentBid,
    if (buyNowPrice != null) 'buy_now_price': buyNowPrice,
    'end_at': endAt,
    'lifecycle': lifecycle,
    if (hasWinner) 'has_winner': hasWinner,
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
    hasWinner,
    seller,
  ];
}

/// The sealed canonical envelope. `resourceId` is REQUIRED in both states —
/// a tombstone that cannot be identified cannot be deduplicated or audited.
sealed class ResourceProjection extends Equatable {
  final ResourceProjectionState state;
  final ResourceProjectionType resourceType;
  final ResourceViewerCapabilities viewerCapabilities;

  const ResourceProjection({
    required this.state,
    required this.resourceType,
    required this.viewerCapabilities,
  });

  String get resourceId;

  /// LIVE-only: a URL into a dead resource is a lie.
  String? get canonicalUrl;

  ResourceProjectionPayload? get payload;

  bool get isLive => state == ResourceProjectionState.live;
  bool get isTombstone => state == ResourceProjectionState.tombstone;

  String get typeLabel => resourceType.displayLabel;

  /// Client route for the canonical identity. Identity is not a lifecycle
  /// claim: this is reachable for TOMBSTONEs too, and the screen decides what
  /// to render for a dead resource.
  String get canonicalPath {
    switch (resourceType) {
      case ResourceProjectionType.profile:
        return '/user/$resourceId';
      case ResourceProjectionType.content:
        return '/content/$resourceId';
      case ResourceProjectionType.fixedPriceSale:
        return '/for-sale/$resourceId';
      case ResourceProjectionType.auction:
        return '/auction/$resourceId';
    }
  }

  /// The canonical URL when LIVE, otherwise the derived path (never null).
  String get resolvedPath => canonicalUrl ?? canonicalPath;

  String get titleText {
    if (!isLive) {
      return '${resourceType.displayLabel} tidak tersedia';
    }
    return switch (payload) {
      ProfileLivePayload(:final username) => '@$username',
      ContentLivePayload(:final caption) =>
        (caption != null && caption.trim().isNotEmpty)
            ? caption.trim()
            : 'Konten',
      ForSaleLivePayload(:final title) => title,
      AuctionLivePayload(:final title) => title,
      null => resourceType.displayLabel,
    };
  }

  String get canonicalDisplayLabel => isLive
      ? resourceType.displayLabel
      : '${resourceType.displayLabel} tidak tersedia';

  /// Wire status of the resource (lifecycle/status vocabulary), never a
  /// translated label — display text belongs to the surface.
  String? get statusLabel {
    if (!isLive) {
      return 'TOMBSTONE';
    }
    return switch (payload) {
      ProfileLivePayload(:final lifecycle) => lifecycle,
      ContentLivePayload(:final lifecycle) => lifecycle,
      ForSaleLivePayload(:final status) => status,
      AuctionLivePayload(:final lifecycle) => lifecycle,
      null => null,
    };
  }

  String? get primaryImageUrl {
    if (!isLive) {
      return null;
    }
    return switch (payload) {
      ProfileLivePayload(:final avatarUrl) => avatarUrl,
      ContentLivePayload(:final media) =>
        media.isEmpty ? null : media.first.url,
      ForSaleLivePayload p => p.primaryImageUrl,
      AuctionLivePayload p => p.primaryImageUrl,
      null => null,
    };
  }

  String? get nestedResourceLabel {
    final p = payload;
    if (p is! ContentLivePayload) {
      return null;
    }
    return p.nestedResource?.resourceType.displayLabel;
  }

  Map<String, dynamic> toJson();

  static ResourceProjection fromJson(Map<String, dynamic> json) {
    final stateRaw = json['state'];
    final typeRaw = json['resource_type'];
    final idRaw = json['resource_id'];
    final viewerRaw = json['viewer_capabilities'];
    if (stateRaw is! String || stateRaw.isEmpty) {
      throw const FormatException('resource_projection requires state');
    }
    if (typeRaw is! String || typeRaw.isEmpty) {
      throw const FormatException('resource_projection requires resource_type');
    }
    if (idRaw is! String || idRaw.isEmpty) {
      throw const FormatException('resource_projection requires resource_id');
    }
    if (viewerRaw is! Map<String, dynamic>) {
      throw const FormatException(
        'resource_projection requires viewer_capabilities',
      );
    }

    final state = ResourceProjectionStateX.fromWire(stateRaw);
    final resourceType = ResourceProjectionTypeX.fromWire(typeRaw);
    final viewerCapabilities = ResourceViewerCapabilities.fromJson(
      viewerRaw,
      state: state,
    );

    switch (state) {
      case ResourceProjectionState.live:
        return _parseLive(json, resourceType, idRaw, viewerCapabilities);
      case ResourceProjectionState.tombstone:
        return _parseTombstone(json, resourceType, idRaw, viewerCapabilities);
    }
  }

  static ResourceProjection _parseLive(
    Map<String, dynamic> json,
    ResourceProjectionType resourceType,
    String resourceId,
    ResourceViewerCapabilities viewerCapabilities,
  ) {
    final canonicalUrl = _readRequiredString(json, 'canonical_url');

    final payloadKey = resourceType.wireValue;
    for (final key in const ['profile', 'content', 'for_sale', 'auction']) {
      if (key != payloadKey && json.containsKey(key)) {
        throw FormatException(
          'unexpected payload key in resource_projection: $key',
        );
      }
    }
    if (resourceType != ResourceProjectionType.content &&
        json.containsKey('nested_resource')) {
      throw const FormatException(
        'unexpected payload key in resource_projection: nested_resource',
      );
    }

    final payloadRaw = json[payloadKey];
    if (payloadRaw is! Map<String, dynamic>) {
      throw FormatException(
        'LIVE ${resourceType.wireValue} projection requires $payloadKey payload',
      );
    }

    final ResourceProjectionPayload payload;
    switch (resourceType) {
      case ResourceProjectionType.profile:
        payload = ProfileLivePayload.fromJson(payloadRaw);
      case ResourceProjectionType.content:
        payload = ContentLivePayload.fromJson(payloadRaw);
      case ResourceProjectionType.fixedPriceSale:
        payload = ForSaleLivePayload.fromJson(payloadRaw);
      case ResourceProjectionType.auction:
        payload = AuctionLivePayload.fromJson(payloadRaw);
    }

    return LiveResourceProjection(
      state: ResourceProjectionState.live,
      resourceType: resourceType,
      resourceId: resourceId,
      canonicalUrl: canonicalUrl,
      viewerCapabilities: viewerCapabilities,
      payload: payload,
    );
  }

  static ResourceProjection _parseTombstone(
    Map<String, dynamic> json,
    ResourceProjectionType resourceType,
    String resourceId,
    ResourceViewerCapabilities viewerCapabilities,
  ) {
    for (final key in const [
      'canonical_url',
      'profile',
      'content',
      'for_sale',
      'auction',
      'nested_resource',
    ]) {
      if (json.containsKey(key)) {
        throw FormatException(
          'TOMBSTONE resource_projection must not contain $key',
        );
      }
    }
    return TombstoneResourceProjection(
      state: ResourceProjectionState.tombstone,
      resourceType: resourceType,
      resourceId: resourceId,
      viewerCapabilities: viewerCapabilities,
    );
  }
}

class LiveResourceProjection extends ResourceProjection {
  @override
  final String resourceId;
  @override
  final String canonicalUrl;
  @override
  final ResourceProjectionPayload payload;

  const LiveResourceProjection({
    required super.state,
    required super.resourceType,
    required super.viewerCapabilities,
    required this.resourceId,
    required this.canonicalUrl,
    required this.payload,
  });

  @override
  Map<String, dynamic> toJson() => {
    'state': state.wireValue,
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
    'canonical_url': canonicalUrl,
    'viewer_capabilities': viewerCapabilities.toJson(),
    resourceType.wireValue: payload.toJson(),
  };

  @override
  List<Object?> get props => [
    state,
    resourceType,
    resourceId,
    canonicalUrl,
    viewerCapabilities,
    payload,
  ];
}

class TombstoneResourceProjection extends ResourceProjection {
  @override
  final String resourceId;

  const TombstoneResourceProjection({
    required super.state,
    required super.resourceType,
    required super.viewerCapabilities,
    required this.resourceId,
  });

  @override
  String? get canonicalUrl => null;

  @override
  ResourceProjectionPayload? get payload => null;

  @override
  Map<String, dynamic> toJson() => {
    'state': state.wireValue,
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
    'viewer_capabilities': viewerCapabilities.toJson(),
  };

  @override
  List<Object?> get props => [
    state,
    resourceType,
    resourceId,
    viewerCapabilities,
  ];
}

String _readRequiredString(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is String && value.isNotEmpty) {
    return value;
  }
  throw FormatException('resource_projection requires $key');
}

/// PRESENTATION-ONLY lifecycle label for a Commerce product reference.
///
/// This is the ONE mapping shared by the Chat and Comment product reference
/// cards. It reads the Commerce projection VERBATIM — `ForSaleLivePayload.status`
/// and `AuctionLivePayload.lifecycle` + `hasWinner` — and renders a label.
///
/// It NEVER calculates lifecycle from `end_at`, current time, `current_bid`,
/// bid count, `winnerId`, quantity, or order state, and it never reinterprets
/// an unknown value as a known Commerce state.
///
/// Returns null for non-commerce payloads (profile/content) so each surface
/// keeps its own fallback.
String? commerceLifecycleLabel(ResourceProjectionPayload? payload) {
  return switch (payload) {
    ForSaleLivePayload p => _forSaleLifecycleLabel(p.status),
    AuctionLivePayload p => _auctionLifecycleLabel(p.lifecycle, p.hasWinner),
    _ => null,
  };
}

/// For Sale display mapping (canonical `PublicLifecycle()` vocabulary).
String _forSaleLifecycleLabel(String status) {
  switch (status) {
    case 'active':
    case 'available': // tolerated synonym; canonical wire value is `active`
      return 'Tersedia';
    case 'sold':
      return 'Terjual';
    case 'unavailable':
      return 'Tidak tersedia';
    default:
      return 'Status tidak tersedia';
  }
}

/// Auction display mapping (canonical `PublicPhase()` + `has_winner`).
///
/// `has_winner` only discriminates the `ended` outcome; it never alters the
/// label of any other phase.
String _auctionLifecycleLabel(String lifecycle, bool hasWinner) {
  switch (lifecycle) {
    case 'scheduled':
      return 'Terjadwal';
    case 'active':
      return 'Berlangsung';
    case 'waiting_settlement':
      return 'Menunggu Penyelesaian';
    case 'ended':
      return hasWinner ? 'Terjual' : 'Berakhir tanpa pemenang';
    case 'cancelled':
      return 'Dibatalkan';
    default:
      return 'Status tidak tersedia';
  }
}
