/// Shipping Refactor DI Helper
///
/// Helper functions for initializing the shipping_refactor module.
library;

import 'package:labuda/core/providers/core_providers.dart';

/// Shipping Refactor DI
///
/// Helper class for initializing shipping_refactor module dependencies.
class ShippingRefactorDI {
  /// Create provider overrides for shipping_refactor
  ///
  /// Use this in ProviderScope to provide ApiClient:
  /// ```dart
  /// ProviderScope(
  ///   overrides: ShippingRefactorDI.overrides(
  ///     apiClient: ApiDI.apiClient,
  ///   ),
  ///   child: MyApp(),
  /// )
  /// ```
  static List overrides({required ApiClient apiClient}) {
    return [apiClientProvider.overrideWithValue(apiClient)];
  }
}
