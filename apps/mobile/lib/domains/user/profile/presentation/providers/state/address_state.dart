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

  /// True while a refresh is in flight with last-known-good addresses still
  /// present. Drives the inline update indicator, never a full-page swap.
  final bool isRefreshing;

  /// Mutation failure message (add/edit/delete/set-primary). Consumed by the
  /// screen as a controlled SnackBar — raw backend text is never rendered.
  final String? errorMessage;

  /// Load/refresh failure while [addresses] still holds the last-known-good
  /// collection. Drives the inline refresh-error banner. Distinct from
  /// [errorMessage] so a refresh failure is never confused with a mutation
  /// failure (and vice versa).
  final String? refreshError;

  const AddressState({
    this.addresses = const AsyncValue.data([]),
    this.primaryAddress = const AsyncValue.data(null),
    this.isSaving = false,
    this.isDeleting = false,
    this.isRefreshing = false,
    this.errorMessage,
    this.refreshError,
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
    bool? isRefreshing,
    String? errorMessage,
    String? refreshError,
  }) {
    return AddressState(
      addresses: addresses ?? this.addresses,
      primaryAddress: primaryAddress ?? this.primaryAddress,
      isSaving: isSaving ?? this.isSaving,
      isDeleting: isDeleting ?? this.isDeleting,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      errorMessage: errorMessage,
      refreshError: refreshError,
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
          isRefreshing == other.isRefreshing &&
          errorMessage == other.errorMessage &&
          refreshError == other.refreshError;

  @override
  int get hashCode => Object.hash(
    addresses,
    primaryAddress,
    isSaving,
    isDeleting,
    isRefreshing,
    errorMessage,
    refreshError,
  );
}
