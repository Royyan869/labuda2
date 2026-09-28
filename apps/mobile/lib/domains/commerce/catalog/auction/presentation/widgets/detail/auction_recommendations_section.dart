/// Auction Recommendations Section
///
/// Shows other auctions from same seller and similar auctions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/utils/media_extensions.dart';
import 'package:labuda/shared/widgets/app_image.dart';

/// Recommendations section widget for auction detail
class AuctionRecommendationsSection extends StatelessWidget {
  final Auction currentAuction;
  final AsyncValue<List<Auction>> ownerOtherAuctions;
  final AsyncValue<List<Auction>> similarAuctions;

  const AuctionRecommendationsSection({
    super.key,
    required this.currentAuction,
    required this.ownerOtherAuctions,
    required this.similarAuctions,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Owner's other auctions
        ownerOtherAuctions.when(
          data: (auctions) {
            if (auctions.isEmpty) return const SizedBox.shrink();
            return _buildSection(
              context,
              'Lelang Lainnya dari Penjual',
              auctions,
            );
          },
          loading: () => const SizedBox.shrink(),
          error: (_, _) => const SizedBox.shrink(),
        ),
        const SizedBox(height: 16),
        // Similar auctions
        similarAuctions.when(
          data: (auctions) {
            if (auctions.isEmpty) return const SizedBox.shrink();
            return _buildSection(context, 'Lelang Serupa', auctions);
          },
          loading: () => const SizedBox.shrink(),
          error: (_, _) => const SizedBox.shrink(),
        ),
      ],
    );
  }

  Widget _buildSection(
    BuildContext context,
    String title,
    List<Auction> auctions,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 8),
        SizedBox(
          height: 180,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            itemCount: auctions.length,
            separatorBuilder: (_, _) => const SizedBox(width: 12),
            itemBuilder: (context, index) {
              final auction = auctions[index];
              final colorScheme = Theme.of(context).colorScheme;
              return SizedBox(
                width: 140,
                child: Card(
                  clipBehavior: Clip.antiAlias,
                  child: InkWell(
                    onTap: () {
                      // Navigation handled by parent via AuctionDetailScreen
                      // Recommendations are display-only; tapping a card
                      // would require navigation context - future enhancement
                    },
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (auction.media.isNotEmptyUrls)
                          Expanded(
                            child: AppImage(
                              imageUrl: auction.media.firstUrl,
                              width: double.infinity,
                              fit: BoxFit.cover,
                              backgroundColor:
                                  colorScheme.surfaceContainerHighest,
                              errorWidget: Container(
                                color:
                                    colorScheme.surfaceContainerHighest,
                                child: Icon(
                                  Icons.image,
                                  color: colorScheme.onSurfaceVariant,
                                ),
                              ),
                            ),
                          )
                        else
                          Expanded(
                            child: Container(
                              color:
                                  colorScheme.surfaceContainerHighest,
                              child: Icon(
                                Icons.image,
                                color: colorScheme.onSurfaceVariant,
                              ),
                            ),
                          ),
                        Padding(
                          padding: const EdgeInsets.all(8),
                          child: Text(
                            auction.title,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 12),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}
