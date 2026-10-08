// Address Presentation Providers
//
// These providers wrap the AddressNotifier and provide convenient
// access patterns for UI components. There is ONE canonical address state
// authority: `addressProvider` (AddressNotifier). Address List and checkout
// both consume it; no second list read exists.

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'notifiers/address_notifier.dart';
import '../../domain/entities/address_entity.dart';

/// Provider for addresses list state
final addressesListProvider = Provider<AsyncValue<List<AddressEntity>>>((ref) {
  final addressState = ref.watch(addressProvider);
  return addressState.addresses;
});

/// Provider for primary address state
final primaryAddressProvider = Provider<AsyncValue<AddressEntity?>>((ref) {
  final addressState = ref.watch(addressProvider);
  return addressState.primaryAddress;
});

/// Provider for address loading state
final isAddressLoadingProvider = Provider<bool>((ref) {
  final addressState = ref.watch(addressProvider);
  return addressState.isSaving || addressState.isDeleting;
});

/// Provider for address error message
final addressErrorProvider = Provider<String?>((ref) {
  final addressState = ref.watch(addressProvider);
  return addressState.errorMessage;
});

/// Provider for address count
/// Returns the count of loaded addresses (0 if loading or error)
final addressCountProvider = Provider<int>((ref) {
  final addressState = ref.watch(addressProvider);
  return addressState.addresses.when(
    data: (addresses) => addresses.length,
    loading: () => 0,
    error: (_, _) => 0,
  );
});
