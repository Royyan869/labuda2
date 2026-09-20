import 'dart:async';

import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

import '../domain/domain.dart';
import 'mappers/order_mapper.dart';
import 'models/api/order_api_models.dart';
import 'remote/order_api_datasource_impl.dart';
import 'remote/order_remote_datasource.dart';

/// Order Repository Implementation
///
/// API-based implementation of OrderRepository interface.
/// Uses polling for real-time streams since backend doesn't support WebSocket yet.
class OrderRepositoryImpl implements OrderRepository {
  final OrderRemoteDatasource _datasource;
  final ILoggerService? _logger;

  // Polling intervals for real-time simulation
  static const _orderPollingInterval = Duration(seconds: 15);
  static const _ordersListPollingInterval = Duration(seconds: 30);

  OrderRepositoryImpl(this._datasource, {ILoggerService? logger})
    : _logger = logger;

  RepositoryResult<T> _mapError<T>(Object e) {
    if (e is OrderApiException) {
      return RepositoryResult.error(
        e.message,
        code: e.code,
        details: e.details,
      );
    }
    return RepositoryResult.error(e.toString());
  }

  // ========================================
  // Order Preview Operations
  // ========================================

  @override
  Future<RepositoryResult<PreviewOrderResult>> previewOrder(
    PreviewOrderParams params,
  ) async {
    try {
      final body = <String, dynamic>{
        'product_id': params.productId ?? '',
        'source_type': params.sourceType ?? 'for_sale',
        'source_id': params.sourceId ?? '',
        'quantity': params.quantity,
        if (params.addressId != null) 'address_id': params.addressId,
        if (params.shippingSetupId != null)
          'shipping_option_id': params.shippingSetupId,
        if (params.shippingQuoteId != null)
          'shipping_quote_id': params.shippingQuoteId,
        if (params.negotiationId != null)
          'negotiation_id': params.negotiationId,
        if (params.discountCode != null) 'discount_code': params.discountCode,
      };
      final data = await _datasource.fetchPricingPreview(body);
      final snapshot = data['pricing_snapshot'] as Map<String, dynamic>? ?? {};
      return RepositoryResult.success(
        PreviewOrderResult(
          pricing: OrderPricing(
            subtotal: (snapshot['subtotal'] as num?)?.toDouble() ?? 0.0,
            shippingCost:
                (snapshot['shipping_total'] as num?)?.toDouble() ?? 0.0,
            commissionAmount:
                (snapshot['commission_amount'] as num?)?.toDouble() ?? 0.0,
            serviceFeeAmount:
                (snapshot['service_fee_amount'] as num?)?.toDouble() ?? 0.0,
            totalPayableAmount:
                (snapshot['total_payable_amount'] as num?)?.toDouble() ?? 0.0,
            totalBeforeCoinsAmount:
                (snapshot['total_before_coins_amount'] as num?)?.toDouble(),
          ),
          pricingToken: data['token'] as String?,
          expiresAt: data['expires_at'] != null
              ? DateTime.tryParse(data['expires_at'] as String)
              : null,
          sellerId: snapshot['seller_id'] as String?,
          shippingMode: params.shippingQuoteId != null ? 'quote' : 'standard',
        ),
      );
    } catch (e) {
      return _mapError(e);
    }
  }

  // ========================================
  // Order CRUD Operations
  // ========================================

  @override
  Future<RepositoryResult<Order>> getOrderById(String orderId) async {
    try {
      final result = await _datasource.getOrder(orderId);
      return RepositoryResult.success(OrderMapper.toOrder(result));
    } catch (e) {
      return _mapError(e);
    }
  }

  @override
  Future<RepositoryResult<List<Order>>> getBuyerOrders(
    GetOrdersParams params,
  ) async {
    try {
      final queryParams = OrderFilterParams(
        status: params.status,
        pageSize: params.limit ?? 20,
      );

      final result = await _datasource.listMyOrders(params: queryParams);
      return RepositoryResult.success(OrderMapper.toOrderList(result.data));
    } catch (e) {
      return _mapError(e);
    }
  }

  @override
  Future<RepositoryResult<List<Order>>> getSellerOrders(
    GetOrdersParams params,
  ) async {
    try {
      final queryParams = OrderFilterParams(
        status: params.status,
        pageSize: params.limit ?? 20,
      );

      final result = await _datasource.listSellerOrders(params: queryParams);
      return RepositoryResult.success(OrderMapper.toOrderList(result.data));
    } catch (e) {
      return _mapError(e);
    }
  }

  @override
  Future<RepositoryResult<OrderPageResult>> getBuyerOrdersPage(
    GetOrdersParams params,
  ) async {
    final result = await getBuyerOrders(params);
    if (result.isError || result.data == null) {
      return RepositoryResult.error(
        result.error ?? 'Failed to load buyer orders',
      );
    }
    return RepositoryResult.success(
      OrderPageResult(
        orders: result.data!,
        page: params.page ?? 0,
        pageSize: params.pageSize ?? params.limit ?? 20,
      ),
    );
  }

  @override
  Future<RepositoryResult<OrderPageResult>> getSellerOrdersPage(
    GetOrdersParams params,
  ) async {
    final result = await getSellerOrders(params);
    if (result.isError || result.data == null) {
      return RepositoryResult.error(
        result.error ?? 'Failed to load seller orders',
      );
    }
    return RepositoryResult.success(
      OrderPageResult(
        orders: result.data!,
        page: params.page ?? 0,
        pageSize: params.pageSize ?? params.limit ?? 20,
      ),
    );
  }

