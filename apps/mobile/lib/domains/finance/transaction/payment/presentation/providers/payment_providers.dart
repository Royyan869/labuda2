/// Payment Providers
///
/// Implementation providers for dependency injection.
/// These are used to wire up the repositories in the main app.
///
/// ⚠️ COINS OWNERSHIP:
/// - Coin repository provider has been REMOVED from this file
/// - Coins are now owned by domains/finance/wallet/coins module
/// - Use coinRepositoryProvider from domains/finance/wallet/coins/coins_di.dart
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import '../../domain/repositories/payment_repository.dart';
import '../../data/repositories/payment_repository_impl.dart';
import '../../data/remote/payment_remote_datasource.dart';

// ============================================================================
// Manual Providers (no @riverpod to avoid code generation issues)
// ============================================================================

// Note: apiClientProvider and loggerServiceProvider are now imported from core.dart
// These are defined in shared/providers/core_providers.dart

/// Payment remote datasource provider
final paymentRemoteDatasourceProvider = Provider<PaymentRemoteDatasource>((
  ref,
) {
  return PaymentRemoteDatasource(
    ref.watch(apiClientProvider),
    logger: ref.watch(loggerServiceProvider),
  );
});

/// Payment repository provider
/// Returns the implementation but typed as interface
final paymentRepositoryProvider = Provider<PaymentRepository>((ref) {
  return PaymentRepositoryImpl(
    datasource: ref.watch(paymentRemoteDatasourceProvider),
    logger: ref.watch(loggerServiceProvider),
  );
});
