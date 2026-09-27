/// Shared commerce presentation providers that are NOT owned by a single
/// sale channel.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Resolves the seller's sender (farm) address id for a CREATE request.
///
/// CANONICAL SCOPE: the shipping origin is Product content, not a for_sale
/// concept — BOTH sale channels (for_sale + auction) must carry it on create
/// so `products.farm_address_id` is never channel-dependent.
///
/// The seller-upgrade wizard guarantees a sender address exists; this read
/// picks the primary one (first as fallback) so CREATE = PUBLISH always
/// carries the origin address in the same request.
final senderAddressIdProvider = FutureProvider<String?>((ref) async {
  final authState = ref.watch(authControllerProvider);
  if (authState is! AuthStateAuthenticated) return null;
  final userId = authState.user.id;
  if (userId.isEmpty) return null;

  final repository = ref.watch(addressRepositoryProvider);
  final result = await repository.getAddressesByPurpose(
    userId,
    AddressPurpose.sender,
  );
  final addresses = result.data;
  if (addresses == null || addresses.isEmpty) return null;
  final primary = addresses.where((a) => a.isPrimary).firstOrNull;
  return (primary ?? addresses.first).id;
});
