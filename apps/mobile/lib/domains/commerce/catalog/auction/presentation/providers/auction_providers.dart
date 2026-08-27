/// Auction Refactor - Presentation Providers
/// Re-exports all providers from data and presentation layers for easy importing
library;

// Re-export repository providers from data layer
export 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider, auctionWatchRepositoryProvider;

// Re-export all presentation providers from auction_notifier.dart
export 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart'
    show
        auctionNotifierProvider,
        exploreAuctionsStreamProvider,
        userAuctionsStreamProvider,
        myAuctionsStreamProvider,
        auctionStreamProvider,
        auctionBidsStreamProvider,
        watchStatsStreamProvider,
        auctionDetailProvider,
        auctionBidsProvider,
        watchedAuctionsProvider;
