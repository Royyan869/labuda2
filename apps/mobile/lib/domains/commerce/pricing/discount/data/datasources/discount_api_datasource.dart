/// Discount API Datasource (Go Backend)
///
/// Implements Go API endpoints for discount operations.
/// This datasource communicates with the Go backend via REST API.
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import '../dto/discount_dto.dart';
import '../models/discount_model.dart';

/// API Remote datasource for Discount using Go Backend
///
/// Canonical backend endpoints (migration 000204 required for seller-owned discount targets):
/// - GET    /api/v1/discounts/code/{code}
/// - GET    /api/v1/discounts/seller/{sellerId}
/// - GET    /api/v1/discounts/active
/// - POST   /api/v1/discounts/validate       ← UX authority for code validation
/// - POST   /api/v1/discounts
/// - PUT    /api/v1/discounts/{discountId}
/// - DELETE /api/v1/discounts/{discountId}   ← soft-deactivate (sets is_active=false)
///
/// REMOVED (were 404 in backend):
/// - GET /discounts/{id}           → fetch from seller list or create response instead
/// - GET /discounts/seller/{id}/active → filter locally from GET /seller/{id}
/// - PUT /discounts/{id}/deactivate → use DELETE /discounts/{id} (same soft-delete)
/// - GET /discounts/{id}/usage/{userId}     → server-side only; never expose
/// - POST /discounts/{id}/usage             → server-side only; never expose
/// - POST /discounts/{id}/usage/increment   → server-side only; never expose
/// - GET /discounts/{id}/statistics         → P3 future feature; not yet implemented
class DiscountApiDatasource {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  static const String _basePath = '/discounts';

