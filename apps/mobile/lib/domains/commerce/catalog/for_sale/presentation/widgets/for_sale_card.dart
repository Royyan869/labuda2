import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_card_seller_metadata.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

/// Canonical buyer-facing forSale card — a THIN channel wrapper.
///
/// CANONICAL DESIGN (owner-locked): every public commerce card is one
/// `CommerceMarketplaceCardShell` inside the shared `CommerceMarketplaceGrid`
/// (2 columns). A channel may only fill slots; it may NOT re-implement the
/// frame, typography, media badge or seller block.
///
/// Slots: badges (item state) → title → value (money) → seller metadata.
/// Channel-specific content lives in the slot data only (fixed price here,
/// current bid on [AuctionCard]); description belongs to the detail surface.
///
/// Used by every discovery / browsing surface (Marketplace, ForSaleList,
/// ProfileStore). Seller identity is redacted when [ForSale.sellerUserLifecycle]
/// is degraded, providing parity with SearchResultItem (E8.4).
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
        imageUrl: media?.originalUrl,
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
      metadata: CommerceCardSellerMetadata(
        username: forSale.sellerUsername,
        storeName: forSale.sellerFarmName,
        sellerUserLifecycle: forSale.sellerUserLifecycle,
        sellerTrustLifecycle: forSale.sellerTrustLifecycle,
      ),
    );
  }
}
