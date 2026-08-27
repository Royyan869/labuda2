import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'shipping_state.dart';
import 'providers.dart';
import '../../domain/domain.dart';

/// Notifier untuk Shipping Options Management
/// Menggunakan Riverpod Notifier (bukan StateNotifier untuk menghindari masalah compatibility)
class ShippingNotifier extends Notifier<ShippingOptionsListState> {
  ShippingRepository get _repository => ref.read(shippingRepositoryProvider);

  @override
  ShippingOptionsListState build() {
    return const ShippingOptionsListInitial();
  }

  /// Load all shipping options for a seller
  Future<void> loadShippingOptions() async {
    state = const ShippingOptionsListLoading();
    final result = await _repository.listMyShippingOptions();

    final newState = result.isSuccess && result.data != null
        ? ShippingOptionsListLoaded(result.data!)
        : ShippingOptionsListError(result.error ?? 'Unknown error');
    state = newState;
  }

  /// Load active shipping options only
  Future<void> loadActiveShippingOptions() async {
    state = const ShippingOptionsListLoading();
    final result = await _repository.listMyActiveShippingOptions();

    final newState = result.isSuccess && result.data != null
        ? ShippingOptionsListLoaded(result.data!)
        : ShippingOptionsListError(result.error ?? 'Unknown error');
    state = newState;
  }

  /// Create new shipping option
  Future<String?> createShippingOption(
    CreateShippingOptionRequest request,
  ) async {
    final result = await _repository.createShippingOption(request);

    if (result.isSuccess && result.data != null) {
      return result.data!.id;
    } else {
      final newState = ShippingOptionsListError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return null;
    }
  }

  /// Update shipping option
  Future<bool> updateShippingOption(
    String optionId,
    UpdateShippingOptionRequest request,
  ) async {
    final result = await _repository.updateShippingOption(optionId, request);

    if (result.isSuccess) {
      return true;
    } else {
      final newState = ShippingOptionsListError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }

  /// Delete shipping option
  Future<bool> deleteShippingOption(String optionId) async {
    final result = await _repository.deleteShippingOption(optionId);

    if (result.isSuccess) {
      return true;
    } else {
      final newState = ShippingOptionsListError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }

  /// Toggle active status
  Future<bool> toggleActiveStatus(String optionId, bool isActive) async {
    final result = await _repository.toggleActiveStatus(optionId, isActive);

    if (result.isSuccess) {
      return true;
    } else {
      final newState = ShippingOptionsListError(
        result.error ?? 'Unknown error',
      );
      state = newState;
      return false;
    }
  }
}

/// Notifier untuk Single Shipping Option Detail
class ShippingOptionDetailNotifier extends Notifier<ShippingOptionDetailState> {
  ShippingRepository get _repository => ref.read(shippingRepositoryProvider);

  @override
  ShippingOptionDetailState build() {
    return const ShippingOptionDetailInitial();
  }

  /// Load single shipping option by ID
  Future<void> loadOption(String optionId) async {
    state = const ShippingOptionDetailLoading();
    final result = await _repository.getShippingOptionById(optionId);

    final newState = result.isSuccess && result.data != null
        ? ShippingOptionDetailLoaded(result.data!)
        : ShippingOptionDetailError(result.error ?? 'Unknown error');
    state = newState;
  }

  /// Add coverage to current option
  Future<bool> addCoverage(String optionId, AddCoverageRequest request) async {
    final result = await _repository.addCoverage(optionId, request);

    if (result.isSuccess) {
      loadOption(optionId);
      return true;
    } else {
      final newState = ShippingOptionDetailError(
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
      if (currentState is ShippingOptionDetailLoaded) {
        loadOption(currentState.option.id);
      }
      return true;
    } else {
      final newState = ShippingOptionDetailError(
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
      if (currentState is ShippingOptionDetailLoaded) {
        loadOption(currentState.option.id);
      }
      return true;
    } else {
      final newState = ShippingOptionDetailError(
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
