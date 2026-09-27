/// Auction Detail Info
///
/// Shows auction detailed information
library;

import 'package:flutter/material.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart';

/// Detail info widget for auction
///
/// Canonical Product content from the detail wire (variety, size_cm,
/// age_months, gender, breeder, bloodline, certificates, preparation_time,
/// preparation_note, description) is consumed through the shared
/// [CommerceCommonProductDetailSection] — the same section the ForSale/ForSale
/// sibling uses, so no canonical value preserved in the Auction read model is
/// left dead. 'Bid Increment' remains the auction-specific row.
class AuctionDetailInfo extends StatelessWidget {
  final Auction auction;

  const AuctionDetailInfo({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      color: colorScheme.surface,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Detail Lelang',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          // AUCTION EXPLANATION - Minimal 1-line explanation
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: colorScheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 16,
                  color: colorScheme.onSurfaceVariant,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Lelang — harga naik, penawar tertinggi menang',
                    style: TextStyle(
                      fontSize: 13,
                      color: colorScheme.onSurfaceVariant,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          // Canonical shared product detail section — renders only the rows
          // whose canonical value is present (variety/size/age/gender/
          // breeder/bloodline/certificates/preparation/description). When the
          // payload carries none of them it collapses to nothing.
          CommerceCommonProductDetailSection(
            title: '',
            data: CommerceCommonProductDetailsData.fromAuction(auction),
          ),
          const SizedBox(height: 12),
          _buildInfoRow(
            context,
            'Bid Increment',
            'Rp ${formatGroupedAmount(auction.bidIncrement.round())}',
          ),
        ],
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    final colorScheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: TextStyle(color: colorScheme.onSurfaceVariant),
          ),
          Text(value, style: const TextStyle(fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}
