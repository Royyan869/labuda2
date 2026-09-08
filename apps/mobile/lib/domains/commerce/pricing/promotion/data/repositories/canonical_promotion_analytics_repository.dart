/// Canonical Promotion Analytics Repository
///
/// HTTP-based implementation for fetching canonical promotion delivery analytics.
/// Calls GET /api/v1/promotions/contracts/:id/analytics — the contract_id
/// authority projection (canonical_promotion_delivery_events).
library;

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/canonical_promotion_analytics_dto.dart';

/// Abstract interface for canonical promotion analytics.
abstract class CanonicalPromotionAnalyticsRepository {
  /// Fetches delivery analytics for a canonical promotion contract.
  ///
  /// Returns [Result.success] with analytics data if the caller owns the contract.
  /// Returns [Result.error] if:
  /// - Contract not found (404)
  /// - Caller not authorized (403 equivalent - treated as not found)
  /// - Network error
  Future<Result<CanonicalPromotionAnalyticsDto>> getDeliveryAnalytics(
    String contractId,
  );
}

/// HTTP-based implementation of [CanonicalPromotionAnalyticsRepository].
class CanonicalPromotionAnalyticsRepositoryImpl
    implements CanonicalPromotionAnalyticsRepository {
  final ApiClient _apiClient;

  CanonicalPromotionAnalyticsRepositoryImpl(this._apiClient);

  @override
  Future<Result<CanonicalPromotionAnalyticsDto>> getDeliveryAnalytics(
    String contractId,
  ) async {
    try {
      final response = await _apiClient.get(
        '/promotions/contracts/$contractId/analytics',
      );

      // Parse the response envelope { "success": true, "data": {...} }
      final data = response.data;
      if (data == null) {
        return Result.error('Empty response from server');
      }

      // Extract the data field from the envelope
      final analyticsData = data['data'] as Map<String, dynamic>?;
      if (analyticsData == null) {
        return Result.error('Invalid response format');
      }

      final dto = CanonicalPromotionAnalyticsDto.fromJson(analyticsData);
      return Result.success(dto);
    } on ApiException catch (e) {
      // Handle specific API errors
      if (e is NotFoundException) {
        // Contract not found or not owned by caller
        return Result.error('Promotion not found');
      }
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }
}