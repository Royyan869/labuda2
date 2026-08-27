/// Auction Detail Info
///
/// Shows auction detailed information
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';

/// Detail info widget for auction
class AuctionDetailInfo extends StatelessWidget {
  final Auction auction;

  const AuctionDetailInfo({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      color: isDark ? AppColors.darkGray800 : Colors.white,
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
              color: isDark
                  ? AppColors.neutralGray800.withValues(alpha: 0.5)
                  : AppColors.neutralGray100,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 16,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Lelang — harga naik, penawar tertinggi menang',
                    style: TextStyle(
                      fontSize: 13,
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          _buildInfoRow('Kelamin', auction.koiDetails.gender),
          _buildInfoRow('Ukuran', '${auction.koiDetails.sizeInCm} cm'),
          _buildInfoRow('Varietas', auction.koiDetails.variety),
          _buildInfoRow(
            'Bid Increment',
            'Rp ${auction.bidIncrement.toStringAsFixed(0)}',
          ),
          const SizedBox(height: 12),
          const Text(
            'Deskripsi',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(auction.description),
        ],
      ),
    );
  }

  Widget _buildInfoRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(color: Colors.grey)),
          Text(value, style: const TextStyle(fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}
