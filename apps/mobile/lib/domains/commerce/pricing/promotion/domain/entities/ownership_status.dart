/// Ownership status enum.
///
/// Lifecycle: available -> consumed/expired/cancelled
enum OwnershipStatus {
  /// Ownership is ready to use
  available('available'),

  /// Duration has been fully consumed
  consumed('consumed'),

  /// Validity window has expired
  expired('expired'),

  /// Ownership was cancelled
  cancelled('cancelled');

  const OwnershipStatus(this.value);

  /// The API value for this enum
  final String value;

  /// Parses ownership status from string
  static OwnershipStatus fromString(String value) {
    return OwnershipStatus.values.firstWhere(
      (status) => status.value == value,
      orElse: () => OwnershipStatus.available,
    );
  }
}

/// Extension for OwnershipStatus utility methods
extension OwnershipStatusX on OwnershipStatus {
  /// Whether this is a terminal state (no further transitions possible)
  bool get isTerminal =>
      this == OwnershipStatus.consumed ||
      this == OwnershipStatus.expired ||
      this == OwnershipStatus.cancelled;

  /// Whether this ownership can be used to activate a promotion
  bool get canActivate => this == OwnershipStatus.available;
}
