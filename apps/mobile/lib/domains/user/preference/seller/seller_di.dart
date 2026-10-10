/// Seller Refactor DI Helper
///
/// Dependency Injection helper for seller module.
///
/// ⚠️ ATURAN: File ini hanya berisi helper class untuk overrides.
/// Repository provider sudah di-export dari data/seller_providers.dart
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/user/preference/seller/data/seller_providers.dart';
import 'package:hishumi/domains/user/preference/seller/domain/domain.dart';

// Re-export repository provider for convenience
export 'package:hishumi/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRepositoryProvider;

// ============================================
// SUBSCRIPTION PROVIDERS
// ============================================

/// FutureProvider for subscription (one-time)
final sellerSubscriptionFutureProvider =
    FutureProvider.family<SellerSubscription, String>((ref, sellerId) async {
      final repository = ref.read(sellerRepositoryProvider);
      final result = await repository.getSubscription(sellerId);

      if (result.isSuccess && result.data != null) {
        return result.data!;
      }
      // Do NOT fabricate a SellerSubscription here. An API/network failure
      // (or a 403/404 from the market-gated endpoint) must not become a fake
      // "subscription expiring now" business state downstream. Surface the
      // failure as an AsyncError so consumers see null data instead.
      throw Exception(result.error ?? 'Failed to load seller subscription');
    });

// ============================================
// ANALYTICS PROVIDER
// ============================================

/// FutureProvider for seller analytics (30-day read projection).
final sellerAnalyticsProvider = FutureProvider.family<SellerAnalytics, String>((
  ref,
  sellerId,
) async {
  final repository = ref.read(sellerRepositoryProvider);
  final result = await repository.getAnalytics(sellerId);

  if (result.isSuccess && result.data != null) {
    return result.data!;
  }
  throw Exception(result.error ?? 'Failed to load analytics');
});

// ============================================
// EARNINGS PROVIDER
// ============================================

/// FutureProvider for seller earnings
/// Matches backend GET /api/v1/seller/earnings response
final sellerEarningsProvider = FutureProvider.family<SellerEarnings, String>((
  ref,
  sellerId,
) async {
  final repository = ref.read(sellerRepositoryProvider);
  final result = await repository.getEarnings(sellerId);

  if (result.isSuccess && result.data != null) {
    return result.data!;
  }
  throw Exception(result.error ?? 'Failed to load earnings');
});

// ============================================
// PERFORMANCE PROVIDER
// ============================================

/// FutureProvider for seller performance (canonical Reputation + Rating
/// projection). Matches backend GET /api/v1/seller/performance response.
final sellerPerformanceProvider = FutureProvider.family<SellerPerformance, String>((
  ref,
  sellerId,
) async {
  final repository = ref.read(sellerRepositoryProvider);
  final result = await repository.getPerformance(sellerId);

  if (result.isSuccess && result.data != null) {
    return result.data!;
  }
  throw Exception(result.error ?? 'Failed to load performance');
});
