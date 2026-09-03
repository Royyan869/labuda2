import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'shipping_state.dart';
import 'providers.dart';
import '../../domain/domain.dart';

/// Notifier untuk Shipping Options Management
/// Menggunakan Riverpod Notifier (bukan StateNotifier untuk menghindari masalah compatibility)
class ShippingNotifier extends Notifier<ShippingSetupsListState> {
  ShippingRepository get _repository => ref.read(shippingRepositoryProvider);

  @override
  ShippingSetupsListState build() {
    return const ShippingSetupsListInitial();
  }

  /// Load all shipping options for a seller
  Future<void> loadShippingSetups() async {
    state = const ShippingSetupsListLoading();
    final result = await _repository.listMyShippingSetups();

    final newState = result.isSuccess && result.data != null
        ? ShippingSetupsListLoaded(result.data!)
        : ShippingSetupsListError(result.error ?? 'Unknown error');
    state = newState;
  }

  /// Load active shipping options only
  Future<void> loadActiveShippingSetups() async {
    state = const ShippingSetupsListLoading();
    final result = await _repository.listMyActiveShippingSetups();

    final newState = result.isSuccess && result.data != null
        ? ShippingSetupsListLoaded(result.data!)
        : ShippingSetupsListError(result.error ?? 'Unknown error');
    state = newState;
  }

  /// Create new shipping option
  Future<String?> createShippingSetup(
    CreateShippingSetupRequest request,
  ) async {
    final result = await _repository.createShippingSetup(request);

    if (result.isSuccess && result.data != null) {
      return result.data!.id;
    } else {
      final newState = ShippingSetupsListError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return null;
    }
  }

  /// Update shipping option
  Future<bool> updateShippingSetup(
    String optionId,
    UpdateShippingSetupRequest request,
  ) async {
    final result = await _repository.updateShippingSetup(optionId, request);

    if (result.isSuccess) {
      return true;
    } else {
      final newState = ShippingSetupsListError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }

  /// Delete shipping option
  Future<bool> deleteShippingSetup(String optionId) async {
    final result = await _repository.deleteShippingSetup(optionId);

    if (result.isSuccess) {
      return true;
    }
    // Do NOT replace loaded state on delete failure — the list is still valid.
    return false;
  }

  /// Toggle active status
  Future<bool> toggleActiveStatus(String optionId, bool isActive) async {
    final result = await _repository.toggleActiveStatus(optionId, isActive);

    if (result.isSuccess) {
      return true;
    }
    // Do NOT replace loaded state on toggle failure — the list is still valid.
    return false;
  }
}

/// Notifier untuk Single Shipping Option Detail
class ShippingSetupDetailNotifier extends Notifier<ShippingSetupDetailState> {
  ShippingRepository get _repository => ref.read(shippingRepositoryProvider);

  @override
  ShippingSetupDetailState build() {
    return const ShippingSetupDetailInitial();
  }

  /// Load single shipping option by ID
  Future<void> loadOption(String optionId) async {
    state = const ShippingSetupDetailLoading();
    final result = await _repository.getShippingSetupById(optionId);

    final newState = result.isSuccess && result.data != null
        ? ShippingSetupDetailLoaded(result.data!)
        : ShippingSetupDetailError(result.error ?? 'Unknown error');
    state = newState;
  }

  /// Add coverage to current option
  Future<bool> addCoverage(String optionId, AddCoverageRequest request) async {
    final result = await _repository.addCoverage(optionId, request);

    if (result.isSuccess) {
      loadOption(optionId);
      return true;
    } else {
      final newState = ShippingSetupDetailError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }

  /// Update coverage
  Future<bool> updateCoverage(
    String coverageId,
    UpdateCoverageRequest request,
  ) async {
    final result = await _repository.updateCoverage(coverageId, request);

    if (result.isSuccess) {
      final currentState = state;
      if (currentState is ShippingSetupDetailLoaded) {
        loadOption(currentState.option.id);
      }
      return true;
    } else {
      final newState = ShippingSetupDetailError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }

  /// Delete coverage
  Future<bool> deleteCoverage(String coverageId) async {
    final result = await _repository.deleteCoverage(coverageId);

    if (result.isSuccess) {
      final currentState = state;
      if (currentState is ShippingSetupDetailLoaded) {
        loadOption(currentState.option.id);
      }
      return true;
    } else {
      final newState = ShippingSetupDetailError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }
}

// Phase 3 cleanup: DeliveryCheckNotifier and ShippingProofNotifier removed.
// Both had zero consumers — the buyer-side delivery check flow uses
// shippingRepository.checkDeliveryAvailability directly from
// auction_claim_shipping_modal.dart, and the shipping-proof upload flow runs
// through order_repository_impl.dart against the order datasource (not via
// the shipping notifier). The underlying repository methods on
// ShippingRepository / ShippingProofRepository remain available for future use.
