/// Auction Seller Card
///
/// Shows seller information
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

/// Seller card widget for auction detail
class AuctionSellerCard extends StatelessWidget {
  final Auction auction;

  const AuctionSellerCard({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    // E8.2 — Seller user-identity lifecycle redaction. When the seller's
    // user identity is degraded (banned/deleted), render an italic
    // placeholder + neutral avatar. The auction itself is controlled by
    // `auction.status` and stays visible — seller-user lifecycle MUST NOT
    // hide the item.
    //
    // AXIS BOUNDARY: Only the user-axis gate fires. Top-level
    // auction.seller.lifecycle (seller trust / capability axis) is NOT
    // consumed — doctrine-reserved.
    final sellerDegraded = auction.sellerUserLifecycle.isDegraded;

    final identity = sellerDegraded
        ? null
        : buildCommerceSellerIdentity(
            username: auction.sellerUsername,
            storeName: auction.sellerFarmName,
          );

    if (!sellerDegraded && identity == null) {
      return const SizedBox.shrink();
    }

    final String primaryLabel;
    if (sellerDegraded) {
      // Canonical 2-string redaction vocabulary.
      // Parity rule: degraded seller is NOT tappable and exposes no
      // navigation affordance (chevron / InkWell). This card is structurally
      // non-interactive even for active sellers, so the tap-gate is
      // satisfied by the absence of an InkWell wrapper below — mirroring
      // _ListingSellerCard._buildDegradedRow.
      primaryLabel = auction.sellerUserLifecycle.publicRedactionLabel;
    } else if (identity != null) {
      primaryLabel = identity.line1;
    } else {
      primaryLabel = '';
    }

    // Show the store/farm line as subtitle when present.
    final showHandleSubtitle = identity?.line2 != null;

    final showAvatar = !sellerDegraded && auction.sellerAvatar != null;

    // Stage 2 — Seller tier badge. Visible only when:
    //   1. User-identity axis is active (sellerDegraded == false).
    //   2. Seller-trust axis is active (subscription not expired).
    //   3. Tier is "pro" or "elite" (SellerTierBadge hides null/basic/unknown).
    // Lifecycle governance dominates: badge NEVER renders for degraded sellers.
    final tierBadgeVisible =
        !sellerDegraded &&
        auction.sellerTrustLifecycle == ContentLifecycle.active &&
        auction.sellerTier != null;

    return Container(
      color: Colors.white,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              CircleAvatar(
                radius: 24,
                backgroundColor: sellerDegraded ? Colors.grey[200] : null,
                backgroundImage: showAvatar
                    ? NetworkImage(auction.sellerAvatar!)
                    : null,
                child: showAvatar
                    ? null
                    : Icon(
                        Icons.person,
                        color: sellerDegraded ? Colors.grey[500] : null,
                      ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      primaryLabel,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        fontStyle: sellerDegraded
                            ? FontStyle.italic
                            : FontStyle.normal,
                        color: sellerDegraded ? Colors.grey[500] : null,
                      ),
                    ),
                    if (showHandleSubtitle)
                      Text(
                        identity!.line2!,
                        style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                      ),
                  ],
                ),
              ),
            ],
          ),
          // Stage 2 — tier badge as subtle secondary trust signal below the
          // identity row. SellerInactiveBadge (trust axis degraded) dominates
          // the card visually above; this badge only shows when fully active.
          if (tierBadgeVisible) ...[
            const SizedBox(height: 8),
            SellerTierBadge(tier: auction.sellerTier),
          ],
        ],
      ),
    );
  }
}
