/// Auction Bid Section
///
/// Shows current bid and bid increment info
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';

/// Bid section widget for auction detail
class AuctionBidSection extends StatelessWidget {
  final Auction auction;

  const AuctionBidSection({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    final currentBid = auction.currentBid;
    final nextBid = currentBid + auction.bidIncrement;

    return Container(
      color: Colors.white,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                'Bid Saat Ini',
                style: TextStyle(fontSize: 14, color: Colors.grey),
              ),
              Text(
                'Rp ${currentBid.toStringAsFixed(0)}',
                style: const TextStyle(
                  fontSize: 24,
                  fontWeight: FontWeight.bold,
                  color: Colors.green,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                'Bid Berikutnya',
                style: TextStyle(fontSize: 12, color: Colors.grey),
              ),
              Text(
                'Rp ${nextBid.toStringAsFixed(0)}',
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
                const Text(
                  'Buy Now',
                  style: TextStyle(fontSize: 12, color: Colors.grey),
                ),
                Text(
                  'Rp ${auction.buyNowPrice!.toStringAsFixed(0)}',
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: Colors.blue,
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
