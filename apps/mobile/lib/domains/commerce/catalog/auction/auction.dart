// Auction Feature Module
// Clean Architecture implementation

// Domain Layer
export 'domain/domain.dart';

// Data Layer - NOT EXPORTED (Implementation Detail)
// ❌ NOT exported: data/data.dart, data/dto/, data/mappers/, data/remote/
// These are internal to the module and accessed via repository interface only.

// Presentation layer - Providers (State, Notifier)
export 'presentation/providers/auction_state.dart';
export 'presentation/providers/auction_notifier.dart';
export 'presentation/providers/auction_providers.dart';

// Presentation layer - Screens
export 'presentation/screens/auction_list_screen.dart';
export 'presentation/screens/auction_detail_screen.dart';
export 'presentation/screens/create_auction_screen.dart';

// Presentation layer - Barrel
export 'presentation/presentation.dart';
