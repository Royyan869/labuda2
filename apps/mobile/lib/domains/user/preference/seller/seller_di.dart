/// Seller Refactor DI Helper
///
/// Dependency Injection helper for seller module.
///
/// ⚠️ ATURAN: File ini hanya berisi helper class untuk overrides.
/// Repository provider sudah di-export dari data/seller_providers.dart
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart';
import 'package:labuda/domains/user/preference/seller/domain/domain.dart';

// Re-export repository provider for convenience
export 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRepositoryProvider;

// ============================================
// DASHBOARD STATS PROVIDER
// ============================================

/// FutureProvider for dashboard stats (replaces old sellerDashboardStatsProvider)
final sellerDashboardStatsProvider =
    FutureProvider.family<SellerDashboardStats, String>((ref, sellerId) async {
      final repository = ref.read(sellerRepositoryProvider);
      final result = await repository.getDashboardStats(sellerId);

      if (result.isSuccess && result.data != null) {
        return result.data!;
      }
      throw Exception(result.error ?? 'Failed to load dashboard stats');
    });

// ============================================
// ACTIVITY PROVIDERS
// ============================================

/// FutureProvider for recent activity (replaces old sellerRecentActivityProvider)
final sellerRecentActivityProvider =
    FutureProvider.family<List<RecentActivityItem>, String>((
      ref,
      sellerId,
    ) async {
      final repository = ref.read(sellerRepositoryProvider);
      final result = await repository.getRecentActivity(sellerId, limit: 10);

      if (result.isSuccess && result.data != null) {
        return result.data!;
      }
      return [];
    });

// ============================================
// SUBSCRIPTION PROVIDERS
// ============================================

/// StreamProvider for subscription (real-time)
final sellerSubscriptionProvider =
    StreamProvider.family<SellerSubscription?, String>((ref, sellerId) {
      final repository = ref.read(sellerRepositoryProvider);
      return repository.watchSubscription(sellerId);
    });

/// FutureProvider for subscription (one-time)
final sellerSubscriptionFutureProvider =
    FutureProvider.family<SellerSubscription, String>((ref, sellerId) async {
      final repository = ref.read(sellerRepositoryProvider);
      final result = await repository.getSubscription(sellerId);

      if (result.isSuccess && result.data != null) {
        return result.data!;
      }
      return SellerSubscription.empty();
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
