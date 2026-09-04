/// ForSale API DTOs
///
/// Data Transfer Objects for Go backend integration.
/// Matches the backend forSale response format from /api/v1/for-sale
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// API CONTRACT ADAPTER BOUNDARY
/// ═══════════════════════════════════════════════════════════════════════════════
/// Backend field names use snake_case (e.g., `media_urls`, `seller_id`).
/// Domain entities use camelCase (e.g., `MediaEntity`, `sellerFarmName`).
/// This mapper is the EXPLICIT BOUNDARY between backend contract and domain model.
///
/// Field naming differences:
/// - `media_urls` (backend) → `media: List<MediaEntity>` (domain)
///
/// DO NOT change backend field names without coordinating with backend team.
/// ═══════════════════════════════════════════════════════════════════════════════
library;

import 'package:equatable/equatable.dart';

// =============================================================================
// Response DTOs
// =============================================================================

/// ForSale response from Go backend
///
/// Backend API: GET /api/v1/for-sale/:id
/// Returns a single forSale with all details
/// Typed media item from backend response.
class ForSaleMediaItemDto {
  final String id;
  final String type; // "image" or "video"
  final String url;
  final int position;
  final String? thumbnailUrl;
  final int? width;
  final int? height;
  final int? duration;

  const ForSaleMediaItemDto({
    required this.id,
    required this.type,
    required this.url,
    required this.position,
    this.thumbnailUrl,
    this.width,
    this.height,
    this.duration,
  });

  factory ForSaleMediaItemDto.fromJson(Map<String, dynamic> json) {
    return ForSaleMediaItemDto(
      id: json['id'] as String? ?? '',
      type: json['type'] as String? ?? 'image',
      url: json['url'] as String? ?? '',
      position: json['position'] as int? ?? 0,
      thumbnailUrl: json['thumbnail_url'] as String?,
      width: json['width'] as int?,
      height: json['height'] as int?,
      duration: json['duration'] as int?,
    );
  }
}

class ForSaleResponseDto extends Equatable {
  final String id;
  final String? productId;
  final String sellerId;
  final String title;
  final String description;
  final List<String> mediaUrls;
  final List<ForSaleMediaItemDto> mediaItems;
  final String? variety;
  final int? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String> certificates;
  final int price;
  final int quantity;
  final bool negotiationEnabled;
  final String visibility;
  final String status;
  final String? farmAddressId;
  final String? preparationTime;
  final String? preparationNote;
  final DateTime createdAt;
  final DateTime updatedAt;

  // ===========================================================================
  // STAGE 2 — IDENTITY PARSE-ONLY FIELDS (Phase 5)
  // ===========================================================================
  // Owner-truth identity fields parsed from backend Stage 1.
  // Receive-only plumbing: not yet wired into the entity / UI mapping.
  // Stage 3 will switch the mapper to consume these.
  // - seller_username   = account/user identity
  // - seller_farm_name  = seller/store identity (Owner Truth: farm name)
  // - seller_avatar_url = display avatar
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;

  /// E8.2 — Canonical seller user-identity lifecycle.
  ///
  /// Sourced at parse time from the wire's nested
  /// `listing.seller.user.lifecycle` slot populated by E8.1's
  /// publiccard.NewSellerCardWithUserLifecycle. Tolerant: null / missing /
  /// unknown / empty → null here (mapper converts to ContentLifecycle.active).
  ///
  /// AXIS BOUNDARY: This field carries ONLY the user-identity axis
  /// (banned/deleted user). Seller-trust/capability axis is carried on
  /// [sellerTrustLifecycle].
  final String? sellerUserLifecycle;

  /// Expired-seller visibility — seller-trust lifecycle from the wire's
  /// top-level `listing.seller.lifecycle` slot. Populated by backend
  /// coarsening of the latest `seller_subscriptions.status` row.
  /// Tolerant: null / missing / unknown / empty → null (mapper converts
  /// to ContentLifecycle.active so legacy payloads keep current render).
  final String? sellerTrustLifecycle;

  /// Seller tier — raw wire value from `listing.seller.tier`.
  /// Populated by backend GatedSellerTier when
  /// ENABLE_PUBLIC_SELLER_TIER_PROFILE is true and all lifecycle gates pass.
  /// Values: "pro", "elite". Null when gated out or flag disabled.
  final String? sellerTier;

