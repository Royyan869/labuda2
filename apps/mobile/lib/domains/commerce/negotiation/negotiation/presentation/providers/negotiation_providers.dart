/// Negotiation Presentation Providers - Presentation layer providers for Negotiation feature
///
/// R4.1B DI MIGRATION: Migrated from ApiDI.apiClient to canonical apiClientProvider
///
/// Previous implementation used ApiDI.apiClient directly which violates
/// canonical DI path. Now uses apiClientProvider from core/providers/core_providers.dart.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import '../../data/remote/negotiation_remote_datasource.dart';
import '../../data/negotiation_repository_impl.dart';
import '../../domain/repositories/negotiation_repository.dart';
import 'negotiation_notifier.dart';
import 'negotiation_state.dart';

// ============================================================================
// R4.1B DI MIGRATION: Using canonical core providers
// ============================================================================
// Previous implementation used ApiDI.apiClient directly.
// Now uses apiClientProvider from core/providers/core_providers.dart.
// ============================================================================

// ============================================================================
// DATA LAYER PROVIDERS
// ============================================================================

/// Remote Datasource provider - uses canonical apiClientProvider
final negotiationRemoteDatasourceProvider =
    Provider<NegotiationRemoteDatasource>((ref) {
      final apiClient = ref.watch(apiClientProvider);
      return NegotiationRemoteDatasource(apiClient: apiClient);
    });

/// Repository provider
final negotiationRepositoryProvider = Provider<NegotiationRepository>((ref) {
  final remote = ref.watch(negotiationRemoteDatasourceProvider);
  return NegotiationRepositoryImpl(remote: remote);
});

// ============================================================================
// PRESENTATION LAYER PROVIDERS (Notifiers)
// ============================================================================

/// Notifier provider
final negotiationNotifierProvider =
    NotifierProvider<NegotiationNotifier, NegotiationState>(
      NegotiationNotifier.new,
    );
