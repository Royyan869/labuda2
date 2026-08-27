/// Coins Feature (Loyalty Points System)
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
/// Coins CANNOT be:
/// - Withdrawn as cash
/// - Transferred to other users
/// - Used as payment instrument
/// - Stored as monetary value
/// - Exchanged for fiat currency
///
/// Coins MAY ONLY be used as:
/// - Discount input during checkout
/// - Promotional rewards
///
/// Clean architecture implementation with:
/// - Domain layer: Entities, Repository interfaces
/// - Data layer: DTOs, Mappers, Datasource, Repository implementation
/// - Presentation layer: Providers (Riverpod - no get_it)
///
/// ⚠️ ATURAN MIGRASI_2_GUIDE:
/// - Datasource, Repository Implementation, DTOs, Mappers TIDAK BOLEH di-export
/// - Hanya Domain entities dan Repository interfaces yang boleh di-export
///
/// ⚠️ OWNERSHIP RULE:
/// - This is the SINGLE SOURCE OF TRUTH for Coins domain
/// - Other features (checkout, payment) MUST consume this, NOT duplicate
library;

// Domain Layer (ONLY these are exported)
export 'domain/entities/coin_balance.dart';
export 'domain/entities/coin_transaction.dart';
export 'domain/repositories/coin_repository.dart';

// DI - Core providers (only place with datasource/repo impl imports)
export 'coins_di.dart';

// Presentation Layer
export 'presentation/providers/coin_providers.dart';
export 'presentation/providers/coin_notifier.dart';
export 'presentation/providers/coin_state.dart';

// Screens
export 'presentation/screens/coin_balance_screen.dart';
export 'presentation/screens/coin_history_screen.dart';

// Widgets
export 'presentation/widgets/coin_balance_card.dart';

// ❌ DILARANG EXPORT (sesuai MIGRASI_2_GUIDE):
// - DTOs (data/dto/coin_dto.dart)
// - Mappers (data/mappers/coin_mapper.dart)
// - Datasource (data/remote/coin_api_datasource.dart)
// - Repository Implementation (data/coin_api_repository_impl.dart)
