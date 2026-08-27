import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'shipping_notifier.dart';
import 'shipping_state.dart';
import '../../data/data.dart';
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
    NotifierProvider<ShippingNotifier, ShippingOptionsListState>(
      ShippingNotifier.new,
    );

/// Provider for ShippingOptionDetailNotifier
final shippingOptionDetailNotifierProvider =
    NotifierProvider<ShippingOptionDetailNotifier, ShippingOptionDetailState>(
      ShippingOptionDetailNotifier.new,
    );

// Phase 3 cleanup: deliveryCheckNotifierProvider and shippingProofNotifierProvider
// removed. Both had zero ref.watch / ref.read call sites. The underlying
// shippingRepositoryProvider and shippingProofRepositoryProvider remain for
// direct use (e.g. auction_claim_shipping_modal.dart calls
// shippingRepository.checkDeliveryAvailability directly).
