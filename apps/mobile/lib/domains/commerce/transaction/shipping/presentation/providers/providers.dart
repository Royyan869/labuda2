import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'shipping_notifier.dart';
import 'shipping_state.dart';
import '../../data/data.dart';
import '../../data/repositories/shipping_quote_repository.dart';
import '../../domain/domain.dart';

// =====================================
// Repository Providers
// =====================================

/// Provider for ShippingRemoteDatasource
final shippingRemoteDatasourceProvider = Provider<ShippingRemoteDatasource>((
  ref,
) {
  final apiClient = ref.watch(apiClientProvider);
  return ShippingRemoteDatasource(apiClient);
});

/// Provider for ShippingRepository
final shippingRepositoryProvider = Provider<ShippingRepository>((ref) {
  final datasource = ref.watch(shippingRemoteDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return ShippingRepositoryImpl(datasource: datasource, logger: logger);
});

/// Provider for ShippingQuoteRepository — the manual shipping quote
/// (ongkir) transport authority. Kept apart from setup CRUD so the quote
/// write path has exactly one home in the Shipping domain.
final shippingQuoteRepositoryProvider = Provider<ShippingQuoteRepository>((
  ref,
) {
  return ShippingQuoteRepository(ref.watch(shippingRemoteDatasourceProvider));
});

/// Provider for ShippingProofRepository
final shippingProofRepositoryProvider = Provider<ShippingProofRepository>((
  ref,
) {
  final datasource = ref.watch(shippingRemoteDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return ShippingProofRepositoryImpl(datasource: datasource, logger: logger);
});

// =====================================
// Notifier Providers
// =====================================

/// Provider for ShippingNotifier
final shippingNotifierProvider =
    NotifierProvider<ShippingNotifier, ShippingSetupsListState>(
      ShippingNotifier.new,
    );

// KILLED DESIGN: shippingSetupDetailNotifierProvider removed with the
// per-coverage CRUD contract (one-package contract replaces it).
// Phase 3 cleanup: deliveryCheckNotifierProvider and shippingProofNotifierProvider
// removed. Both had zero ref.watch / ref.read call sites. The underlying
// shippingRepositoryProvider and shippingProofRepositoryProvider remain for
// direct use (e.g. checkout_screen_impl.dart calls
// shippingRepository.checkDeliveryAvailability directly).
