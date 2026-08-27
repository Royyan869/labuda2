/// Analytics Event Entity
///
/// Represents an analytics event to be tracked.
/// Pure domain object - no Firebase dependencies.
///
/// GUIDELINES compliant:
/// - Pure Dart entity ✅
/// - No external dependencies ✅
class AnalyticsEvent {
  final String name;
  final Map<String, dynamic>? parameters;
  final String? userId;
  final DateTime timestamp;

  AnalyticsEvent({
    required this.name,
    this.parameters,
    this.userId,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  /// Business logic: Check if event is valid
  bool get isValid {
    // Event name must not be empty
    if (name.trim().isEmpty) return false;

    // Event name must be <= 40 characters (Firebase limit)
    if (name.length > 40) return false;

    // Parameters count must be <= 25 (Firebase limit)
    if (parameters != null && parameters!.length > 25) return false;

    return true;
  }

  /// Business logic: Get sanitized event name (lowercase, no spaces)
  String get sanitizedName {
    return name.toLowerCase().replaceAll(' ', '_');
  }

  @override
  String toString() {
    return 'AnalyticsEvent(name: $name, userId: $userId, parameters: $parameters)';
  }
}
