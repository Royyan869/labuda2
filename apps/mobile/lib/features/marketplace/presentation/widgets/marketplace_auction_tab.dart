import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';

/// Auction tab content for Marketplace screen.
class MarketplaceAuctionTab extends ConsumerWidget {
  const MarketplaceAuctionTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auctionsAsync = ref.watch(marketplaceAuctionsStreamProvider);

    return auctionsAsync.when(
      data: (auctions) {
        return CustomScrollView(
          physics: const AlwaysScrollableScrollPhysics(),
          slivers: [
            if (auctions.isEmpty)
              SliverFillRemaining(child: _buildEmptyState(context))
            else
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                sliver: SliverList(
                  delegate: SliverChildBuilderDelegate((context, index) {
                    final auction = auctions[index];
                    return AuctionCard(
                      auction: auction,
                      onTap: () => _navigateToAuctionDetail(context, auction),
                    );
                  }, childCount: auctions.length),
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
