/// Coins DI Helper
///
/// Dependency Injection configuration for Coins module.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
///
/// ⚠️ ATURAN: File ini satu-satunya tempat yang boleh import datasource/repo impl
/// untuk keperluan DI. Layer lain HANYA boleh menggunakan repository interface.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/repositories/coin_repository.dart';
import 'package:labuda/domains/finance/wallet/coins/data/coin_api_repository_impl.dart';
import 'package:labuda/domains/finance/wallet/coins/data/remote/coin_api_datasource.dart';

// Export for convenience
export 'domain/entities/coin_balance.dart' show CoinBalance;
export 'domain/entities/coin_transaction.dart'
    show CoinTransaction, CoinTransactionType, CoinSourceType;
export 'domain/repositories/coin_repository.dart' show CoinRepository;

// ============================================================
// CORE DEPENDENCIES - imported from core_providers
// ============================================================

// ============================================================
// DATASOURCE PROVIDERS
// ============================================================

/// Provides CoinApiDatasource (Go Backend)
///
/// ⚠️ INTERNAL USE ONLY - Not for export to public API
final coinApiDatasourceProvider = Provider<CoinApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return CoinApiDatasource(apiClient, logger: logger);
});

// ============================================================
// REPOSITORY PROVIDERS
// ============================================================

/// Provider for CoinRepository (API version)
///
/// Migrated to Go Backend API - Firestore version deprecated
final coinRepositoryProvider = Provider<CoinRepository>((ref) {
  final datasource = ref.watch(coinApiDatasourceProvider);
  return CoinApiRepositoryImpl(datasource: datasource);
});

// ============================================================
// DI HELPER (for main app)
// ============================================================

/// Coins DI Helper
///
/// Helper class for initializing Coins module dependencies.
class CoinsDI {
  /// Create provider overrides for Coins module
  ///
  /// Use this in ProviderScope to provide ApiClient and ILoggerService:
  /// ```dart
  /// ProviderScope(
  ///   overrides: [
  ///     ...CoinsDI.overrides(
  ///       apiClient: ApiDI.apiClient,
  ///       logger: ServiceLocator.getService<ILoggerService>(),
  ///     ),
  ///   ],
  ///   child: MyApp(),
  /// )
  /// ```
  static List overrides({
    required ApiClient apiClient,
    required ILoggerService logger,
  }) {
    return [
      apiClientProvider.overrideWithValue(apiClient),
      loggerServiceProvider.overrideWithValue(logger),
    ];
  }
}
