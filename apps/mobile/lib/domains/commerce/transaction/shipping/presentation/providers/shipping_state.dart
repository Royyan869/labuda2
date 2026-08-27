import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

// =====================================
// Shipping Options List States
// =====================================

/// Base state for shipping options list
abstract class ShippingOptionsListState {
  const ShippingOptionsListState();
}

class ShippingOptionsListInitial extends ShippingOptionsListState {
  const ShippingOptionsListInitial();
}

class ShippingOptionsListLoading extends ShippingOptionsListState {
  const ShippingOptionsListLoading();
}

class ShippingOptionsListLoaded extends ShippingOptionsListState {
  final List<ShippingOption> options;
  const ShippingOptionsListLoaded(this.options);
}

class ShippingOptionsListError extends ShippingOptionsListState {
  final String message;
  const ShippingOptionsListError(this.message);
}

// =====================================
// Shipping Option Detail States
// =====================================

abstract class ShippingOptionDetailState {
  const ShippingOptionDetailState();
}

class ShippingOptionDetailInitial extends ShippingOptionDetailState {
  const ShippingOptionDetailInitial();
}

class ShippingOptionDetailLoading extends ShippingOptionDetailState {
  const ShippingOptionDetailLoading();
}

class ShippingOptionDetailLoaded extends ShippingOptionDetailState {
  final ShippingOption option;
  const ShippingOptionDetailLoaded(this.option);
}

class ShippingOptionDetailError extends ShippingOptionDetailState {
  final String message;
  const ShippingOptionDetailError(this.message);
}

// Phase 3 cleanup: DeliveryCheckState* and ShippingProofState* families
// removed. Both were exclusively consumed by their dedicated notifiers
// (DeliveryCheckNotifier, ShippingProofNotifier), which have also been
// removed as orphan code. The repository and entity definitions remain
// available for direct use.
