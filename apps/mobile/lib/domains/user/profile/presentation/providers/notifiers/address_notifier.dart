import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;

part 'address_notifier.g.dart';

/// Address Notifier - Application Layer Orchestrator
///
/// Phase 5: Replaces old address providers with Riverpod Notifier
/// Using @riverpod annotation for code generation (pure Riverpod, no get_it)
///
/// Responsibilities:
/// - Orchestrate address CRUD operations
/// - Manage loading/error states
/// - Delegate to repository interface (no direct API/Firebase access)
///
/// 🚫 RULES:
/// - No Firebase imports
/// - No get_it/service locator
/// - No UI logic (formatting, etc.)
/// - Only business orchestration
@riverpod
class AddressNotifier extends _$AddressNotifier {
  @override
  AddressState build() {
    // Repository is injected via ref.watch() - no get_it!
    return const AddressState();
  }

  /// Load all addresses for a user
  Future<void> loadAddresses(String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    state = const AddressState(addresses: AsyncValue.loading());

    final result = await repository.getAddressesByUserId(userId);

    result.fold(
      (error) {
        state = AddressState(
          addresses: AsyncValue.error(error, StackTrace.current),
          primaryAddress: const AsyncValue.data(null),
        );
      },
      (data) {
        state = AddressState(
          addresses: AsyncValue.data(data),
          primaryAddress: const AsyncValue.data(null),
        );
      },
    );
  }

  /// Load addresses by purpose (shipping/sender)
  Future<void> loadAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {
    final repository = ref.read(addressRepositoryProvider);

    state = const AddressState(addresses: AsyncValue.loading());

    final result = await repository.getAddressesByPurpose(userId, purpose);

    result.fold(
      (error) {
        state = AddressState(
          addresses: AsyncValue.error(error, StackTrace.current),
        );
      },
      (data) {
        state = AddressState(addresses: AsyncValue.data(data));
      },
    );
  }

  /// Load primary address
  Future<void> loadPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    final repository = ref.read(addressRepositoryProvider);

    final result = await repository.getPrimaryAddress(userId, purpose: purpose);

    result.fold(
      (error) {
        state = state.copyWith(
          primaryAddress: AsyncValue.error(error, StackTrace.current),
        );
      },
      (data) {
        state = state.copyWith(primaryAddress: AsyncValue.data(data));
      },
    );
  }

  /// Add new address
  Future<void> addAddress(AddressEntity address) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(isSaving: true);

    final result = await repository.addAddress(address);

    result.fold(
      (error) {
        state = state.copyWith(isSaving: false, errorMessage: error);
      },
      (_) {
        // Refresh addresses after add
        loadAddresses(address.userId);
      },
    );
  }

  /// Update existing address
  Future<void> updateAddress(AddressEntity address) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(isSaving: true);

    final result = await repository.updateAddress(address);

    result.fold(
      (error) {
        state = state.copyWith(isSaving: false, errorMessage: error);
      },
      (_) {
        // Refresh addresses after update
        loadAddresses(address.userId);
      },
    );
  }

  /// Delete address
  Future<void> deleteAddress(String addressId, String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(isDeleting: true);

    final result = await repository.deleteAddress(addressId);

    result.fold(
      (error) {
        state = state.copyWith(isDeleting: false, errorMessage: error);
      },
      (_) {
        // Refresh addresses after delete
        loadAddresses(userId);
      },
    );
  }

  /// Set address as primary
  Future<void> setPrimaryAddress(String addressId, String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(isSaving: true);

    final result = await repository.setPrimaryAddress(addressId, userId);

    result.fold(
      (error) {
        state = state.copyWith(isSaving: false, errorMessage: error);
      },
      (_) {
        // Refresh addresses after setting primary
        loadAddresses(userId);
      },
    );
  }

  /// Get address count
  Future<int> getAddressCount(String userId, {AddressPurpose? purpose}) async {
    final repository = ref.read(addressRepositoryProvider);

    final result = await repository.countAddresses(userId, purpose: purpose);

    return result.fold((error) => 0, (count) => count);
  }

  /// Clear error message
  void clearError() {
    state = state.copyWith(errorMessage: null);
  }

  /// Reset to initial state
  void reset() {
    state = const AddressState();
  }
}

// Alias for backward compatibility
// The generated provider name is 'addressProvider', but code expects 'addressNotifierProvider'
final addressNotifierProvider = addressProvider;
