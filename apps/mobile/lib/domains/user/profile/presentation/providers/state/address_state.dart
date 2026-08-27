import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Address Application State
///
/// Immutable state container for address feature.
/// Phase 5: Application Layer
///
/// Rules:
/// - Immutable (all fields final)
/// - No BuildContext stored
/// - No UI state (scroll, focus, etc.)
class AddressState {
  final AsyncValue<List<AddressEntity>> addresses;
  final AsyncValue<AddressEntity?> primaryAddress;
  final bool isSaving;
  final bool isDeleting;
  final String? errorMessage;

  const AddressState({
    this.addresses = const AsyncValue.data([]),
    this.primaryAddress = const AsyncValue.data(null),
    this.isSaving = false,
    this.isDeleting = false,
    this.errorMessage,
  });

  /// Initial state
  static const initial = AddressState();

  /// Loading state helper
  static AddressState loading() => const AddressState(
    addresses: AsyncValue.loading(),
    primaryAddress: AsyncValue.data(null),
  );

  AddressState copyWith({
    AsyncValue<List<AddressEntity>>? addresses,
    AsyncValue<AddressEntity?>? primaryAddress,
    bool? isSaving,
    bool? isDeleting,
    String? errorMessage,
  }) {
    return AddressState(
      addresses: addresses ?? this.addresses,
      primaryAddress: primaryAddress ?? this.primaryAddress,
      isSaving: isSaving ?? this.isSaving,
      isDeleting: isDeleting ?? this.isDeleting,
      errorMessage: errorMessage,
    );
  }

  /// Convenience getters
  bool get isLoading => addresses.isLoading || isSaving || isDeleting;
  bool get hasError => addresses.hasError || errorMessage != null;
  bool get hasData => addresses.hasValue;
  bool get isBusy => isSaving || isDeleting;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is AddressState &&
          addresses == other.addresses &&
          primaryAddress == other.primaryAddress &&
          isSaving == other.isSaving &&
          isDeleting == other.isDeleting &&
          errorMessage == other.errorMessage;

  @override
  int get hashCode => Object.hash(
    addresses,
    primaryAddress,
    isSaving,
    isDeleting,
    errorMessage,
  );
}
