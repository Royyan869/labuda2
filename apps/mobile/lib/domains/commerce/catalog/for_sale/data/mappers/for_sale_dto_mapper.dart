/// ForSale DTO Mapper
///
/// Maps ForSale DTOs from backend API to ForSale domain entities.
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// API CONTRACT ADAPTER BOUNDARY
/// ═══════════════════════════════════════════════════════════════════════════════
/// This mapper is the EXPLICIT BOUNDARY between backend contract and domain model.
///
/// Field name translations:
/// - `media_urls` (backend: List&lt;String&gt;) → `media: List&lt;MediaEntity&gt;` (domain)
///   Backend provides simple URL strings; domain wraps them in MediaEntity for metadata.
/// - `price` (backend: int in minor units) → `price: double` (domain)
///   Backend uses integer minor units; domain uses double for display convenience.
///
/// All naming drift is intentional and documented here.
/// ═══════════════════════════════════════════════════════════════════════════════
library;

import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// ForSaleDtoMapper
///
/// Converts backend forSale API responses to domain entities
class ForSaleDtoMapper {
  /// Convert ForSaleResponseDto to ForSale entity
  static ForSale toEntity(ForSaleResponseDto dto) {
    // Convert backend typed media items to MediaEntity.
    // Type is inferred from URL extension by the backend via InferMediaType.
    // Falls back to flat media_urls with image-only if typed media is absent.
    List<MediaEntity> media;
    if (dto.mediaItems.isNotEmpty) {
      media = dto.mediaItems
          .map(
            (item) => MediaEntity(
              id: item.id.isNotEmpty ? item.id : _generateMediaId(item.url),
              originalUrl: item.url,
              type: item.type == 'video' ? MediaType.video : MediaType.image,
              position: item.position,
              duration: item.duration,
              createdAt: DateTime.now(),
            ),
          )
          .toList();
    } else {
      // Fallback: flat media_urls with image-only (legacy)
      media = dto.mediaUrls
          .map(
            (url) => MediaEntity(
              id: _generateMediaId(url),
              originalUrl: url,
              type: MediaType.image,
              createdAt: DateTime.now(),
            ),
          )
          .toList();
    }

    // Owner Truth: username = account; farmName = seller/store; fullName = private/KYC.
    // Identity slots map directly from backend identity scalars:
    //   sellerUsername  ← seller_username
    //   sellerFarmName  ← seller_farm_name
    //   sellerAvatar    ← seller_avatar_url
    // No fullName fallback (KYC field).
    return ForSale(
      forSaleId: dto.id,
      productId: dto.productId,
      title: dto.title,
      description: dto.description,
      price: dto.price.toDouble(),
      stock: dto.quantity,
      media: media,
      sellerId: dto.sellerId,
      sellerUsername: dto.sellerUsername,
      sellerFarmName: dto.sellerFarmName,
      sellerAvatar: dto.sellerAvatarUrl,
      // E8.2 — Canonical seller user-identity lifecycle parsed tolerantly.
      // Null / missing / unknown → active (legacy payloads stay backward
      // compatible).
      sellerUserLifecycle: ContentLifecycleParse.fromWire(
        dto.sellerUserLifecycle,
      ),
      // Expired-seller visibility — top-level seller-trust lifecycle.
      // Independent axis. Null / missing wire field → active so pre-rollout
      // payloads keep current render (the canonical parser fails CLOSED on
      // null, which would mis-classify every legacy forSale as unavailable).
      sellerTrustLifecycle: dto.sellerTrustLifecycle == null
          ? ContentLifecycle.active
          : ContentLifecycleParse.fromWire(dto.sellerTrustLifecycle),
      // Stage 2 — seller reputation tier badge. Pass-through; SellerTierBadge
      // handles null/basic/unknown gracefully (renders nothing).
      sellerTier: dto.sellerTier,
      status: _mapStatus(dto.status),
      visibility: _mapVisibility(dto.visibility),
      isNegotiable: dto.negotiationEnabled,
      viewCount: 0,
      preparationTime: _mapPreparationTime(dto.preparationTime),
      preparationNote: dto.preparationNote,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      variety: dto.variety,
      sizeCm: dto.sizeCm?.toDouble(),
      ageMonths: dto.ageMonths,
      gender: dto.gender,
      breeder: dto.breeder,
      bloodline: dto.bloodline,
    );
  }

  /// Convert list of ForSaleResponseDto to list of ForSale entities
  static List<ForSale> toEntityList(List<ForSaleResponseDto> dtos) {
    return dtos.map(toEntity).toList();
  }

  /// Map backend status string to ForSaleStatus enum
  ///
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// BACKEND TRUTH ALIGNMENT (Go backend):
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// Backend statuses: draft, active, sold, withdrawn
  /// Lifecycle flow: draft -> active -> sold/withdrawn
  ///
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// SAFETY FIRST - UNKNOWN STATUS HANDLING:
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// Unknown statuses are mapped to 'draft' (NOT 'active') for safety.
  /// This prevents showing unknown/new listings as purchasable.
  ///
  /// IF THIS BEHAVIOR CAUSES ISSUES: Fix the source (backend), don't change the default.
  /// ═══════════════════════════════════════════════════════════════════════════════
  static ForSaleStatus _mapStatus(String status) {
    switch (status.toLowerCase()) {
      case 'draft':
        return ForSaleStatus.draft;
      case 'active':
        return ForSaleStatus.active;
      case 'withdrawn':
        // Backend-authoritative status: seller removed from sale
        return ForSaleStatus.withdrawn;
      case 'sold':
        return ForSaleStatus.sold;
      default:
        // SAFETY: Unknown status defaults to draft (not active) to avoid false availability
        // This prevents showing unknown listings as purchasable
        // DO NOT change this to 'active' - fix the backend instead
        return ForSaleStatus.draft;
    }
  }

  /// Map backend visibility string to ForSaleVisibility enum
  ///
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// BACKEND TRUTH ALIGNMENT (Go backend):
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// Backend visibility: public, private
  ///
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// WORKSPACE vs MARKET BOUNDARY:
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// - private: Workspace-only (seller can create/edit without subscription)
  /// - public:  Market-visible (requires active seller subscription)
  ///
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// SAFETY FIRST - UNKNOWN VISIBILITY HANDLING:
  /// ═══════════════════════════════════════════════════════════════════════════════
  /// Unknown visibility defaults to 'private' for safety.
  /// This prevents accidentally showing private listings as market-visible.
  /// ═══════════════════════════════════════════════════════════════════════════════
  static ForSaleVisibility _mapVisibility(String? visibility) {
    if (visibility == null) {
      // Default to private for safety (workspace-only)
      return ForSaleVisibility.private;
    }
    switch (visibility.toLowerCase()) {
      case 'public':
        return ForSaleVisibility.public;
      case 'private':
        return ForSaleVisibility.private;
      default:
        // SAFETY: Unknown visibility defaults to private (not public) to avoid
        // accidentally showing workspace listings in the marketplace
        return ForSaleVisibility.private;
    }
  }

  /// Map backend preparation_time string to PreparationTime enum
  /// Defaults to 'immediate' for null/unknown values (safe default)
  static PreparationTime _mapPreparationTime(String? preparationTime) {
    return PreparationTime.fromJson(preparationTime);
  }

  /// Generate a simple media ID from URL for bridge migration
  static String _generateMediaId(String url) {
    final uri = Uri.tryParse(url);
    if (uri != null && uri.pathSegments.isNotEmpty) {
      return uri.pathSegments.last.split('.').first;
    }
    return url.hashCode.toString();
  }
}
