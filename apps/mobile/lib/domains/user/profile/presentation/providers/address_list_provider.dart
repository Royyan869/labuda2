// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';

// Core
import 'package:labuda/core/core.dart';

// Profile Domain
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

// Profile Data - Use the canonical provider from data layer
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;

// =============================================================================
// ADDRESS LIST PROVIDERS
// =============================================================================

/// Stream provider for addresses
///
/// For API implementation, uses polling (30s interval) instead of Firestore streams.
/// TODO: Phase 3D - Replace with WebSocket when implemented.
final addressesStreamProvider =
    StreamProvider.family<Result<List<AddressEntity>>, String>((ref, userId) {
      final repository = ref.watch(addressRepositoryProvider);
      return repository.watchAddresses(userId);
    });

/// Provider for primary address (any purpose - legacy)
final primaryAddressProvider =
    FutureProvider.family<Result<AddressEntity?>, String>((ref, userId) async {
      final repository = ref.watch(addressRepositoryProvider);
      return repository.getPrimaryAddress(userId);
    });

/// Provider for primary shipping address (for checkout)
/// Returns: primary shipping address, or first shipping address
final primaryShippingAddressProvider =
    FutureProvider.family<Result<AddressEntity?>, String>((ref, userId) async {
      final repository = ref.watch(addressRepositoryProvider);

      // Get shipping addresses only
      final result = await repository.getAddressesByPurpose(
        userId,
        AddressPurpose.shipping,
      );

      if (result.isError || result.data == null) {
        return Result.error(result.error ?? 'Failed to get addresses');
      }

      final shippingAddresses = result.data!;

      if (shippingAddresses.isEmpty) {
        return Result.success(null);
      }

      // Priority 1: Primary shipping address
      final primaryShipping = shippingAddresses
          .where((addr) => addr.isPrimary)
          .firstOrNull;

      if (primaryShipping != null) {
        return Result.success(primaryShipping);
      }

      // Priority 2: First shipping address (fallback)
      return Result.success(shippingAddresses.first);
    });

/// Provider for address count
final addressCountProvider = FutureProvider.family<Result<int>, String>((
  ref,
  userId,
) async {
  final repository = ref.watch(addressRepositoryProvider);
  return repository.countAddresses(userId);
});
