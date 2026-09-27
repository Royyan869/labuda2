/// Auction Bid Section
///
/// Shows current bid and bid increment info
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';

/// Bid section widget for auction detail
class AuctionBidSection extends StatelessWidget {
  final Auction auction;

  const AuctionBidSection({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    final currentBid = auction.currentBid;
    final nextBid = currentBid + auction.bidIncrement;
    final colorScheme = Theme.of(context).colorScheme;

    // CANONICAL SECTION FRAME — the same 16-margin card language the ForSale
    // detail uses, so both detail surfaces read as one design.
    return CommerceDetailSectionCard(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Bid Saat Ini',
                style: TextStyle(
                  fontSize: 14,
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
              Text(
                'Rp ${formatGroupedAmount(currentBid.round())}',
                style: const TextStyle(
                  fontSize: 24,
                  fontWeight: FontWeight.bold,
                  color: AppColors.statusSuccess,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Bid Berikutnya',
                style: TextStyle(
                  fontSize: 12,
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
              Text(
                'Rp ${formatGroupedAmount(nextBid.round())}',
                style: const TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
          if (auction.buyNowPrice != null) ...[
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  'Buy Now',
                  style: TextStyle(
                    fontSize: 12,
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
                Text(
                  'Rp ${formatGroupedAmount(auction.buyNowPrice!.round())}',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: colorScheme.secondary,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
