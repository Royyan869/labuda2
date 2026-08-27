/// Payment Refactor DI Helper
///
/// Helper functions for initializing the payment_refactor module.
library;

import 'package:labuda/core/core.dart';

/// Payment Refactor DI
///
/// Helper class for initializing payment_refactor module dependencies.
class PaymentRefactorDI {
  /// Create provider overrides for payment_refactor
  ///
  /// Use this in ProviderScope to provide ApiClient and ILoggerService:
  /// ```dart
  /// ProviderScope(
  ///   overrides: [
  ///     ...PaymentRefactorDI.overrides(
  ///       apiClient: sl<ApiClient>(),
  ///       logger: sl<ILoggerService>(),
  ///     ),
  ///   ],
  ///   child: MyApp(),
  /// )
  /// ```
  static List overrides({
    required ApiClient apiClient,
    required ILoggerService logger,
  }) {
    return [
      apiClientProvider.overrideWithValue(apiClient),
      loggerServiceProvider.overrideWithValue(logger),
    ];
  }
}
