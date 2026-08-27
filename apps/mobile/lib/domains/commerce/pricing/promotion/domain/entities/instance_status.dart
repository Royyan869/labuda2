/// Instance status enum.
///
/// Lifecycle: inactive -> active -> paused/expired/cancelled
enum InstanceStatus {
  /// Instance was created but not yet activated
  inactive('inactive'),

  /// Instance is currently promoting the target
  active('active'),

  /// Instance was paused by user (can be resumed)
  paused('paused'),

  /// Instance duration was exhausted
  expired('expired'),

  /// Instance was cancelled
  cancelled('cancelled');

  const InstanceStatus(this.value);

  /// The API value for this enum
  final String value;

  /// Parses instance status from string
  static InstanceStatus fromString(String value) {
    return InstanceStatus.values.firstWhere(
      (status) => status.value == value,
      orElse: () => InstanceStatus.inactive,
    );
  }
}

/// Extension for InstanceStatus utility methods
extension InstanceStatusX on InstanceStatus {
  /// Whether this is a terminal state (no further transitions possible)
  bool get isTerminal =>
      this == InstanceStatus.expired || this == InstanceStatus.cancelled;

  /// Whether the instance is currently promoting
  bool get isActive => this == InstanceStatus.active;

  /// Whether the instance can be activated
  bool get canActivate =>
      this == InstanceStatus.inactive || this == InstanceStatus.paused;
}
