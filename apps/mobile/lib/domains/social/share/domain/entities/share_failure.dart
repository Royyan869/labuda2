/// Failure types for share operations
/// Domain entity - pure, no dependencies
class ShareFailure {
  final ShareFailureType type;
  final String message;

  const ShareFailure({required this.type, required this.message});

  factory ShareFailure.invalidTarget(String message) {
    return ShareFailure(type: ShareFailureType.invalidTarget, message: message);
  }

  factory ShareFailure.invalidDestination(String message) {
    return ShareFailure(
      type: ShareFailureType.invalidDestination,
      message: message,
    );
  }

  factory ShareFailure.network(String message) {
    return ShareFailure(type: ShareFailureType.network, message: message);
  }

  factory ShareFailure.unauthorized(String message) {
    return ShareFailure(type: ShareFailureType.unauthorized, message: message);
  }

  factory ShareFailure.unknown(String message) {
    return ShareFailure(type: ShareFailureType.unknown, message: message);
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ShareFailure &&
          runtimeType == other.runtimeType &&
          type == other.type &&
          message == other.message;

  @override
  int get hashCode => type.hashCode ^ message.hashCode;
}

/// Types of failures
enum ShareFailureType {
  invalidTarget,
  invalidDestination,
  network,
  unauthorized,
  unknown,
}
