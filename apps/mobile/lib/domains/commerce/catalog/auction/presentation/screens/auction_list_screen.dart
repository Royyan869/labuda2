/// Auction List Screen
/// Displays list of active auctions with filter options
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';

/// Auction List Screen
///
/// Shows active auctions in a 2-column grid layout
/// Uses exploreAuctionsStreamProvider from auction_refactor
class AuctionListScreen extends ConsumerStatefulWidget {
  const AuctionListScreen({super.key});

  @override
  ConsumerState<AuctionListScreen> createState() => _AuctionListScreenState();
}

class _AuctionListScreenState extends ConsumerState<AuctionListScreen> {
  @override
  Widget build(BuildContext context) {
    final auctionsAsync = ref.watch(exploreAuctionsStreamProvider);

    return PopScope(
      canPop: true,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Active Auctions'),
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ),
        body: SafeArea(
          child: Column(
            children: [
              _buildFilterChips(),
              Expanded(
                child: auctionsAsync.when(
                  loading: () =>
                      const Center(child: CircularProgressIndicator()),
                  error: (error, _) =>
                      const Center(child: Text('Data belum bisa dimuat.')),
                  data: (auctions) => auctions.isEmpty
                      ? _buildEmptyState()
                      : _buildAuctionsList(auctions),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildFilterChips() {
    return Container(
      height: 60,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: ListView(
        scrollDirection: Axis.horizontal,
        children: [
          FilterChip(
            label: const Text('Active'),
            selected: true,
            onSelected: (selected) {},
          ),
          const SizedBox(width: 8),
          FilterChip(
            label: const Text('Ending Soon'),
            selected: false,
            onSelected: (selected) {},
          ),
          const SizedBox(width: 8),
          FilterChip(
            label: const Text('Buy Now'),
            selected: false,
            onSelected: (selected) {},
          ),
          const SizedBox(width: 8),
          FilterChip(
            label: const Text('Kohaku'),
            selected: false,
            onSelected: (selected) {},
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.gavel,
            size: 64,
            color: Theme.of(context).colorScheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            'No active auctions',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            'Check back later for new auctions',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }

  Widget _buildAuctionsList(List<Auction> auctions) {
    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: (auctions.length / 2).ceil(),
      itemBuilder: (context, rowIndex) {
        final firstIndex = rowIndex * 2;
        final secondIndex = firstIndex + 1;
        final hasSecondItem = secondIndex < auctions.length;

        return Padding(
          padding: EdgeInsets.only(
            bottom: rowIndex < (auctions.length / 2).ceil() - 1 ? 12 : 0,
          ),
          child: IntrinsicHeight(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Expanded(child: _buildAuctionPlaceholder(auctions[firstIndex])),
                if (hasSecondItem) ...[
                  const SizedBox(width: 12),
                  Expanded(
                    child: _buildAuctionPlaceholder(auctions[secondIndex]),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }

  void _navigateToAuctionDetail(Auction auction) {
    ref.read(navigationHandlerProvider).navigateToAuction(auction.id);
  }

  /// Placeholder widget until dedicated AuctionCard component is implemented
  /// This provides basic auction display with navigation support
  Widget _buildAuctionPlaceholder(Auction auction) {
    return Card(
      child: InkWell(
        onTap: () => _navigateToAuctionDetail(auction),
        child: Padding(
          padding: const EdgeInsets.all(8.0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                auction.title,
                style: const TextStyle(fontWeight: FontWeight.bold),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 4),
              Text('Starting bid: ${auction.openingBid}'),
            ],
          ),
        ),
      ),
    );
  }
}
