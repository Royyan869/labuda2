/// Profile Store Tab - Commerce surface for seller profiles
///
/// Displays seller's commerce items (forSales, auctions) in sub-tabs.
/// This is a PUBLIC VIEW surface, not a management dashboard.
///
/// Structure:
/// - Dijual (For Sale): Shows seller's active forSales
/// - Lelang (Auction): Shows seller's active auctions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/auction/auction.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/shared.dart';

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
// FOR SALE TAB - Shows seller's forSales
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

    // CANONICAL LAYOUT: shared 2-column grid (public commerce surface).
    final activeForSales = (forSalesAsync.asData?.value ?? const <ForSale>[])
        .where((forSale) => forSale.status == ForSaleStatus.active)
        .toList();

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(sellerForSalesProvider(params));
      },
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          CommerceMarketplaceGrid(
            itemCount: activeForSales.length,
            isLoading: forSalesAsync.isLoading,
            error: forSalesAsync.hasError ? forSalesAsync.error : null,
            itemBuilder: (context, index) {
              final forSale = activeForSales[index];
              return ForSaleCard(
                forSale: forSale,
                onTap: () => _navigateToForSaleDetail(ref, forSale.forSaleId),
              );
            },
            emptyBuilder: (context) => EmptyState(
              icon: Icons.storefront_outlined,
              title: context.l10n.emptyForSaleTitle,
              subtitle: context.l10n.emptySellerForSaleMessage,
            ),
            // CANONICAL page-level error (PageErrorState): safe localized
            // copy only, the raw [error] never reaches the screen.
            errorBuilder: (context, error, stackTrace) => PageErrorState(
              onRetry: () => ref.invalidate(sellerForSalesProvider(params)),
            ),
          ),
        ],
      ),
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
    // One engine with For Sale: FutureProvider, not Stream
    final auctionsAsync = ref.watch(sellerAuctionsProvider(userId));

    // CANONICAL LAYOUT: shared 2-column grid (public commerce surface).
    final activeAuctions = (auctionsAsync.asData?.value ?? const <Auction>[])
        .where(
          (a) =>
              (a.status == AuctionStatus.scheduled ||
                  a.status == AuctionStatus.active) &&
              !a.hasEnded,
        )
        .toList();

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(sellerAuctionsProvider(userId));
      },
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          CommerceMarketplaceGrid(
            itemCount: activeAuctions.length,
            isLoading: auctionsAsync.isLoading,
            error: auctionsAsync.hasError ? auctionsAsync.error : null,
            itemBuilder: (context, index) {
              final auction = activeAuctions[index];
              return AuctionCard(
                auction: auction,
                onTap: () => _navigateToAuctionDetail(ref, auction.id),
              );
            },
            emptyBuilder: (context) => EmptyState(
              icon: Icons.gavel_outlined,
              title: context.l10n.emptyAuctionTitle,
              subtitle: context.l10n.emptySellerAuctionMessage,
            ),
            // CANONICAL page-level error (PageErrorState): safe localized
            // copy only, the raw [error] never reaches the screen.
            errorBuilder: (context, error, stackTrace) => PageErrorState(
              onRetry: () => ref.invalidate(sellerAuctionsProvider(userId)),
            ),
          ),
        ],
      ),
    );
  }

  void _navigateToAuctionDetail(WidgetRef ref, String auctionId) {
    final navigation = ref.read(navigationHandlerProvider);
    navigation.navigateToAuction(auctionId);
  }
}
