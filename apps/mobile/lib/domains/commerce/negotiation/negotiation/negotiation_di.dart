/// Negotiation Refactor DI Helper
///
/// Helper functions for initializing the negotiation_refactor module.
library;

import 'package:labuda/core/core.dart';

/// Negotiation Refactor DI
///
/// Helper class for initializing negotiation_refactor module dependencies.
class NegotiationRefactorDI {
  /// Create provider overrides for negotiation_refactor
  ///
  /// Use this in ProviderScope to provide ApiClient:
  /// ```dart
  /// ProviderScope(
  ///   overrides: NegotiationRefactorDI.overrides(),
  ///   child: MyApp(),
  /// )
  /// ```
  static List overrides({ApiClient? apiClient, ILoggerService? logger}) {
    return [
      if (apiClient != null) apiClientProvider.overrideWithValue(apiClient),
      if (logger != null) loggerServiceProvider.overrideWithValue(logger),
    ];
  }
}
