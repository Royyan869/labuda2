/// Checkout Repository Implementation
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/api_error_codes.dart' as api_codes;
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_response.dart';

/// Checkout Repository Interface
abstract class CheckoutRepository {
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  });
}

/// Checkout-specific exceptions for better error handling
class CheckoutException implements Exception {
  final String message;
  final String userFriendlyMessage;
  final String? code;

  CheckoutException({
    required this.message,
    required this.userFriendlyMessage,
    this.code,
  });

  @override
  String toString() => userFriendlyMessage;
}

/// Checkout Repository Implementation
///
/// Creates orders via POST /api/v1/orders
class CheckoutRepositoryImpl implements CheckoutRepository {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  CheckoutRepositoryImpl(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  @override
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async {
    // STRICT VALIDATION: productId must not be empty
    if (request.productId == null || request.productId!.isEmpty) {
      throw CheckoutException(
        message: 'productId cannot be empty',
        userFriendlyMessage:
            'ID produk tidak valid. Silakan pilih produk kembali.',
        code: 'INVALID_PRODUCT_ID',
      );
    }

    // STRICT VALIDATION: fixedPriceSaleId must not be empty
    if (request.fixedPriceSaleId.isEmpty) {
      throw CheckoutException(
        message: 'fixedPriceSaleId cannot be empty',
        userFriendlyMessage:
            'ID produk tidak valid. Silakan pilih produk kembali.',
        code: 'INVALID_LISTING_ID',
      );
    }

    // STRICT VALIDATION: pricingToken must not be empty
    // This ensures order uses the exact pricing snapshot from preview
    if (request.pricingToken.isEmpty) {
      throw CheckoutException(
        message:
            'pricingToken cannot be empty - order must use preview pricing',
        userFriendlyMessage:
            'Token harga tidak valid. Silakan refresh harga terbaru.',
        code: 'INVALID_PRICING_TOKEN',
      );
    }

    try {
      final headers = idempotencyKey != null
          ? {'Idempotency-Key': idempotencyKey}
          : null;

      final response = await _apiClient.post(
        '/orders',
        data: {
          'product_id': request.productId,
          'source_type': request.auctionId != null
              ? 'auction'
              : 'fixed_price_sale',
          'source_id': request.auctionId ?? request.fixedPriceSaleId,
          'quantity': request.quantity,
          'address_id': request.addressId,
          'pricing_token': request.pricingToken,
          if (request.useCoins != null) 'use_coins': request.useCoins,
          if (request.notes != null) 'notes': request.notes,
          // Commerce context - pass through for backend validation
          if (request.auctionId != null) 'auction_id': request.auctionId,
          if (request.negotiationId != null)
            'negotiation_id': request.negotiationId,
          if (request.shippingQuoteId != null)
            'shipping_quote_id': request.shippingQuoteId,
          if (request.shippingSetupId != null)
            'shipping_setup_id': request.shippingSetupId,
        },
        options: headers != null ? Options(headers: headers) : null,
      );

      final data = response.data;

      if (data is Map<String, dynamic>) {
        // Handle standard API response
        if (data['success'] == false) {
          final error = data['error'] as Map<String, dynamic>?;
          final errorCode = error?['code'] as String?;
          final errorMessage = error?['message']?.toString();

          throw _parseApiError(errorCode, errorMessage);
        }

        final responseData = data['data'] as Map<String, dynamic>?;
        if (responseData != null) {
          // Parse the Order entity returned by POST /orders.
          // Payment URL is NOT expected here — that comes from POST /payments.
          return CheckoutRepositoryImpl.buildCheckoutResponseFromData(
            responseData,
          );
        }
      }

      throw CheckoutException(
        message: 'Invalid response format',
        userFriendlyMessage: 'Format respons tidak valid. Silakan coba lagi.',
        code: 'INVALID_RESPONSE',
      );
    } on DioException catch (e) {
      _logger?.error(
        'Dio error creating order: ${e.message}',
        stackTrace: e.stackTrace,
      );

      if (e.response?.statusCode == 400 || e.response?.statusCode == 403) {
        // 403 carries email-gating rejections like EMAIL_VERIFICATION_REQUIRED.
        // We let `_parseApiError` preserve the API code so the call site
        // can react via state.errorCode.
        final data = e.response?.data as Map<String, dynamic>?;
        final error = data?['error'] as Map<String, dynamic>?;
        final errorCode = error?['code'] as String?;
        final errorMessage = error?['message']?.toString();

        throw _parseApiError(errorCode, errorMessage);
      }

      if (e.response?.statusCode == 404) {
        throw CheckoutException(
          message: 'Listing not found',
          userFriendlyMessage: 'Produk tidak ditemukan atau telah dihapus.',
          code: 'LISTING_NOT_FOUND',
        );
      }

      if (e.response?.statusCode == 409) {
        throw CheckoutException(
          message: 'Conflict - resource state changed',
          userFriendlyMessage:
              'Produk mungkin telah terjual atau harga berubah. Silakan refresh harga.',
          code: 'RESOURCE_CONFLICT',
        );
      }

      if (e.response?.statusCode == 422) {
        throw CheckoutException(
          message: 'Unprocessable entity - validation failed',
          userFriendlyMessage:
              'Data tidak lengkap atau tidak valid. Mohon periksa kembali formulir.',
          code: 'VALIDATION_ERROR',
        );
      }

      throw CheckoutException(
        message: 'Network error: ${e.message}',
        userFriendlyMessage:
            'Terjadi kesalahan jaringan. Silakan periksa koneksi dan coba lagi.',
        code: 'NETWORK_ERROR',
      );
    } on CheckoutException {
      rethrow;
    } catch (e, stackTrace) {
      _logger?.error('Failed to create order: $e', stackTrace: stackTrace);

      if (e is CheckoutException) {
        rethrow;
      }

      throw CheckoutException(
        message: e.toString(),
        userFriendlyMessage:
            'Terjadi kesalahan tidak terduga. Silakan coba lagi.',
        code: 'UNKNOWN_ERROR',
      );
    }
  }

  /// Validates the `data` payload of a successful POST /orders response
  /// and constructs a [CheckoutResponse], or throws a
  /// [CheckoutException] with code `CHECKOUT_INCOMPLETE_RESPONSE` if any
  /// required field is missing or invalid.
  ///
  /// Backend returns a raw Order entity. Required field: `id`.
  /// Payment URL is obtained separately via POST /payments (2-step flow).
  ///
  /// Exposed (public, no leading underscore) so the validation logic is
  /// directly unit-testable without needing to mock Dio.
  static CheckoutResponse buildCheckoutResponseFromData(
    Map<String, dynamic> responseData,
  ) {
    final orderId = (responseData['id'] as String?)?.trim() ?? '';

    if (orderId.isEmpty) {
      throw CheckoutException(
        message: 'Checkout response missing required field: id',
        userFriendlyMessage:
            'Pesanan tidak dapat dilanjutkan karena respons server tidak '
            'lengkap. Silakan coba lagi.',
        code: 'CHECKOUT_INCOMPLETE_RESPONSE',
      );
    }

    // Parse pricing snapshot from Order entity (int64 cents from backend)
    final subtotal = (responseData['subtotal'] as num?)?.toInt() ?? 0;
    final shippingTotal =
        (responseData['shipping_total'] as num?)?.toInt() ?? 0;
    final commissionAmount =
        (responseData['commission_amount'] as num?)?.toInt() ?? 0;
    final escrowAmount =
        (responseData['escrow_amount'] as num?)?.toInt() ??
        (responseData['total_amount'] as num?)?.toInt() ??
        0;

    // Parse created_at — backend sends RFC3339 string
    final createdAtStr = responseData['created_at'] as String?;
    final createdAt = createdAtStr != null
        ? (DateTime.tryParse(createdAtStr) ?? DateTime.now())
        : DateTime.now();

    return CheckoutResponse(
      orderId: orderId,
      orderNumber: responseData['order_number'] as String?,
      status: responseData['status'] as String? ?? 'pending_payment',
      subtotal: subtotal,
      shippingTotal: shippingTotal,
      commissionAmount: commissionAmount,
      escrowAmount: escrowAmount,
      coinsUsed: (responseData['coins_used'] as num?)?.toInt(),
      createdAt: createdAt,
    );
  }

  /// Parses API error codes into user-friendly exceptions
  CheckoutException _parseApiError(String? code, String? message) {
    // Map specific error codes to user-friendly messages
    switch (code) {
      case 'PRICING_TOKEN_EXPIRED':
      case 'TOKEN_EXPIRED':
        return CheckoutException(
          message: 'Pricing token expired',
          userFriendlyMessage:
              'Waktu harga telah habis. Silakan refresh untuk mendapatkan harga terbaru.',
          code: 'PRICING_TOKEN_EXPIRED',
        );

      case 'PRICING_TOKEN_INVALID':
      case 'INVALID_TOKEN':
        return CheckoutException(
          message: 'Invalid pricing token',
          userFriendlyMessage:
              'Token harga tidak valid. Silakan refresh harga terbaru.',
          code: 'PRICING_TOKEN_INVALID',
        );

      case 'LISTING_UNAVAILABLE':
      case 'OUT_OF_STOCK':
        return CheckoutException(
          message: 'Listing unavailable or out of stock',
          userFriendlyMessage:
              'Maaf, produk ini tidak tersedia atau telah habis terjual.',
          code: 'LISTING_UNAVAILABLE',
        );

      case 'SHIPPING_INVALID':
        return CheckoutException(
          message: 'Invalid shipping address',
          userFriendlyMessage:
              'Alamat pengiriman tidak valid. Mohon periksa kembali alamat Anda.',
          code: 'SHIPPING_INVALID',
        );

      case 'PREVIEW_INVALID':
        return CheckoutException(
          message: 'Preview data invalid or expired',
          userFriendlyMessage:
              'Data preview tidak valid. Silakan kembali dan ulangi proses checkout.',
          code: 'PREVIEW_INVALID',
        );

      case 'INSUFFICIENT_COIN_BALANCE':
        return CheckoutException(
          message: 'Insufficient coin balance',
          userFriendlyMessage:
              'Saldo koin tidak mencukupi untuk transaksi ini.',
          code: 'INSUFFICIENT_COIN_BALANCE',
        );

      case 'ADDRESS_INVALID':
        return CheckoutException(
          message: 'Invalid address',
          userFriendlyMessage:
              'Alamat pengiriman tidak lengkap atau tidak valid.',
          code: 'ADDRESS_INVALID',
        );

      // Commerce restriction — preserve raw backend code so the UI
      // layer can branch via CheckoutState.errorCode.
      case api_codes.commerceRestricted:
        return CheckoutException(
          message: 'Commerce activity restricted',
          userFriendlyMessage:
              'Aktivitas commerce Anda saat ini dibatasi.',
          code: api_codes.commerceRestricted,
        );

      default:
        return CheckoutException(
          message: message ?? 'Unknown error',
          userFriendlyMessage:
              message ?? 'Terjadi kesalahan. Silakan coba lagi.',
          code: code ?? 'UNKNOWN',
        );
    }
  }
}
