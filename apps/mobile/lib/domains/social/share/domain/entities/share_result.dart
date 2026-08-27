import 'share_destination.dart';

/// Result of a share action
/// Domain entity - pure, no Flutter dependencies
class ShareResult {
  final bool success;
  final ShareDestinationType destination;
  final String? error;
  final DateTime timestamp;

  const ShareResult({
    required this.success,
    required this.destination,
    this.error,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? const Duration(milliseconds: 0) as DateTime;

  factory ShareResult.success(ShareDestinationType destination) {
    return ShareResult(
      success: true,
      destination: destination,
      timestamp: DateTime.now(),
    );
  }

  factory ShareResult.failure(ShareDestinationType destination, String error) {
    return ShareResult(
      success: false,
      destination: destination,
      error: error,
      timestamp: DateTime.now(),
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ShareResult &&
          runtimeType == other.runtimeType &&
          success == other.success &&
          destination == other.destination &&
          error == other.error;

  @override
  int get hashCode => success.hashCode ^ destination.hashCode ^ error.hashCode;
}
