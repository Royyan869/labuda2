/// Checkout Data Providers
///
/// Data layer dependency injection using pure Riverpod.
/// MIGRATION: Previously accessed via ServiceLocator or direct instantiation.
/// Now uses constructor injection via Riverpod providers.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart';

// Re-export repository interface for presentation layer
// (CheckoutRepository interface is defined in the impl file - pre-existing structure)
export 'package:labuda/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart'
    show CheckoutRepository, CheckoutException;

// =============================================================================
// DATA LAYER PROVIDERS (Dependency Injection)
// =============================================================================

/// Provider for CheckoutRepository
///
/// Uses core ApiClient and logger services directly.
/// MIGRATED: No UnimplementedError override pattern.
final checkoutRepositoryProvider = Provider<CheckoutRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return CheckoutRepositoryImpl(apiClient, logger: logger);
});
