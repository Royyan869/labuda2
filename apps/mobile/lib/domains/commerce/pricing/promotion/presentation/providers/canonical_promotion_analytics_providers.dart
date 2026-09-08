/// Canonical Promotion Analytics Providers
///
/// Riverpod providers for canonical promotion delivery analytics.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/canonical_promotion_analytics_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/canonical_promotion_analytics_repository.dart';

/// Canonical promotion analytics repository provider.
final canonicalPromotionAnalyticsRepositoryProvider =
    Provider<CanonicalPromotionAnalyticsRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return CanonicalPromotionAnalyticsRepositoryImpl(apiClient);
});

/// Canonical promotion delivery analytics provider.
///
/// Fetches analytics for a specific canonical promotion contract ID.
/// Returns [Result.success] with analytics data or [Result.error] on failure.
///
/// The backend enforces ownership - only the contract owner can read analytics.
/// A foreign/missing contract ID returns the same error (no existence oracle).
final canonicalPromotionDeliveryAnalyticsProvider = FutureProvider.autoDispose
    .family<Result<CanonicalPromotionAnalyticsDto>, String>(
        (ref, contractId) async {
  final repository = ref.watch(canonicalPromotionAnalyticsRepositoryProvider);
  return repository.getDeliveryAnalytics(contractId);
});