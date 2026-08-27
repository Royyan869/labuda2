import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';

/// Service for fetching seller upgrade configuration
///
/// FIRESTORE SUNSET (2025-02-20): Firestore removed.
/// Now fetches the backend-authoritative seller subscription config.
class SellerUpgradeConfigService {
  final ApiClient _apiClient;
  SellerUpgradeConfigEntity? _cachedConfig;

  SellerUpgradeConfigService({required ApiClient apiClient})
    : _apiClient = apiClient;

  /// Get configuration from cache or backend.
  ///
  /// The backend is the payment authority, so the disclosure step must read
  /// from the server contract rather than a mobile fallback.
  Future<SellerUpgradeConfigEntity> getConfiguration() async {
    if (_cachedConfig != null) {
      return _cachedConfig!;
    }

    try {
      final response = await _apiClient.get('/seller/subscription/config');
      final data = response.data;
      final configJson = _extractConfigJson(data);
      if (configJson == null) {
        throw ApiExceptionFactory.fromStatusCode(
          response.statusCode ?? 500,
          'Seller subscription config response missing config payload',
        );
      }

      _cachedConfig = SellerUpgradeConfigEntity.fromJson(configJson);
      return _cachedConfig!;
    } on DioException catch (e) {
      if (e.error is ApiException) {
        throw e.error as ApiException;
      }
      rethrow;
    }
  }

  /// Watch configuration for updates (stream)
  ///
  /// Backend-backed disclosure is a one-shot fetch for now.
  Stream<SellerUpgradeConfigEntity> watchConfiguration() {
    return Stream.fromFuture(getConfiguration());
  }

  /// Refresh configuration (clear cache)
  Future<SellerUpgradeConfigEntity> refreshConfig() async {
    _cachedConfig = null;
    return getConfiguration();
  }

  /// Clear cached configuration
  void clearCache() {
    _cachedConfig = null;
  }

  /// Update configuration (admin only)
  ///
  /// FIRESTORE SUNSET: This method is no longer supported.
  /// Use Backend API to update configuration.
  Future<void> updateConfiguration(
    SellerUpgradeConfigEntity config,
    String adminUserId,
  ) async {
    throw UnimplementedError(
      'Configuration update moved to Backend API. Use Backend API.',
    );
  }

  /// Reset to default configuration (admin only)
  ///
  /// FIRESTORE SUNSET: This method is no longer supported.
  /// Use Backend API to reset configuration.
  Future<void> resetToDefaults(String adminUserId) async {
    throw UnimplementedError(
      'Configuration reset moved to Backend API. Use Backend API.',
    );
  }

  Map<String, dynamic>? _extractConfigJson(dynamic data) {
    if (data is Map<String, dynamic>) {
      final nestedConfig = data['config'];
      if (nestedConfig is Map<String, dynamic>) {
        return nestedConfig;
      }

      final nestedData = data['data'];
      if (nestedData is Map<String, dynamic>) {
        final innerConfig = nestedData['config'];
        if (innerConfig is Map<String, dynamic>) {
          return innerConfig;
        }
        return nestedData;
      }

      return data;
    }
    return null;
  }
}
