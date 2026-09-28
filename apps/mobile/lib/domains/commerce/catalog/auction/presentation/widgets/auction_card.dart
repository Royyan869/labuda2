import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_time_extension.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

/// Canonical buyer-facing auction card — a THIN channel wrapper.
///
/// CANONICAL DESIGN (owner-locked): every public commerce card is one
/// `CommerceMarketplaceCardShell` inside the shared `CommerceMarketplaceGrid`
/// (2 columns). A channel may only fill slots; it may NOT re-implement the
/// frame, typography or media badge stack.
///
/// CARD CONTRACT (owner decision 2026-09-27) — IDENTICAL to [ForSaleCard] so
/// the promotion grid can reuse this exact card for both channels:
///   media (4:5 contain, countdown/status chip as an OVERLAY on the media)
///   → title (one line) → value (one line, current bid / opening bid).
/// No badge row under the media, and NO seller identity: username and store
/// name never render on a discovery card (detail + search keep identity,
/// redaction and the seller-trust badge).
///
/// Used by every discovery / browsing surface (Marketplace, ProfileStore) and
/// by future promoted Auction placements.
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
        overlay: _mediaOverlay(scheme),
        fallback: Icon(
          Icons.image_outlined,
          size: 48,
          color: scheme.onSurfaceVariant,
        ),
      ),
      title: auction.title,
      value: CommerceMarketplaceCardValue(value: _priceLabel, compact: true),
    );
  }

  /// Item-state chip rendered ON the media (bottom-left) instead of a row
  /// under it — keeps For Sale and Auction card rhythm identical.
  Widget _mediaOverlay(ColorScheme scheme) {
    if (auction.isActive) {
      final timeRemaining = auction.getTimeRemaining();
      return CommerceMarketplaceCardBadge(
        label: timeRemaining.displayText,
        backgroundColor: _urgencyColor(scheme, timeRemaining.urgencyLevel),
        foregroundColor: timeRemaining.urgencyLevel == AuctionUrgencyLevel.ended
            ? scheme.surface
            : scheme.onPrimary,
        compact: true,
      );
    }

    return CommerceMarketplaceCardBadge(
      label: auction.status.displayName,
      compact: true,
    );
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