  const ForSaleResponseDto({
    required this.id,
    this.productId,
    required this.sellerId,
    required this.title,
    required this.description,
    this.mediaUrls = const [],
    this.mediaItems = const [],
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates = const [],
    required this.price,
    required this.quantity,
    this.negotiationEnabled = false,
    required this.visibility,
    required this.status,
    this.farmAddressId,
    this.preparationTime,
    this.preparationNote,
    required this.createdAt,
    required this.updatedAt,
    // Stage 2 identity parse-only fields
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    // E8.2 seller user-axis lifecycle (nested wire slot)
    this.sellerUserLifecycle,
    // Expired-seller visibility — top-level seller-trust lifecycle.
    this.sellerTrustLifecycle,
    // Stage 2 seller tier badge.
    this.sellerTier,
  });

  factory ForSaleResponseDto.fromJson(Map<String, dynamic> json) {
    return ForSaleResponseDto(
      id: json['id'] as String,
      productId: json['product_id'] as String?,
      sellerId: json['seller_id'] as String,
      title: json['title'] as String,
      description: json['description'] as String,
      mediaUrls:
          (json['media_urls'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      mediaItems:
          (json['media'] as List<dynamic>?)
              ?.map((e) => ForSaleMediaItemDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      variety: json['variety'] as String?,
      sizeCm: json['size_cm'] as int?,
      ageMonths: json['age_months'] as int?,
      gender: json['gender'] as String?,
      breeder: json['breeder'] as String?,
      bloodline: json['bloodline'] as String?,
      certificates:
          (json['certificates'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      price: json['price'] as int? ?? 0,
      quantity: json['quantity'] as int? ?? 1,
      negotiationEnabled: json['negotiation_enabled'] as bool? ?? false,
      visibility: json['visibility'] as String? ?? 'public',
      status: json['status'] as String? ?? 'active',
      farmAddressId: json['farm_address_id'] as String?,
      preparationTime: json['preparation_time'] as String?,
      preparationNote: json['preparation_note'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
      // Stage 2 identity parse-only fields. Tolerate old payload (null) and
      // new payload. No fullName fallback — owner truth is username/farm.
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      // E8.2 — Walk the nested canonical PublicCard wire slot
      // (`listing.seller.user.lifecycle`). Pre-E8.1 payloads omit it →
      // null fall-through.
      sellerUserLifecycle: _readForSaleSellerUserLifecycle(json),
      // Expired-seller visibility — walk the top-level seller-trust slot
      // (`for_sale.seller.lifecycle`). Independent axis from user lifecycle.
      sellerTrustLifecycle: _readForSaleSellerTrustLifecycle(json),
      // Stage 2 — seller reputation tier badge from `for_sale.seller.tier`.
      sellerTier: _readForSaleSellerTier(json),
    );
  }

  @override
  List<Object?> get props => [id, sellerId, title, status];
}

/// E8.2 — Extract the embedded seller user-identity lifecycle string from
/// the forSale wire shape. Walks `for_sale.seller.user.lifecycle`; returns
/// null when any segment is absent / empty / not a string. The mapper
/// converts null into [ContentLifecycle.active] via the canonical
/// [ContentLifecycleParse.fromWire] helper.
///
/// AXIS BOUNDARY: This walker only reaches the USER axis. The top-level
/// `for_sale.seller.lifecycle` slot is doctrine-reserved (seller trust /
/// capability axis) and is NEVER read here.
String? _readForSaleSellerUserLifecycle(Map<String, dynamic> json) {
  final forSale = json['for_sale'];
  if (forSale is Map<String, dynamic>) {
    final seller = forSale['seller'];
    if (seller is Map<String, dynamic>) {
      final user = seller['user'];
      if (user is Map<String, dynamic>) {
        final lc = user['lifecycle'];
        if (lc is String && lc.isNotEmpty) return lc;
      }
    }
  }
  return null;
}

/// Expired-seller visibility — Extract the top-level seller-trust lifecycle
/// string from the forSale wire shape. Walks `for_sale.seller.lifecycle`
/// (NOT `for_sale.seller.user.lifecycle`); returns null when any segment is
/// absent / not a non-empty string. The mapper converts null into
/// [ContentLifecycle.active] via the canonical fromWire helper.
String? _readForSaleSellerTrustLifecycle(Map<String, dynamic> json) {
  final forSale = json['for_sale'];
  if (forSale is Map<String, dynamic>) {
    final seller = forSale['seller'];
    if (seller is Map<String, dynamic>) {
      final lc = seller['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
  }
  return null;
}

/// Stage 2 — Extract the seller tier string from the forSale wire shape.
/// Walks `for_sale.seller.tier`; returns null when any segment is absent /
/// not a non-empty string. Mobile renders via SellerTierBadge which hides
/// for null/basic/unknown — no additional mapping needed at DTO level.
String? _readForSaleSellerTier(Map<String, dynamic> json) {
  final forSale = json['for_sale'];
  if (forSale is Map<String, dynamic>) {
    final seller = forSale['seller'];
    if (seller is Map<String, dynamic>) {
      final tier = seller['tier'];
      if (tier is String && tier.isNotEmpty) return tier;
    }
  }
  return null;
}

/// ForSale list response from Go backend
///
/// Backend API: GET /api/v1/for-sale
/// Returns paginated list of forSales
class ForSaleListResponseDto extends Equatable {
  final List<ForSaleResponseDto> forSales;
  final int page;
  final int limit;
  final int total;

  const ForSaleListResponseDto({
    required this.forSales,
    required this.page,
    required this.limit,
    required this.total,
  });

  factory ForSaleListResponseDto.fromJson(Map<String, dynamic> json) {
    final forSalesData = json['for_sales'] as List<dynamic>? ?? [];
    return ForSaleListResponseDto(
      forSales: forSalesData
          .map((e) => ForSaleResponseDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      page: json['page'] as int? ?? 1,
      limit: json['limit'] as int? ?? 20,
      total: json['total'] as int? ?? forSalesData.length,
    );
  }

  @override
  List<Object?> get props => [page, limit, total];
}

// =============================================================================
// Request DTOs
// =============================================================================

/// Request to create a new forSale
///
/// Backend API: POST /api/v1/for-sale
class CreateForSaleRequestDto {
  final String title;
  final String description;
  final int price;
  final int quantity;
  final bool negotiationEnabled;
  final String visibility;
  final List<String> mediaUrls;
  final String? variety;
  final int? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String> certificates;
  final String? farmAddressId;
  final String? preparationTime;
  final String? preparationNote;

  const CreateForSaleRequestDto({
    required this.title,
    required this.description,
    required this.price,
    required this.quantity,
    this.negotiationEnabled = false,
    this.visibility = 'public',
    this.mediaUrls = const [],
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates = const [],
    this.farmAddressId,
    this.preparationTime,
    this.preparationNote,
  });

  Map<String, dynamic> toJson() => {
    'title': title,
    'description': description,
    'price': price,
    'quantity': quantity,
    'negotiation_enabled': negotiationEnabled,
    'visibility': visibility,
    if (mediaUrls.isNotEmpty) 'media_urls': mediaUrls,
    if (variety != null) 'variety': variety,
    if (sizeCm != null) 'size_cm': sizeCm,
    if (ageMonths != null) 'age_months': ageMonths,
    if (gender != null) 'gender': gender,
    if (breeder != null) 'breeder': breeder,
    if (bloodline != null) 'bloodline': bloodline,
    if (certificates.isNotEmpty) 'certificates': certificates,
    if (farmAddressId != null) 'farm_address_id': farmAddressId,
    if (preparationTime != null) 'preparation_time': preparationTime,
    if (preparationNote != null) 'preparation_note': preparationNote,
  };
}

/// Request to update a forSale
///
/// Backend API: PUT /api/v1/for-sale/:id
///
/// **PUBLISH FLOW:**
/// To publish a draft forSale, set status to "active". This requires:
/// - Active seller subscription (hasMarketAuthority)
/// - visibility is derived from status (active = public)
class UpdateForSaleRequestDto {
  final String? title;
  final String? description;
  final int? price;
  final int? quantity;
  final bool? negotiationEnabled;
  final String? status; // draft, active, withdrawn, sold
  final List<String>? mediaUrls;
  final String? variety;
  final int? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String>? certificates;
  final String? preparationTime;
  final String? preparationNote;

  const UpdateForSaleRequestDto({
    this.title,
    this.description,
    this.price,
    this.quantity,
    this.negotiationEnabled,
    this.status,
    this.mediaUrls,
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates,
    this.preparationTime,
    this.preparationNote,
  });

  Map<String, dynamic> toJson() => {
    if (title != null) 'title': title,
    if (description != null) 'description': description,
    if (price != null) 'price': price,
    if (quantity != null) 'quantity': quantity,
    if (negotiationEnabled != null) 'negotiation_enabled': negotiationEnabled,
    if (status != null) 'status': status,
    if (mediaUrls != null) 'media_urls': mediaUrls,
    if (variety != null) 'variety': variety,
    if (sizeCm != null) 'size_cm': sizeCm,
    if (ageMonths != null) 'age_months': ageMonths,
    if (gender != null) 'gender': gender,
    if (breeder != null) 'breeder': breeder,
    if (bloodline != null) 'bloodline': bloodline,
    if (certificates != null) 'certificates': certificates,
    if (preparationTime != null) 'preparation_time': preparationTime,
    if (preparationNote != null) 'preparation_note': preparationNote,
  };
}
