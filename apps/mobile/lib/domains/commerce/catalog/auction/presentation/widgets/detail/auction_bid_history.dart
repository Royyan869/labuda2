/// Auction Bid History
///
/// Shows list of bids for the auction.
/// Bidder identity is redacted when [AuctionBid.bidderLifecycle] is
/// degraded (unavailable/removed), providing parity with other seller/
/// bidder identity surfaces (E8.4).
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Bid history widget for auction detail
class AuctionBidHistory extends StatelessWidget {
  final List<AuctionBid> bids;

  const AuctionBidHistory({super.key, required this.bids});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Colors.white,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Riwayat Bid (${bids.length})',
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 12),
          if (bids.isEmpty)
            const Text('Belum ada bid', style: TextStyle(color: Colors.grey))
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
                  leading: CircleAvatar(
                    radius: 16,
                    child: hasName
                        ? Text(bid.bidderUsername[0].toUpperCase())
                        : const Icon(Icons.person, size: 16),
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
                    'Rp ${bid.amount.toStringAsFixed(0)}',
                    style: const TextStyle(
                      fontWeight: FontWeight.bold,
                      color: Colors.green,
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
