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

  /// Create a shipping option as ONE package (identity + destinations).
  /// Bare options without destinations are rejected by the backend gate.
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

  /// Update a shipping option as ONE package (full destination replace when
  /// destinations are provided). Editing is allowed at any time — orders keep
  /// their checkout snapshot.
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

  /// Delete shipping option. Refused by the backend while the option is
  /// linked to any listing — the seller must deactivate instead.
  Future<bool> deleteShippingSetup(String optionId) async {
    final result = await _repository.deleteShippingSetup(optionId);

    if (result.isSuccess) {
      return true;
    }
    // Do NOT replace loaded state on delete failure — the list is still valid.
    return false;
  }

  /// Toggle active status (canonical retire/restore path).
  Future<bool> toggleActiveStatus(String optionId, bool isActive) async {
    final result = await _repository.toggleActiveStatus(optionId, isActive);

    if (result.isSuccess) {
      return true;
    }
    // Do NOT replace loaded state on toggle failure — the list is still valid.
    return false;
  }
}

// KILLED DESIGN: ShippingSetupDetailNotifier (add/update/delete coverage)
// removed with the per-coverage CRUD contract. Destinations are authored
// inside the one-package setup screen and saved via create/update package.
