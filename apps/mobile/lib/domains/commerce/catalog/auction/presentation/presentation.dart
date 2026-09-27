/// Auction Refactor - Presentation Layer
/// Exports providers, screens, and barrel for presentation
library;

// Providers
export 'providers/auction_providers.dart';

// Screens
// NOTE: auction_list_screen.dart was a dormant, unrouted browse surface built
// on a placeholder 2-column card. Its contract (marketplace_surface_contract_test)
// demands removal — public auction discovery lives in MarketplaceAuctionTab on
// the shared CommerceMarketplaceGrid.
export 'screens/auction_detail_screen.dart';
export 'screens/create_auction_screen.dart';
