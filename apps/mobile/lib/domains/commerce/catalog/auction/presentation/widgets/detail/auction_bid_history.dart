/// Auction Bid History
///
/// Shows list of bids for the auction
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';

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
                // Owner Truth: bidderUsername is the public bidder identity.
                // No fake fallback ('Anonymous', 'Bidder', 'User'): when the
                // username is empty, the avatar shows a generic person icon
                // and the title is hidden — bid amount still renders.
                final hasName = bid.bidderUsername.isNotEmpty;
                return ListTile(
                  dense: true,
                  leading: CircleAvatar(
                    radius: 16,
                    child: hasName
                        ? Text(bid.bidderUsername[0].toUpperCase())
                        : const Icon(Icons.person, size: 16),
                  ),
                  title: hasName ? Text('@${bid.bidderUsername}') : null,
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
