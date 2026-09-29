import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

/// Canonical buyer-facing forSale card — a THIN channel wrapper.
///
/// CANONICAL DESIGN (owner-locked): every public commerce card is one
/// `CommerceMarketplaceCardShell` inside the shared `CommerceMarketplaceGrid`
/// (2 columns). A channel may only fill slots; it may NOT re-implement the
/// frame, typography or media badge stack.
///
/// CARD CONTRACT (owner decision 2026-09-27) — IDENTICAL to [AuctionCard] so
/// the promotion grid can reuse this exact card for both channels:
///   media (4:5 contain, video chip OVERLAY on the media)
///   → title (one line) → value (one line).
/// No badge row under the media, and NO seller identity: username and store
/// name never render on a discovery card (detail + search keep identity,
/// redaction and the seller-trust badge).
///
/// Used by every discovery / browsing surface (Marketplace, ForSaleList,
/// ProfileStore) and by future promoted For Sale placements.
///
/// NOT for seller management surfaces — use SellerForSaleManagementCard there.
class ForSaleCard extends StatelessWidget {
  final ForSale forSale;
  final VoidCallback onTap;

  const ForSaleCard({super.key, required this.forSale, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final media = forSale.media.isNotEmpty ? forSale.media.first : null;

    return CommerceMarketplaceCardShell(
      onTap: onTap,
      semanticLabel: forSale.title,
      media: CommerceMarketplaceCardMedia(
        imageUrl: media?.thumbnailUrl,
        mediaType: media?.type ?? MediaType.image,
        fallback: Icon(
          Icons.image_outlined,
          size: 48,
          color: scheme.onSurfaceVariant,
        ),
      ),
      title: forSale.title,
      value: CommerceMarketplaceCardValue(
        value: forSale.formattedPrice,
        compact: true,
      ),
    );
  }
}
