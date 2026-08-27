import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:labuda/features/explore/presentation/providers/explore_promotion_providers.dart';
import 'package:labuda/features/explore/presentation/widgets/explore_promoted_section.dart';

/// Auction tab content for Explore screen.
///
/// FEED / DISCOVERY QUALITY PASS V1:
/// - Commerce surface (auction only)
/// - Promoted auctions appear in a small top section
/// - Organic auction stream remains the primary browse experience
/// - External products are intentionally excluded
class ExploreAuctionTab extends ConsumerWidget {
  const ExploreAuctionTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auctionsAsync = ref.watch(exploreAuctionsStreamProvider);
    final promotedIdsAsync = ref.watch(explorePromotedAuctionIdsProvider);

    return auctionsAsync.when(
      data: (auctions) {
        final promotedIds = promotedIdsAsync.maybeWhen(
          data: (ids) => ids.toSet(),
          orElse: () => <String>{},
        );
        final promotedAuctions = auctions
            .where((auction) => promotedIds.contains(auction.id))
            .toList();
        final organicAuctions = auctions
            .where((auction) => !promotedIds.contains(auction.id))
            .toList();

        return CustomScrollView(
          physics: const AlwaysScrollableScrollPhysics(),
          slivers: [
            if (promotedAuctions.isNotEmpty)
              SliverToBoxAdapter(
                child: ExplorePromotedSection(
                  title: 'Lelang Dipromosikan',
                  children: promotedAuctions
                      .map(
                        (auction) => AuctionCard(
                          auction: auction,
                          onTap: () =>
                              _navigateToAuctionDetail(context, auction),
                        ),
                      )
                      .toList(),
                ),
              ),
            if (organicAuctions.isEmpty && promotedAuctions.isEmpty)
              SliverFillRemaining(child: _buildEmptyState(context))
            else
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                sliver: SliverList(
                  delegate: SliverChildBuilderDelegate((context, index) {
                    final auction = organicAuctions[index];
                    return AuctionCard(
                      auction: auction,
                      onTap: () => _navigateToAuctionDetail(context, auction),
                    );
                  }, childCount: organicAuctions.length),
                ),
              ),
          ],
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => const Center(child: Text('Data belum bisa dimuat.')),
    );
  }

  void _navigateToAuctionDetail(BuildContext context, Auction auction) {
    context.go('/auction/${auction.id}');
  }

  Widget _buildEmptyState(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.gavel_outlined,
              size: 64,
              color: AppColors.neutralGray400,
            ),
            const SizedBox(height: 16),
            Text(
              'Belum ada lelang aktif',
              style: AppTypography.h6.copyWith(color: AppColors.neutralGray700),
            ),
            const SizedBox(height: 8),
            Text(
              'Cek lagi nanti ya!',
              style: AppTypography.bodyMedium.copyWith(
                color: AppColors.neutralGray500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