  // ========================================
  // Order Status Operations
  // ========================================

  @override
  Future<RepositoryResult<Order>> cancelOrder(
    String orderId,
    CancelOrderParams params,
  ) async {
    try {
      await _datasource.cancelOrder(orderId);
      return await getOrderById(orderId);
    } catch (e) {
      return _mapError(e);
    }
  }

  @override
  Future<RepositoryResult<Order>> markAsShipped(
    MarkAsShippedParams params,
  ) async {
    try {
      await _datasource.shipOrder(params.orderId, params);
      return await getOrderById(params.orderId);
    } catch (e) {
      return _mapError(e);
    }
  }

  @override
  Future<RepositoryResult<Order>> markAsDelivered(String orderId) async {
    try {
      await _datasource.completeOrder(orderId);
      return await getOrderById(orderId);
    } catch (e) {
      return _mapError(e);
    }
  }

  // ========================================
  // Order Action Operations (Decision V2)
  // ========================================

  @override
  Future<RepositoryResult<void>> extendOrderConfirmation(String orderId) async {
    try {
      await _datasource.extendOrderConfirmation(orderId);
      // Backend action response is slim; refetch canonical order payload.
      await _datasource.getOrder(orderId);
      return RepositoryResult.success(null);
    } catch (e) {
      _logger?.error('Failed to extend order confirmation: $e');
      return _mapError(e);
    }
  }

  // ========================================
  // Real-time Streams (Polling Implementation)
  // ========================================

  @override
  Stream<Order> watchOrder(String orderId) async* {
    bool isFetching = false;

    // Immediate first fetch
    final initialResult = await getOrderById(orderId);
    if (initialResult.isSuccess && initialResult.data != null) {
      yield initialResult.data!;
    } else if (initialResult.isError) {
      _logger?.warning('Initial order fetch failed: ${initialResult.error}');
    }

    // Polling loop with concurrency guard
    while (true) {
      if (!isFetching) {
        isFetching = true;
        try {
          final result = await getOrderById(orderId);
          if (result.isSuccess && result.data != null) {
            yield result.data!;
          } else if (result.isError) {
            _logger?.warning('Order polling failed: ${result.error}');
            // Continue polling on error - don't break the stream
          }
        } finally {
          isFetching = false;
        }
      }
      await Future.delayed(_orderPollingInterval);
    }
  }

  @override
  Stream<List<Order>> watchBuyerOrders(WatchOrdersParams params) async* {
    bool isFetching = false;

    // Immediate first fetch
    final initialResult = await getBuyerOrders(
      GetOrdersParams(userId: params.userId, status: params.status),
    );
    if (initialResult.isSuccess && initialResult.data != null) {
      yield initialResult.data!;
    } else if (initialResult.isError) {
      _logger?.warning(
        'Initial buyer orders fetch failed: ${initialResult.error}',
      );
    }

    // Polling loop with concurrency guard
    while (true) {
      if (!isFetching) {
        isFetching = true;
        try {
          final result = await getBuyerOrders(
            GetOrdersParams(userId: params.userId, status: params.status),
          );
          if (result.isSuccess && result.data != null) {
            yield result.data!;
          } else if (result.isError) {
            _logger?.warning('Buyer orders polling failed: ${result.error}');
          }
        } finally {
          isFetching = false;
        }
      }
      await Future.delayed(_ordersListPollingInterval);
    }
  }

  @override
  Stream<List<Order>> watchSellerOrders(WatchOrdersParams params) async* {
    bool isFetching = false;

    // Immediate first fetch
    final initialResult = await getSellerOrders(
      GetOrdersParams(userId: params.userId, status: params.status),
    );
    if (initialResult.isSuccess && initialResult.data != null) {
      yield initialResult.data!;
    } else if (initialResult.isError) {
      _logger?.warning(
        'Initial seller orders fetch failed: ${initialResult.error}',
      );
    }

    // Polling loop with concurrency guard
    while (true) {
      if (!isFetching) {
        isFetching = true;
        try {
          final result = await getSellerOrders(
            GetOrdersParams(userId: params.userId, status: params.status),
          );
          if (result.isSuccess && result.data != null) {
            yield result.data!;
          } else if (result.isError) {
            _logger?.warning('Seller orders polling failed: ${result.error}');
          }
        } finally {
          isFetching = false;
        }
      }
      await Future.delayed(_ordersListPollingInterval);
    }
  }

  @override
  Stream<List<Order>> watchSellerNewOrders(String sellerId) async* {
    bool isFetching = false;

    // Immediate first fetch
    final initialResult = await getSellerOrders(
      GetOrdersParams(
        userId: sellerId,
        status: OrderStatus.paid, // New orders are paid but not yet processed
      ),
    );
    if (initialResult.isSuccess && initialResult.data != null) {
      yield initialResult.data!;
    } else if (initialResult.isError) {
      _logger?.warning(
        'Initial new orders fetch failed: ${initialResult.error}',
      );
    }

    // Polling loop with concurrency guard
    while (true) {
      if (!isFetching) {
        isFetching = true;
        try {
          final result = await getSellerOrders(
            GetOrdersParams(userId: sellerId, status: OrderStatus.paid),
          );
          if (result.isSuccess && result.data != null) {
            yield result.data!;
          } else if (result.isError) {
            _logger?.warning('New orders polling failed: ${result.error}');
          }
        } finally {
          isFetching = false;
        }
      }
      await Future.delayed(_ordersListPollingInterval);
    }
  }
}
