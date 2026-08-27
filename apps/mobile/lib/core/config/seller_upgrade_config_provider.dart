import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_service.dart';

/// Provider for SellerUpgradeConfigService
final sellerUpgradeConfigServiceProvider = Provider<SellerUpgradeConfigService>(
  (ref) {
    return SellerUpgradeConfigService(apiClient: ref.watch(apiClientProvider));
  },
);

/// Provider for getting seller upgrade configuration (one-time fetch)
final sellerUpgradeConfigProvider = FutureProvider<SellerUpgradeConfigEntity>((
  ref,
) {
  return ref.read(sellerUpgradeConfigServiceProvider).getConfiguration();
});

/// Provider for watching seller upgrade configuration (real-time updates)
final watchSellerUpgradeConfigProvider =
    StreamProvider<SellerUpgradeConfigEntity>((ref) {
      return ref.read(sellerUpgradeConfigServiceProvider).watchConfiguration();
    });
