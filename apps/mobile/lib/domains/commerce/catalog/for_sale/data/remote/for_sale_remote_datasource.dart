/// ForSale Remote Datasource
///
/// API-based datasource using Go backend's /api/v1/for-sale endpoints —
/// the fixed-price sale channel, a sibling of Auction over Product.
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:uuid/uuid.dart';

/// ForSale remote datasource
///
/// Provides direct access to backend fixed-price forSale API endpoints.
class ForSaleRemoteDatasource extends BaseApiRepository {
  ForSaleRemoteDatasource(super.apiClient, {super.logger});

  /// Generates a UUID-based idempotency key for forSale operations
  String _generateIdempotencyKey() {
    return const Uuid().v4();
  }

  // ========================================
  // ForSale Query Operations
  // ========================================

  /// Get forSale by ID
  ///
  /// Backend API: GET /api/v1/for-sale/:id
  Future<Result<ForSaleResponseDto>> getForSale(String forSaleId) async {
    return executeRequest(
      () => apiClient.get('/for-sale/$forSaleId'),
      parser: (data) =>
          ForSaleResponseDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get multiple forSales by IDs
  ///
  /// Backend API: POST /api/v1/for-sale/batch
  /// Request body: { "ids": ["id1", "id2", ...] }
  Future<Result<List<ForSaleResponseDto>>> getForSalesByIds(
    List<String> forSaleIds,
  ) async {
    if (forSaleIds.isEmpty) {
      return Result.success([]);
    }

    return executeRequest(
      () => apiClient.post('/for-sale/batch', data: {'ids': forSaleIds}),
      parser: (data) {
        final list = data as List;
        return list
            .map(
              (item) =>
                  ForSaleResponseDto.fromJson(item as Map<String, dynamic>),
            )
            .toList();
      },
    );
  }

  /// List public forSales with pagination
  ///
  /// Backend API: GET /api/v1/for-sale
  /// Query params:
  /// - page: Page number (default: 1)
  /// - limit: Results per page (default: 20, max: 100)
  /// - seller_id: Filter by seller ID (optional)
  /// - sort: Sort order (created_at, price, etc.)
  Future<Result<ForSaleListResponseDto>> listForSales({
    int page = 1,
    int limit = 20,
    String? sellerId,
    String? search,
    int? priceMin,
    int? priceMax,
    String? sortBy,
  }) async {
    // Build query parameters
    final queryParams = <String, dynamic>{
      'page': page,
      'limit': limit.clamp(1, 100),
      if (sellerId != null) 'seller_id': sellerId,
      if (search != null && search.isNotEmpty) 'q': search,
      if (priceMin != null) 'price_min': priceMin,
      if (priceMax != null) 'price_max': priceMax,
      if (sortBy != null) 'sort': sortBy,
    };

    return executeRequest(
      () => apiClient.get('/for-sale', queryParameters: queryParams),
      parser: (data) =>
          ForSaleListResponseDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Search forSales
  ///
  /// Backend API: GET /api/v1/search/for-sale
  /// Query params:
  /// - q: Search query (required)
  /// - price_min: Minimum price (optional)
  /// - price_max: Maximum price (optional)
  /// - variety: Koi variety filter (optional)
  /// - seller_id: Filter by seller ID (optional)
  /// - cursor: Pagination cursor (optional)
  /// - limit: Results per page (default: 20, max: 100)
  Future<Result<ForSaleListResponseDto>> searchForSales({
    required String query,
    int? priceMin,
    int? priceMax,
    String? variety,
    String? sellerId,
    String? cursor,
    int limit = 20,
    String? sortBy,
    String? sortDir,
  }) async {
    final queryParams = <String, dynamic>{
      'q': query,
      'limit': limit.clamp(1, 100),
      if (priceMin != null) 'price_min': priceMin,
      if (priceMax != null) 'price_max': priceMax,
      if (variety != null) 'variety': variety,
      if (sellerId != null) 'seller_id': sellerId,
      if (cursor != null) 'cursor': cursor,
      if (sortBy != null) 'sort': sortBy,
      if (sortDir != null) 'sort_dir': sortDir,
    };

    return executeRequest(
      () => apiClient.get('/search/for-sale', queryParameters: queryParams),
      parser: (data) =>
          ForSaleListResponseDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get seller's forSales
  ///
  /// Backend API: GET /api/v1/for-sale?seller_id=:sellerId
  Future<Result<ForSaleListResponseDto>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int limit = 20,
  }) async {
    return listForSales(page: page, limit: limit, sellerId: sellerId);
  }

  // ========================================
  // ForSale Mutations
  // ========================================

  /// Create a new forSale
  ///
  /// Backend API: POST /api/v1/for-sale
  /// Includes Idempotency-Key header for duplicate prevention on retries
  Future<Result<ForSaleResponseDto>> createForSale(
    CreateForSaleRequestDto request,
  ) async {
    final idempotencyKey = _generateIdempotencyKey();
    return executeRequest(
      () => apiClient.post(
        '/for-sale',
        data: request.toJson(),
        options: Options(headers: {'Idempotency-Key': idempotencyKey}),
      ),
      parser: (data) =>
          ForSaleResponseDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Update a forSale
  ///
  /// Backend API: PUT /api/v1/for-sale/:id
  Future<Result<ForSaleResponseDto>> updateForSale(
    String forSaleId,
    UpdateForSaleRequestDto request,
  ) async {
    return executeRequest(
      () => apiClient.put('/for-sale/$forSaleId', data: request.toJson()),
      parser: (data) =>
          ForSaleResponseDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Delete/withdraw a forSale
  ///
  /// Backend API: DELETE /api/v1/for-sale/:id
  Future<Result<void>> deleteForSale(String forSaleId) async {
    return executeVoidRequest(() => apiClient.delete('/for-sale/$forSaleId'));
  }
}
