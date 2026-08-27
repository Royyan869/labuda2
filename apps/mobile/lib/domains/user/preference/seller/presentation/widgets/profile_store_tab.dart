/// Profile Store Tab - Commerce surface for seller profiles
///
/// Displays seller's commerce items (listings, auctions) in sub-tabs.
/// This is a PUBLIC VIEW surface, not a management dashboard.
///
/// Structure:
/// - Dijual (For Sale): Shows seller's active listings
/// - Lelang (Auction): Shows seller's active auctions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:labuda/shared/shared.dart';

/// Main store tab with sub-tabs for commerce items
class ProfileStoreTab extends ConsumerStatefulWidget {
  final String userId;
  final TabController subTabController;

  const ProfileStoreTab({
    super.key,
    required this.userId,
    required this.subTabController,
  });

  @override
  ConsumerState<ProfileStoreTab> createState() => _ProfileStoreTabState();
}

class _ProfileStoreTabState extends ConsumerState<ProfileStoreTab> {
  @override
  Widget build(BuildContext context) {
    return TabBarView(
      controller: widget.subTabController,
      children: [
        _ForSaleTab(userId: widget.userId),
        _AuctionTab(userId: widget.userId),
      ],
    );
  }
}

// =============================================================================
// FOR SALE TAB - Shows seller's listings
// =============================================================================

class _ForSaleTab extends ConsumerWidget {
  final String userId;

  const _ForSaleTab({required this.userId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final params = SellerForSalesParams(
      sellerId: userId,
      page: 1,
      pageSize: 50,
    );

    final forSalesAsync = ref.watch(sellerForSalesProvider(params));

    return forSalesAsync.when(
      data: (forSales) {
        // Filter to show only active listings (public view)
        final activeForSales = forSales
            .where((forSale) => forSale.status == ForSaleStatus.active)
            .toList();

        if (activeForSales.isEmpty) {
          return _buildEmptyState(context);
        }

        return RefreshIndicator(
          onRefresh: () async {
            ref.invalidate(sellerForSalesProvider(params));
          },
          child: ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: activeForSales.length,
            itemBuilder: (context, index) {
              final forSale = activeForSales[index];
              return ForSaleCard(
                listing: forSale,
                onTap: () => _navigateToForSaleDetail(ref, forSale.forSaleId),
              );
            },
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: AppColors.statusError),
            const SizedBox(height: 16),
            Text('Gagal memuat listing', style: AppTypography.bodyLarge),
            const SizedBox(height: 8),
            Text(
              error.toString(),
              style: AppTypography.bodySmall.copyWith(
                color: AppColors.neutralGray600,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    return ListView(
      children: [
        SizedBox(
          height: MediaQuery.of(context).size.height * 0.5,
          child: EmptyStateWidget.list(
            title: 'Belum ada listing',
            subtitle: 'Seller ini belum memiliki listing aktif',
          ),
        ),
      ],
    );
  }

  void _navigateToForSaleDetail(WidgetRef ref, String forSaleId) {
    final navigation = ref.read(navigationHandlerProvider);
    navigation.navigateToForSaleDetail(forSaleId);
  }
}

// =============================================================================
// AUCTION TAB - Shows seller's auctions
// =============================================================================

class _AuctionTab extends ConsumerWidget {
  final String userId;

  const _AuctionTab({required this.userId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auctionsAsync = ref.watch(userAuctionsStreamProvider(userId));

    return auctionsAsync.when(
      data: (auctions) {
        // Filter to show only active auctions
        final activeAuctions = auctions
            .where((a) => a.isActive && !a.hasEnded)
            .toList();

        if (activeAuctions.isEmpty) {
          return _buildEmptyState(context);
        }

        return RefreshIndicator(
          onRefresh: () async {
            ref.invalidate(userAuctionsStreamProvider(userId));
          },
          child: ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: activeAuctions.length,
            itemBuilder: (context, index) {
              final auction = activeAuctions[index];
              return AuctionCard(
                auction: auction,
                onTap: () => _navigateToAuctionDetail(ref, auction.id),
              );
            },
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: AppColors.statusError),
            const SizedBox(height: 16),
            Text('Gagal memuat lelang', style: AppTypography.bodyLarge),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    return ListView(
      children: [
        SizedBox(
          height: MediaQuery.of(context).size.height * 0.5,
          child: EmptyStateWidget.list(
            title: 'Belum ada lelang',
            subtitle: 'Seller ini belum memiliki lelang aktif',
          ),
        ),
      ],
    );
  }

  void _navigateToAuctionDetail(WidgetRef ref, String auctionId) {
    final navigation = ref.read(navigationHandlerProvider);
    navigation.navigateToAuction(auctionId);
  }
}
