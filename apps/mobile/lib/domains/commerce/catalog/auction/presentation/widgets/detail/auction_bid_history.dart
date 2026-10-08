/// Auction Bid History
///
/// Shows list of bids for the auction.
/// Bidder identity is redacted when [AuctionBid.bidderLifecycle] is
/// degraded (unavailable/removed), providing parity with other seller/
/// bidder identity surfaces (E8.4).
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';

/// Bid history widget for auction detail
class AuctionBidHistory extends StatelessWidget {
  final List<AuctionBid> bids;

  const AuctionBidHistory({super.key, required this.bids});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    // CANONICAL SECTION FRAME — same card language as the ForSale detail.
    return CommerceDetailSectionCard(
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p0,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Riwayat Bid (${bids.length})',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 12),
          if (bids.isEmpty)
            Text(
              'Belum ada bid',
              style: TextStyle(color: colorScheme.onSurfaceVariant),
            )
          else
            ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: bids.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, index) {
                final bid = bids[index];
                final lifecycle = ContentLifecycleParse.fromWire(
                  bid.bidderLifecycle,
                );
                final isDegraded = lifecycle.isDegraded;
                final hasName = bid.bidderUsername.isNotEmpty && !isDegraded;
                final displayName = isDegraded
                    ? lifecycle.publicRedactionLabel
                    : (bid.bidderUsername.isNotEmpty
                          ? '@${bid.bidderUsername}'
                          : null);
                return ListTile(
                  dense: true,
                  leading: ProfileAvatar(
                    userId: bid.bidderId,
                    size: 32,
                    imageUrl: (hasName && bid.bidderAvatarUrl != null)
                        ? bid.bidderAvatarUrl
                        : null,
                  ),
                  title: displayName != null
                      ? Text(
                          displayName,
                          style: isDegraded
                              ? const TextStyle(fontStyle: FontStyle.italic)
                              : null,
                        )
                      : null,
                  trailing: Text(
                    'Rp ${formatGroupedAmount(bid.amount.round())}',
                    style: TextStyle(
                      fontWeight: FontWeight.bold,
                      color: context.statusColors.success,
                    ),
                  ),
                );
              },
            ),
        ],
      ),
    );
  }
}