  DiscountApiDatasource(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  // ============================================================
  // HELPER METHODS
  // ============================================================

  /// Execute request and return Result with data or error
  Future<Result<T>> _executeRequest<T>({
    required Future<Response<dynamic>> Function() request,
    required T Function(dynamic data) parser,
  }) async {
    try {
      final response = await request();
      final data = response.data;

      if (data is Map<String, dynamic>) {
        // Handle standard API response with success field
        if (data['success'] == false && data['error'] != null) {
          final error = data['error'] as Map<String, dynamic>?;
          return Result.error(
            error?['message']?.toString() ?? 'Request failed',
          );
        }

        // Parse the data field if available, otherwise use entire response
        final parsedData = data['data'] ?? data;
        return Result.success(parser(parsedData));
      }

      // Direct data (not wrapped in standard format)
      return Result.success(parser(data));
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'Discount API request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      return Result.error(exception.message);
    } catch (e, stackTrace) {
      _logger?.error(
        'Unexpected error in Discount API: $e',
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Execute void request (no return data)
  Future<Result<void>> _executeVoidRequest({
    required Future<Response<dynamic>> Function() request,
  }) async {
    try {
      await request();
      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'Discount API request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      return Result.error(exception.message);
    } catch (e, stackTrace) {
      _logger?.error(
        'Unexpected error in Discount API: $e',
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  // ============================================================
  // DISCOUNT OPERATIONS (Go API)
  // ============================================================

  /// Get discount by code via Go API
  ///
  /// GET /api/v1/discounts/code/{code}
  Future<Result<DiscountModel?>> getDiscountByCode(String code) async {
    return _executeRequest<DiscountModel?>(
      request: () => _apiClient.get('$_basePath/code/$code'),
      parser: (data) {
        if (data == null) return null;
        final dto = DiscountResponseDto.fromJson(data as Map<String, dynamic>);
        return DiscountModel.fromEntity(dto.toEntity());
      },
    );
  }

  /// Get all discounts for seller via Go API
  ///
  /// GET /api/v1/discounts/seller/{sellerId}
  ///
  /// Returns ALL discounts (active + expired + inactive).
  /// Filter active/expired/inactive locally on the client.
  Future<Result<List<DiscountModel>>> getSellerDiscounts(
    String sellerId,
  ) async {
    return _executeRequest<List<DiscountModel>>(
      request: () => _apiClient.get('$_basePath/seller/$sellerId'),
      parser: (data) {
        if (data is List) {
          return data
              .map(
                (e) => DiscountModel.fromEntity(
                  DiscountResponseDto.fromJson(
                    e as Map<String, dynamic>,
                  ).toEntity(),
                ),
              )
              .toList();
        }
        // Handle wrapped response
        final list = data['discounts'] as List? ?? [];
        return list
            .map(
              (e) => DiscountModel.fromEntity(
                DiscountResponseDto.fromJson(
                  e as Map<String, dynamic>,
                ).toEntity(),
              ),
            )
            .toList();
      },
    );
  }

  /// Validate discount code via Go API
  ///
  /// POST /api/v1/discounts/validate
  ///
  /// Backend validates: code existence, active status, time window,
  /// usage limits, minimum purchase. Returns discount object if valid.
  ///
  /// [subtotalCents] is the order subtotal in the smallest currency unit (IDR).
  ///
  /// Context and target validation is enforced by the backend at validation time.
  Future<Result<DiscountModel>> validateDiscountCode({
    required String code,
    required int subtotalCents,
    required String contextType,
    required String sellerId,
    String? listingId,
    String? auctionId,
  }) async {
    return _executeRequest<DiscountModel>(
      request: () => _apiClient.post(
        '$_basePath/validate',
        data: {
          'code': code,
          'subtotal': subtotalCents,
          'context_type': contextType,
          'seller_id': sellerId,
          ...?(listingId == null ? null : {'listing_id': listingId}),
          ...?(auctionId == null ? null : {'auction_id': auctionId}),
        },
      ),
      parser: (data) {
        final discountJson = data['discount'] as Map<String, dynamic>;
        final dto = DiscountResponseDto.fromJson(discountJson);
        return DiscountModel.fromEntity(dto.toEntity());
      },
    );
  }

  /// Create discount via Go API
  ///
  /// POST /api/v1/discounts
  Future<Result<DiscountModel>> createDiscount(DiscountModel discount) async {
    final requestDto = CreateDiscountRequestDto(
      code: discount.code,
      description: discount.description,
      type: DiscountTypeDto.fromEntity(discount.type),
      value: discount.value,
      minPurchase: discount.minPurchase,
      maxDiscount: discount.maxDiscount,
      maxUsagePerUser: discount.maxUsagePerUser,
      totalUsageLimit: discount.totalUsageLimit,
      appliesTo: DiscountAppliesToDto.fromEntity(discount.appliesTo),
      targetMode: DiscountTargetModeDto.fromEntity(discount.targetMode),
      sellerId: discount.sellerId,
      applicableListingIds: discount.applicableListingIds,
      applicableAuctionIds: discount.applicableAuctionIds,
      validFrom: discount.validFrom,
      validUntil: discount.validUntil,
    );

    return _executeRequest<DiscountModel>(
      request: () => _apiClient.post(_basePath, data: requestDto.toJson()),
      parser: (data) {
        final dto = DiscountResponseDto.fromJson(data as Map<String, dynamic>);
        return DiscountModel.fromEntity(dto.toEntity());
      },
    );
  }

  /// Update discount via Go API
  ///
  /// PUT /api/v1/discounts/{discountId}
  Future<Result<void>> updateDiscount(DiscountModel discount) async {
    final requestDto = UpdateDiscountRequestDto.fromEntity(discount);

    return _executeVoidRequest(
      request: () => _apiClient.put(
        '$_basePath/${discount.id}',
        data: requestDto.toJson(),
      ),
    );
  }

  /// Delete (soft-deactivate) discount via Go API
  ///
  /// DELETE /api/v1/discounts/{discountId}
  ///
  /// Backend sets is_active=false (soft delete). Discount history is preserved.
  Future<Result<void>> deleteDiscount(String discountId) async {
    return _executeVoidRequest(
      request: () => _apiClient.delete('$_basePath/$discountId'),
    );
  }
}
