import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../domain/domain.dart';
import '../../data/order_providers.dart';
import 'order_state.dart';

part 'order_notifier.g.dart';

@riverpod
class OrderNotifier extends _$OrderNotifier {
  @override
  OrderState build() {
    return const OrderInitial();
  }

  // Lazy initialization - repository is fetched when methods are called
  OrderRepository get _orderRepository => ref.read(orderRepositoryProvider);

  Future<void> getOrderById(String orderId) async {
    state = const OrderLoading();
    final result = await _orderRepository.getOrderById(orderId);
    result.fold((o) => state = OrderLoaded(o), (e) => state = OrderError(e));
  }

  Future<void> getBuyerOrders({
    required String buyerId,
    OrderStatus? status,
    int page = 1,
    int limit = 20,
  }) async {
    state = OrderListLoading(buyerId);
    final result = await _orderRepository.getBuyerOrders(
      GetOrdersParams(
        userId: buyerId,
        status: status,
        page: page,
        limit: limit,
      ),
    );
    result.fold(
      (o) => state = OrderListLoaded(o, hasMore: o.length >= limit),
      (e) => state = OrderError(e),
    );
  }

  Future<void> getSellerOrders({
    required String sellerId,
    OrderStatus? status,
    int page = 1,
    int limit = 20,
  }) async {
    state = const OrderLoading();
    final result = await _orderRepository.getSellerOrders(
      GetOrdersParams(
        userId: sellerId,
        status: status,
        page: page,
        limit: limit,
      ),
    );
    result.fold(
      (o) => state = OrderListLoaded(o, hasMore: o.length >= limit),
      (e) => state = OrderError(e),
    );
  }

  Future<void> shipOrder(
    String orderId,
    String shippingReference, {
    String? referenceType,
    String? note,
  }) async {
    state = const OrderLoading();
    final result = await _orderRepository.markAsShipped(
      MarkAsShippedParams(
        orderId: orderId,
        shippingReference: shippingReference,
        referenceType: referenceType,
        note: note,
      ),
    );
    result.fold(
      (o) => state = OrderSuccess('Shipped', order: o),
      (e) => state = OrderError(e),
    );
  }

  Future<void> confirmDelivery(String orderId) async {
    state = const OrderLoading();
    final result = await _orderRepository.markAsDelivered(orderId);
    result.fold(
      (o) => state = OrderSuccess('Delivered', order: o),
      (e) => state = OrderError(e),
    );
  }

  Future<void> cancelOrder(String orderId, String reason) async {
    state = const OrderLoading();
    final result = await _orderRepository.cancelOrder(
      orderId,
      CancelOrderParams(reason: reason),
    );
    result.fold(
      (o) => state = OrderSuccess('Cancelled', order: o),
      (e) => state = OrderError(e),
    );
  }

  /// Extend order confirmation deadline (Decision V2 action)
  Future<void> extendOrderConfirmation(String orderId) async {
    state = const OrderLoading();
    final result = await _orderRepository.extendOrderConfirmation(orderId);
    result.fold(
      (_) => state = const OrderSuccess('Confirmation extended'),
      (e) => state = OrderError(e),
    );
  }
}
