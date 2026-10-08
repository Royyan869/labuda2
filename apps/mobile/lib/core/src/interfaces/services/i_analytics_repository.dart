import 'package:labuda/core/common/result.dart';

/// Canonical product-analytics authority for Labuda mobile.
///
/// ONE sink for product/behaviour/business measurement. Features MUST emit
/// through this interface and MUST NOT call the Firebase SDK directly.
///
/// Boundary: this interface measures what users DO. It is NOT crash reporting,
/// performance monitoring, or diagnostic logging, and it is NEVER the authority
/// for business/financial state.
///
/// Event names and parameter keys are owned by the canonical taxonomy in
/// `core/observability/analytics_events.dart`. Screen names are owned by
/// `core/observability/screen_names.dart`.
abstract class IAnalyticsRepository {
  /// Log a canonical product event.
  ///
  /// **Parameters:**
  /// - [eventName]: canonical event name (see `AnalyticsEvents`).
  /// - [parameters]: additional event parameters.
  /// - [userId]: the acting user's id, if any. Sets the SDK user id.
  ///
  /// **Returns:** [Result<void>] success or error.
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  });

  /// Log a screen view using the canonical screen taxonomy.
  ///
  /// [screenName] MUST be a static product concept (e.g. `profile`,
  /// `product_detail`) — never a user/resource identifier.
  ///
  /// **Returns:** [Result<void>] success or error.
  Future<Result<void>> logScreenView({
    required String screenName,
    String? screenClass,
  });
}
