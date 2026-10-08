import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

/// Auction tab content for Marketplace screen.
///
/// CANONICAL LAYOUT (owner-locked): public marketplace surfaces render through
/// the shared `CommerceMarketplaceGrid` (2 columns) with the shared empty /
/// error vocabulary. A surface may not own its own list or state widgets.
class MarketplaceAuctionTab extends ConsumerWidget {
  const MarketplaceAuctionTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // One engine with For Sale: FutureProvider + RefreshIndicator + invalidate
    // after create.
    final auctionsAsync = ref.watch(marketplaceAuctionsProvider);
    final auctions = auctionsAsync.asData?.value ?? const <Auction>[];

    return RefreshIndicator(
      onRefresh: () => ref.refresh(marketplaceAuctionsProvider.future),
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          CommerceMarketplaceGrid(
            itemCount: auctions.length,
            isLoading: auctionsAsync.isLoading,
            error: auctionsAsync.hasError ? auctionsAsync.error : null,
            itemBuilder: (context, index) {
              final auction = auctions[index];
              return AuctionCard(
                auction: auction,
                onTap: () => context.push('/auction/${auction.id}'),
              );
            },
            emptyBuilder: (context) => EmptyState(
              icon: Icons.gavel_outlined,
              title: context.l10n.emptyAuctionTitle,
              subtitle: context.l10n.emptyCheckBackMessage,
            ),
            // CANONICAL page-level error (PageErrorState): safe localized
            // copy only, the raw [error] never reaches the screen.
            errorBuilder: (context, error, stackTrace) => PageErrorState(
              onRetry: () => ref.invalidate(marketplaceAuctionsProvider),
            ),
          ),
        ],
      ),
    );
  }
}
