/// Promotion Discovery Service (Phase 4)
/// Promotion Purchase Service (Phase 5)
///
/// Service for:
/// - Fetching promoted items for discovery surfaces (read-only)
/// - Purchasing promotion packages (with billing/payment integration)
library;

import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_dto.dart';

/// Service for fetching promoted items for discovery surfaces
class PromotionDiscoveryService {
  final ApiClient _apiClient;

  PromotionDiscoveryService(this._apiClient);

  /// Get all promoted items for discovery surfaces
  /// [limit] Maximum number of items to return (default: 10, max: 50)
  Future<PromotedItemsResponse> getPromotedItems({int limit = 10}) async {
    try {
      final queryParams = <String, String>{
        'limit': limit.clamp(1, 50).toString(),
      };

      final response = await _apiClient.get(
        '/promotions/discover',
        queryParameters: queryParams,
      );

      return PromotedItemsResponse.fromJson(
        response.data as Map<String, dynamic>,
      );
    } catch (e) {
      // Return empty on error - promotion is not critical for app functionality
      return PromotedItemsResponse.empty;
    }
  }

  /// Get promoted items by target type.
  /// [targetType] Type of promoted items ('fixed_price_sale', 'auction', 'external_product')
  /// [limit] Maximum number of items to return (default: 10, max: 50)
  Future<PromotedItemsResponse> getPromotedItemsByType({
    required String targetType,
    int limit = 10,
  }) async {
    try {
      final queryParams = <String, String>{
        'limit': limit.clamp(1, 50).toString(),
      };

      final response = await _apiClient.get(
        '/promotions/discover/$targetType',
        queryParameters: queryParams,
      );

      return PromotedItemsResponse.fromJson(
        response.data as Map<String, dynamic>,
      );
    } catch (e) {
      // Return empty on error - promotion is not critical for app functionality
      return PromotedItemsResponse.empty;
    }
  }

  /// Get promoted fixed-price-sale items only
  Future<PromotedItemsResponse> getPromotedFixedPriceSales({int limit = 10}) {
    return getPromotedItemsByType(targetType: 'fixed_price_sale', limit: limit);
  }

  /// Get promoted auctions only
  Future<PromotedItemsResponse> getPromotedAuctions({int limit = 10}) {
    return getPromotedItemsByType(targetType: 'auction', limit: limit);
  }

  /// Check if a specific target is currently promoted
  /// This is useful for adding "Promoted" badges to detail pages
  Future<bool> isTargetPromoted({
    required String targetType,
    required String targetId,
  }) async {
    try {
      final promotedItems = await getPromotedItemsByType(
        targetType: targetType,
        limit: 50, // Get more to check if target is promoted
      );

      return promotedItems.promotedItems.any(
        (item) => item.targetId == targetId,
      );
    } catch (e) {
      return false;
    }
  }

  // ==========================================================================
  // PROMOTION PURCHASE METHODS (Phase 5)
  // ==========================================================================

  /// Get available promotion packages
  /// [includeInactive] Whether to include inactive packages (default: false)
  Future<List<PromotionPackageDto>> getPackages({
    bool includeInactive = false,
  }) async {
    try {
      final queryParams = <String, String>{
        'include_inactive': includeInactive.toString(),
      };

      final response = await _apiClient.get<Map<String, dynamic>>(
        '/promotions/packages',
        queryParameters: queryParams,
      );

      final data = response.data;
      if (data == null) return [];

      final packagesList = data['packages'] as List<dynamic>? ?? [];
      return packagesList
          .map(
            (item) =>
                PromotionPackageDto.fromJson(item as Map<String, dynamic>),
          )
          .toList();
    } catch (e) {
      // Return empty list on error
      return [];
    }
  }

  /// Purchase a promotion package
  ///
  /// PURCHASE TRUTH ENFORCEMENT:
  /// - Client provides ONLY package_id
  /// - Server derives price from package data
  /// - Creates billing transaction for payment
  ///
  /// Returns billing ID which should be used to initiate billing payment
  /// via POST /api/v1/payments/billing
  Future<PurchasePromotionPackageResponseDto> purchasePackage({
    required String packageId,
  }) async {
    final request = PurchasePromotionPackageRequestDto(packageId: packageId);

    final response = await _apiClient.post<Map<String, dynamic>>(
      '/promotions/packages/purchase',
      data: request.toJson(),
    );

    final data = response.data;
    if (data == null) {
      throw Exception('Empty response from server');
    }

    return PurchasePromotionPackageResponseDto.fromJson(data);
  }
}
