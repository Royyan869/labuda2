import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

// =====================================
// Shipping Options List States
// =====================================

/// Base state for shipping options list
abstract class ShippingSetupsListState {
  const ShippingSetupsListState();
}

class ShippingSetupsListInitial extends ShippingSetupsListState {
  const ShippingSetupsListInitial();
}

class ShippingSetupsListLoading extends ShippingSetupsListState {
  const ShippingSetupsListLoading();
}

class ShippingSetupsListLoaded extends ShippingSetupsListState {
  final List<ShippingSetup> options;
  const ShippingSetupsListLoaded(this.options);
}

class ShippingSetupsListError extends ShippingSetupsListState {
  final String message;
  const ShippingSetupsListError(this.message);
}

// KILLED DESIGN: ShippingSetupDetailState family removed with the
// per-coverage CRUD contract. The one-package setup screen edits a full
// package and persists via create/update package.

// Phase 3 cleanup: DeliveryCheckState* and ShippingProofState* families
// removed. Both were exclusively consumed by their dedicated notifiers
// (DeliveryCheckNotifier, ShippingProofNotifier), which have also been
// removed as orphan code. The repository and entity definitions remain
// available for direct use.
