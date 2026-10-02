/// Auction Seller Card
///
/// Thin channel wrapper over the canonical
/// [CommerceDetailSellerCard] — the render authority for the seller block on
/// BOTH detail surfaces. This file only maps the Auction read model onto the
/// shared contract; it owns no layout, typography or gating logic.
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_seller_card.dart';

class AuctionSellerCard extends StatelessWidget {
  final Auction auction;

  const AuctionSellerCard({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    return CommerceDetailSellerCard(
      sellerId: auction.sellerId,
      username: auction.sellerUsername,
      storeName: auction.sellerFarmName,
      avatarUrl: auction.sellerAvatar,
      originLine: auction.publicOriginLine,
      sellerUserLifecycle: auction.sellerUserLifecycle,
      sellerTrustLifecycle: auction.sellerTrustLifecycle,
      tier: auction.sellerTier,
    );
  }
}
