import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;

part 'address_notifier.g.dart';

/// Address Notifier - Application Layer Orchestrator
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
    // The single canonical load (`loadAddresses`) is triggered by each
    // consuming surface (Address List mount, checkout section mount), so the
    // notifier starts in loading with no data rather than an empty result.
    return const AddressState(addresses: AsyncValue.loading());
  }

  /// Load the account's address book — the single canonical operation behind
  /// initial load, retry, refresh, and post-mutation reload.
  ///
  /// - No data yet → full loading (first-load state).
  /// - Data present → the last-known-good collection stays visible while the
  ///   load runs (refresh), and a failure keeps it with [AddressState.refreshError]
  ///   set. A refresh failure is never allowed to become a full-page error or
  ///   a silent empty list.
  Future<void> loadAddresses(String userId) async {
    final repository = ref.read(addressRepositoryProvider);
    final previous = state.addresses;
    final hasPrevious = previous.hasValue;

    state = state.copyWith(
      addresses: hasPrevious ? previous : const AsyncValue.loading(),
      isRefreshing: hasPrevious,
      isSaving: false,
      isDeleting: false,
      errorMessage: null,
      refreshError: null,
    );

    final result = await repository.getAddressesByUserId(userId);

    result.fold(
      (error) {
        if (hasPrevious) {
          state = state.copyWith(isRefreshing: false, refreshError: error);
          return;
        }
        state = AddressState(
          addresses: AsyncValue.error(error, StackTrace.current),
          primaryAddress: const AsyncValue.data(null),
        );
      },
      (data) {
        state = AddressState(
          addresses: AsyncValue.data(data),
          primaryAddress: AsyncValue.data(
            data.where((a) => a.isPrimary).firstOrNull,
          ),
        );
      },
    );
  }

  /// Load the account's primary address
  Future<void> loadPrimaryAddress(String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    final result = await repository.getPrimaryAddress(userId);

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

  /// Add a new address through the canonical authority. Reloads the shared
  /// collection on success; returns false (and keeps the collection intact)
  /// on failure. The caller renders a controlled message from
  /// [AddressState.errorMessage] — never the raw backend text.
  Future<bool> addAddress(AddressEntity address) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(
      isSaving: true,
      errorMessage: null,
      refreshError: null,
    );

    final result = await repository.addAddress(address);

    if (result.isError) {
      state = state.copyWith(isSaving: false, errorMessage: result.error);
      return false;
    }

    await loadAddresses(address.userId);
    return true;
  }

  /// Update an address through the canonical authority.
  Future<bool> updateAddress(AddressEntity address) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(
      isSaving: true,
      errorMessage: null,
      refreshError: null,
    );

    final result = await repository.updateAddress(address);

    if (result.isError) {
      state = state.copyWith(isSaving: false, errorMessage: result.error);
      return false;
    }

    await loadAddresses(address.userId);
    return true;
  }

  /// Delete an address through the canonical authority.
  Future<bool> deleteAddress(String addressId, String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(
      isDeleting: true,
      errorMessage: null,
      refreshError: null,
    );

    final result = await repository.deleteAddress(addressId);

    if (result.isError) {
      state = state.copyWith(isDeleting: false, errorMessage: result.error);
      return false;
    }

    await loadAddresses(userId);
    return true;
  }

  /// Set an address as primary through the canonical authority.
  Future<bool> setPrimaryAddress(String addressId, String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    state = state.copyWith(
      isSaving: true,
      errorMessage: null,
      refreshError: null,
    );

    final result = await repository.setPrimaryAddress(addressId, userId);

    if (result.isError) {
      state = state.copyWith(isSaving: false, errorMessage: result.error);
      return false;
    }

    await loadAddresses(userId);
    return true;
  }

  /// Get address count
  Future<int> getAddressCount(String userId) async {
    final repository = ref.read(addressRepositoryProvider);

    final result = await repository.countAddresses(userId);

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
