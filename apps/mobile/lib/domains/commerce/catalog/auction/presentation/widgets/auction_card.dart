import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_time_extension.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_card_seller_metadata.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

/// Canonical buyer-facing auction card — a THIN channel wrapper.
///
/// CANONICAL DESIGN (owner-locked): every public commerce card is one
/// `CommerceMarketplaceCardShell` inside the shared `CommerceMarketplaceGrid`
/// (2 columns). A channel may only fill slots; it may NOT re-implement the
/// frame, typography, media badge or seller block.
///
/// Slots: badges (item state — auction urgency/status lives here) → title →
/// value (money) → seller metadata. Channel-specific content lives in the
/// slot data only (current bid here, fixed price on [ForSaleCard]);
/// description belongs to the detail surface.
///
/// Used by every discovery / browsing surface (Marketplace, ProfileStore).
/// Seller identity is redacted when [Auction.sellerUserLifecycle] is degraded,
/// providing parity with SearchResultItem (E8.4).
///
/// NOT for seller management surfaces.
class AuctionCard extends StatelessWidget {
  final Auction auction;
  final VoidCallback onTap;

  const AuctionCard({super.key, required this.auction, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final media = auction.media.isNotEmpty ? auction.media.first : null;

    return CommerceMarketplaceCardShell(
      onTap: onTap,
      semanticLabel: auction.title,
      media: CommerceMarketplaceCardMedia(
        imageUrl: media?.originalUrl,
        mediaType: media?.type ?? MediaType.image,
        fallback: Icon(
          Icons.image_outlined,
          size: 48,
          color: scheme.onSurfaceVariant,
        ),
      ),
      badges: _badges(scheme),
      title: auction.title,
      value: CommerceMarketplaceCardValue(value: _priceLabel, compact: true),
      metadata: CommerceCardSellerMetadata(
        username: auction.sellerUsername,
        storeName: auction.sellerFarmName,
        sellerUserLifecycle: auction.sellerUserLifecycle,
        sellerTrustLifecycle: auction.sellerTrustLifecycle,
      ),
    );
  }

  /// Item-state badges. PURGED vs the old card: the "X bid" badge (the wire
  /// never emitted total_bids — always-zero fake truth, owner decision
  /// 2026-09-25) and the location row (AuctionLocation was never hydrated).
  List<Widget> _badges(ColorScheme scheme) {
    final badges = <Widget>[];

    if (auction.isActive) {
      final timeRemaining = auction.getTimeRemaining();
      badges.add(
        CommerceMarketplaceCardBadge(
          label: timeRemaining.displayText,
          backgroundColor: _urgencyColor(scheme, timeRemaining.urgencyLevel),
          foregroundColor:
              timeRemaining.urgencyLevel == AuctionUrgencyLevel.ended
              ? scheme.surface
              : scheme.onPrimary,
          compact: true,
        ),
      );
    } else {
      badges.add(
        CommerceMarketplaceCardBadge(
          label: auction.status.displayName,
          compact: true,
        ),
      );
    }

    return badges;
  }

  String get _priceLabel {
    final currentBid = auction.currentBid;
    if (currentBid > 0) {
      return 'Rp ${formatGroupedAmount(currentBid.round())}';
    }
    return 'Mulai Rp ${formatGroupedAmount(auction.startingBid.round())}';
  }

  Color _urgencyColor(ColorScheme scheme, AuctionUrgencyLevel level) {
    switch (level) {
      case AuctionUrgencyLevel.critical:
        return scheme.error.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.warning:
        return AppColors.statusWarning.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.normal:
        return AppColors.statusSuccess.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.ended:
        return scheme.onSurfaceVariant;
    }
  }
}
